package functest_test

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"go-backend/internal/backend/user"
	userRepo "go-backend/internal/backend/user/repo"
	"go-backend/internal/backend/userdedup"
)

// This file covers the failure that took production down on 2026-08-09, the day after the sqlc
// build shipped: the server would not start, with
//
//	can't init users login index: UNIQUE constraint failed: users.login
//
// gorm_v1.sql cannot reproduce it. That dump carries a CREATE UNIQUE INDEX idx_users_login line
// which GORM never wrote — the index came from the sqlc user repo opening the file while the
// dump was being taken — and an indexed users table cannot hold duplicates in the first place.
// So the migration suite was checking the migration against a database the migration had
// already touched, and the one shape that actually breaks was the one it could not represent.
//
// testdata/legacy/gorm_v1_dup_logins.sql is that shape: no index, and two logins held twice.

const legacyDupDump = "testdata/legacy/gorm_v1_dup_logins.sql"

// loadDupLoginsDB materialises the duplicate-login fixture and returns a handle to it. Unlike
// loadLegacyDB it does not bring the stack up, because that is exactly what fails.
func loadDupLoginsDB(t *testing.T) *sql.DB {
	t.Helper()

	script, err := os.ReadFile(legacyDupDump)
	require.NoError(t, err)

	path := filepath.Join(t.TempDir(), "legacy-dup.db")

	db, err := sql.Open("sqlite3", path)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })

	_, err = db.Exec(string(script))
	require.NoError(t, err)

	return db
}

func logins(t *testing.T, db *sql.DB) []string {
	t.Helper()

	rows, err := db.Query(`SELECT login FROM users ORDER BY login`)
	require.NoError(t, err)

	defer func() { require.NoError(t, rows.Close()) }()

	var res []string

	for rows.Next() {
		var login string
		require.NoError(t, rows.Scan(&login))

		res = append(res, login)
	}

	require.NoError(t, rows.Err())

	return res
}

// The boot has to fail — the alternative is a server running without the constraint — but it
// has to fail in a way that says what is wrong. "UNIQUE constraint failed: users.login" alone
// names neither the login nor the fix, and it is the only thing the operator gets before the
// process exits.
func TestDuplicateLoginsStopTheServerFromStarting(t *testing.T) {
	t.Parallel()

	db := loadDupLoginsDB(t)

	_, err := userRepo.NewRepo(context.Background(), db)
	require.ErrorIs(t, err, userRepo.ErrDuplicateLogins)
	require.Contains(t, err.Error(), `"alice" x2`)
	require.Contains(t, err.Error(), `"bob" x2`)
	require.Contains(t, err.Error(), "shoplannerctl db dedup-logins")
}

// A fresh database has the index from the first boot, so it can never get into this state.
func TestScanFindsNoDuplicatesOnAWorkingDatabase(t *testing.T) {
	t.Parallel()

	a := newApp(t)
	a.newUser(t, "vasya")

	groups, err := userdedup.Scan(context.Background(), a.sqlDB)
	require.NoError(t, err)
	require.Empty(t, groups)
}

// What the operator is really asked is "which of these two is the real account", and the only
// evidence for that is how much each one owns. Getting those counts right, across every table
// that references users, is the whole job of Scan.
func TestScanReportsWhatEachDuplicateOwns(t *testing.T) {
	t.Parallel()

	db := loadDupLoginsDB(t)

	groups, err := userdedup.Scan(context.Background(), db)
	require.NoError(t, err)
	require.Len(t, groups, 2)

	require.Equal(t, user.Login("alice"), groups[0].Login)
	require.Equal(t, user.Login("bob"), groups[1].Login)

	// The account owning the most data comes first, so that the default answer is the safe one.
	alice, aliceDup := groups[0].Accounts[0], groups[0].Accounts[1]
	require.Equal(t, groups[0].Keeper(), alice)
	require.Equal(t, userdedup.Usage{
		"lists": 1, "favorites": 1, "owned maps": 1, "map views": 0,
	}, alice.Usage)
	require.Equal(t, 0, aliceDup.Usage.Total(), "the second alice is the one that owns nothing")

	bob, bobDup := groups[1].Accounts[0], groups[1].Accounts[1]
	require.Equal(t, userdedup.Usage{
		"lists": 2, "favorites": 1, "owned maps": 1, "map views": 1,
	}, bob.Usage)
	// Not empty: deleting this one would take a list membership and a map view with it, which
	// is why the tool only ever offers to delete an account owning nothing.
	require.Equal(t, userdedup.Usage{
		"lists": 1, "favorites": 0, "owned maps": 0, "map views": 1,
	}, bobDup.Usage)
}

// The unattended path, end to end: rename everything that is not the keeper, and the database
// that could not be opened at the top of this file comes up.
func TestRecommendedFixLetsTheServerStart(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := loadDupLoginsDB(t)

	groups, err := userdedup.Scan(ctx, db)
	require.NoError(t, err)

	decisions := lo.FlatMap(groups, func(g userdedup.Group, _ int) []userdedup.Decision {
		return g.Recommend()
	})

	outcomes, err := userdedup.Apply(ctx, db, decisions)
	require.NoError(t, err)
	require.Len(t, outcomes, 2)

	for _, outcome := range outcomes {
		require.Equal(t, userdedup.ActionRename, outcome.Action)
		require.True(t, strings.HasPrefix(string(outcome.NewLogin), string(outcome.Account.Login)+"__dup__"),
			"renamed to %q, which does not point back at %q", outcome.NewLogin, outcome.Account.Login)
	}

	// Nobody is deleted by the unattended path: five accounts in, five accounts out.
	require.Len(t, logins(t, db), 5)

	remaining, err := userdedup.Scan(ctx, db)
	require.NoError(t, err)
	require.Empty(t, remaining)

	_, err = userRepo.NewRepo(ctx, db)
	require.NoError(t, err, "the server still refuses the database the tool just fixed")
}

// Deleting is the one thing here that cannot be undone, so it is refused for anything that
// owns data even when explicitly asked for — and the refusal takes the whole batch with it,
// rather than leaving half the database resolved.
func TestDeletingAnAccountThatOwnsDataIsRefusedAndRollsBack(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := loadDupLoginsDB(t)

	before := logins(t, db)

	groups, err := userdedup.Scan(ctx, db)
	require.NoError(t, err)

	bob := groups[1]
	_, err = userdedup.Apply(ctx, db, []userdedup.Decision{
		// A rename that would succeed on its own, ...
		{Account: groups[0].Accounts[1], Action: userdedup.ActionRename},
		// ... followed by a delete of an account owning a list membership and a map view.
		{Account: bob.Accounts[1], Action: userdedup.ActionDelete},
	})
	require.ErrorIs(t, err, userdedup.ErrNotEmpty)

	require.Equal(t, before, logins(t, db), "the failed batch left changes behind")
}

// Resolve is the policy the prompt drives: keep one account, rename the rest, and delete only
// the ones owning nothing and only when asked.
func TestResolveNeverDeletesAnAccountThatOwnsData(t *testing.T) {
	t.Parallel()

	db := loadDupLoginsDB(t)

	groups, err := userdedup.Scan(context.Background(), db)
	require.NoError(t, err)

	alice, bob := groups[0], groups[1]

	require.Equal(t, []userdedup.Decision{
		{Account: alice.Accounts[1], Action: userdedup.ActionDelete},
	}, alice.Resolve(alice.Keeper(), true))

	// Same answer, different group: this duplicate owns something, so it is renamed instead.
	require.Equal(t, []userdedup.Decision{
		{Account: bob.Accounts[1], Action: userdedup.ActionRename},
	}, bob.Resolve(bob.Keeper(), true))

	// Keeping the empty account is allowed — it is the operator's call — and then the account
	// owning the data is the one that gets renamed, never deleted.
	require.Equal(t, []userdedup.Decision{
		{Account: alice.Accounts[0], Action: userdedup.ActionRename},
	}, alice.Resolve(alice.Accounts[1], true))
}
