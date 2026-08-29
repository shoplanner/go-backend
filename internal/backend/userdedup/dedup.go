// Package userdedup resolves duplicate logins in an existing database.
//
// It exists because of a hole in the GORM-era schema: its user model carried no unique tag on
// Login, AutoMigrate rebuilt the users table from that model, and so nothing on any deployed
// disk ever stopped two accounts from sharing a login. The sqlc code puts the constraint back
// as idx_users_login (see user/repo/schema.sql), which a database holding duplicates cannot
// create — user/repo.NewRepo fails with ErrDuplicateLogins and the server does not start.
//
// Merging two accounts is not something this package will do: each one owns its own lists,
// favorites and maps, and only a human knows whether they are the same person. What it does is
// describe the collision precisely enough to decide — every account in the group, with how much
// data each one owns — and then apply the decision, which is always either "rename this one" or
// "delete this one, it owns nothing". cmd/dedup-logins is the interactive front end.
package userdedup

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/samber/lo"

	"go-backend/internal/backend/user"
	userRepo "go-backend/internal/backend/user/repo"
	"go-backend/pkg/id"
	"go-backend/pkg/myerr"
)

// ErrNotEmpty is returned when a delete is asked for an account that still owns data. The tool
// offers deletion only for the accounts a duplicate group accumulated by accident — an account
// somebody actually used has to be renamed instead, or dealt with by hand.
var ErrNotEmpty = errors.New("account owns data")

// Action is what to do with one account of a duplicate group. The account that keeps the login
// gets no Action at all: it is simply left alone.
type Action int

const (
	// ActionRename gives the account a free login derived from the one it is losing.
	ActionRename Action = iota + 1
	// ActionDelete removes the row. Only legal while Usage.Total is zero.
	ActionDelete
)

// reference is one column in another domain's table that points at users.id. The tool has to
// know about all of them: "does this account own anything" is exactly the question the operator
// is answering, and an account that looks empty because a table was missed is a wrong answer.
type reference struct {
	label  string
	table  string
	column string
}

// referenceTables lists every foreign key into users across the schema. This is deliberately
// hand-maintained rather than read out of PRAGMA foreign_key_list: the tool also runs against
// GORM-era files, where several of these tables declare no constraint at all.
func referenceTables() []reference {
	const userID = "user_id"

	return []reference{
		{label: "lists", table: "product_list_members", column: userID},
		{label: "favorites", table: "favorite_members", column: userID},
		{label: "owned maps", table: "shop_maps", column: "owner_id"},
		{label: "map views", table: "shop_map_viewers", column: userID},
	}
}

// Usage counts the rows that reference one account, keyed by reference.label.
type Usage map[string]int

// Total is how many rows across the whole database point at the account.
func (u Usage) Total() int {
	return lo.Sum(lo.Values(u))
}

// String renders the counts in referenceTables order: `lists=2 favorites=1 owned maps=0`.
func (u Usage) String() string {
	parts := lo.Map(referenceTables(), func(ref reference, _ int) string {
		return fmt.Sprintf("%s=%d", ref.label, u[ref.label])
	})

	return strings.Join(parts, " ")
}

// Account is one row of a duplicate group.
type Account struct {
	ID    id.ID[user.User]
	Login user.Login
	Role  user.Role
	Usage Usage
}

// Group is all the accounts sharing one login, richest first so that the account with the
// strongest claim to the login is the obvious default.
type Group struct {
	Login    user.Login
	Accounts []Account
}

// Decision is one account and what the operator chose to do with it.
type Decision struct {
	Account Account
	Action  Action
}

// Outcome is a Decision that has been carried out; NewLogin is set for ActionRename.
type Outcome struct {
	Account  Account
	Action   Action
	NewLogin user.Login
}

// Scan reports every login held by more than one account. An empty result means the database
// is ready for idx_users_login and the server will start.
func Scan(ctx context.Context, db *sql.DB) ([]Group, error) {
	rows, err := userRepo.FindDuplicateLogins(ctx, db)
	if err != nil {
		return nil, fmt.Errorf("can't find duplicate logins: %w", err)
	}

	present, err := presentTables(ctx, db)
	if err != nil {
		return nil, err
	}

	byLogin := map[user.Login][]Account{}

	for _, row := range rows {
		usage, usageErr := countUsage(ctx, db, present, row.ID)
		if usageErr != nil {
			return nil, usageErr
		}

		byLogin[row.Login] = append(byLogin[row.Login], Account{
			ID:    row.ID,
			Login: row.Login,
			Role:  row.Role,
			Usage: usage,
		})
	}

	groups := lo.MapToSlice(byLogin, func(login user.Login, accounts []Account) Group {
		sortAccounts(accounts)

		return Group{Login: login, Accounts: accounts}
	})

	sort.Slice(groups, func(i, j int) bool { return groups[i].Login < groups[j].Login })

	return groups, nil
}

// sortAccounts orders a group by how much data each account owns, descending, with the id as
// the tie-break so the order is stable across runs.
func sortAccounts(accounts []Account) {
	sort.Slice(accounts, func(i, j int) bool {
		left, right := accounts[i], accounts[j]
		if left.Usage.Total() != right.Usage.Total() {
			return left.Usage.Total() > right.Usage.Total()
		}

		return left.ID.String() < right.ID.String()
	})
}

// Keeper is the account a group would keep if nobody says otherwise: the one owning the most
// data. Scan already sorted the group that way.
func (g Group) Keeper() Account {
	return g.Accounts[0]
}

// Resolve is the change list for keeping one account of a group: everyone else is renamed, or
// deleted when deleteEmpty is set and they own nothing. An account that owns data is never
// deleted, whatever was asked for — that is the one irreversible thing in here.
func (g Group) Resolve(keeper Account, deleteEmpty bool) []Decision {
	others := lo.Filter(g.Accounts, func(acc Account, _ int) bool { return acc.ID != keeper.ID })

	return lo.Map(others, func(acc Account, _ int) Decision {
		if deleteEmpty && acc.Usage.Total() == 0 {
			return Decision{Account: acc, Action: ActionDelete}
		}

		return Decision{Account: acc, Action: ActionRename}
	})
}

// Recommend is the non-interactive plan for a group: keep the Keeper, rename everyone else.
// Renaming rather than deleting even when the other accounts are empty, because unattended is
// the wrong mode in which to remove somebody's account.
func (g Group) Recommend() []Decision {
	return g.Resolve(g.Keeper(), false)
}

// Apply carries out the decisions in a single transaction: either the database comes out with
// every group resolved, or it is left exactly as it was found.
func Apply(ctx context.Context, db *sql.DB, decisions []Decision) ([]Outcome, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("can't start transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	outcomes := make([]Outcome, 0, len(decisions))

	for _, decision := range decisions {
		outcome, applyErr := apply(ctx, tx, decision)
		if applyErr != nil {
			return nil, applyErr
		}

		outcomes = append(outcomes, outcome)
	}

	if err = tx.Commit(); err != nil {
		return nil, fmt.Errorf("can't commit the changes: %w", err)
	}

	return outcomes, nil
}

func apply(ctx context.Context, tx *sql.Tx, decision Decision) (Outcome, error) {
	var none Outcome

	switch decision.Action {
	case ActionRename:
		login, err := rename(ctx, tx, decision.Account)
		if err != nil {
			return none, err
		}

		return Outcome{Account: decision.Account, Action: ActionRename, NewLogin: login}, nil
	case ActionDelete:
		if decision.Account.Usage.Total() != 0 {
			return none, fmt.Errorf("%w: %s owns %s", ErrNotEmpty, decision.Account.ID, decision.Account.Usage)
		}

		if err := userRepo.Delete(ctx, tx, decision.Account.ID); err != nil {
			return none, fmt.Errorf("deleting account: %w", err)
		}

		return Outcome{Account: decision.Account, Action: ActionDelete, NewLogin: ""}, nil
	default:
		return none, fmt.Errorf("%w: unknown action %d", myerr.ErrInvalidArgument, decision.Action)
	}
}

// idPrefixLen is how much of the account id ProposedLogin puts into a renamed login: enough to
// find the row again in the users table, short enough to still be typeable.
const idPrefixLen = 8

// ProposedLogin is the login ActionRename will give an account: `alice` becomes
// `alice__dup__eaf6eca9`. Exported so a caller can show what it is about to do before doing it;
// Apply may still land on a longer suffix if that one turns out to be taken.
func ProposedLogin(acc Account) user.Login {
	return user.Login(fmt.Sprintf("%s__dup__%s", acc.Login, acc.ID.String()[:idPrefixLen]))
}

// rename gives the account a free login derived from the one it is giving up. The id in it
// makes a collision essentially impossible, but "essentially" is not good enough for something
// that runs unattended, so a login that is taken is retried with more of the id.
func rename(ctx context.Context, tx *sql.Tx, acc Account) (user.Login, error) {
	raw := acc.ID.String()

	for length := idPrefixLen; length <= len(raw); length += 4 {
		login := user.Login(fmt.Sprintf("%s__dup__%s", acc.Login, raw[:length]))

		err := userRepo.SetLogin(ctx, tx, acc.ID, login)
		if err == nil {
			return login, nil
		}

		if !errors.Is(err, myerr.ErrAlreadyExists) {
			return "", fmt.Errorf("renaming account: %w", err)
		}
	}

	return "", fmt.Errorf("%w: no free login derived from %q", myerr.ErrAlreadyExists, string(acc.Login))
}

// EnsureUniqueIndex creates idx_users_login, the index the server needs and cannot create
// itself while duplicates exist. Running it here means the tool answers the only question the
// operator actually has — "will it boot now?" — instead of leaving them to restart and find out.
func EnsureUniqueIndex(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, `CREATE UNIQUE INDEX IF NOT EXISTS idx_users_login ON users(login)`)
	if err != nil {
		return fmt.Errorf("can't create the unique index on users.login: %w", err)
	}

	return nil
}

// presentTables reports which of the referenced tables the database actually has. A file
// written by an older version, or one the server has never fully started against, may be
// missing some of them, and a missing table is zero rows rather than an error.
func presentTables(ctx context.Context, db *sql.DB) (map[string]bool, error) {
	present := map[string]bool{}

	for _, ref := range referenceTables() {
		var count int

		row := db.QueryRowContext(ctx,
			`SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, ref.table)
		if err := row.Scan(&count); err != nil {
			return nil, fmt.Errorf("can't look up table %s: %w", ref.table, err)
		}

		present[ref.table] = count > 0
	}

	return present, nil
}

func countUsage(ctx context.Context, db *sql.DB, present map[string]bool, userID id.ID[user.User]) (Usage, error) {
	usage := Usage{}

	for _, ref := range referenceTables() {
		if !present[ref.table] {
			usage[ref.label] = 0

			continue
		}

		var count int

		//nolint:gosec // the table and column names come from referenceTables(), never from input
		query := fmt.Sprintf(`SELECT count(*) FROM %s WHERE %s = ?`, ref.table, ref.column)
		if err := db.QueryRowContext(ctx, query, userID.String()).Scan(&count); err != nil {
			return nil, fmt.Errorf("can't count %s of user %s: %w", ref.label, userID, err)
		}

		usage[ref.label] = count
	}

	return usage, nil
}
