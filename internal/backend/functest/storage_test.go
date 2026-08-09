package functest_test

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/samber/mo"
	"github.com/stretchr/testify/require"

	"go-backend/internal/backend/list"
	"go-backend/internal/backend/product"
	"go-backend/internal/backend/shopmap"
	"go-backend/internal/backend/user"
	"go-backend/pkg/id"
)

// These tests are not about business logic. They pin the bytes on disk and the connection
// settings, which is where a storage rewrite actually goes wrong: a column that silently
// becomes NOT NULL, a timestamp written in a layout the new driver cannot parse, a pragma
// that starts enforcing constraints nobody has been honouring.

func (a *app) pragma(t *testing.T, name string) string {
	t.Helper()

	var value string
	require.NoError(t, a.sqlDB.QueryRow("PRAGMA "+name).Scan(&value))

	return value
}

// columnInfo is one row of PRAGMA table_info.
type columnInfo struct {
	name    string
	decl    string
	notNull bool
}

func (a *app) tableInfo(t *testing.T, table string) map[string]columnInfo {
	t.Helper()

	rows, err := a.sqlDB.Query("PRAGMA table_info(" + table + ")")
	require.NoError(t, err)

	defer func() { require.NoError(t, rows.Close()) }()

	res := map[string]columnInfo{}

	for rows.Next() {
		var (
			cid       int
			name      string
			decl      string
			notNull   int
			dflt      sql.NullString
			primaryPK int
		)

		require.NoError(t, rows.Scan(&cid, &name, &decl, &notNull, &dflt, &primaryPK))

		res[name] = columnInfo{name: name, decl: decl, notNull: notNull == 1}
	}

	require.NoError(t, rows.Err())

	return res
}

// The DSN is the bare database path with no query string, so every pragma is at its default.
// Turning any of these on during the migration changes behaviour for every deployed database.
func TestConnectionPragmasAreDefaults(t *testing.T) {
	t.Parallel()

	a := newApp(t)

	require.Equal(t, "0", a.pragma(t, "foreign_keys"), "foreign keys are declared but not enforced")
	require.Equal(t, "delete", a.pragma(t, "journal_mode"), "rollback journal, not WAL")
	// Not a SQLite default: mattn/go-sqlite3 applies 5s of its own when the DSN says nothing.
	require.Equal(t, "5000", a.pragma(t, "busy_timeout"))
}

// KNOWN BUG (current behaviour pinned). Every table declares foreign keys, and none of them
// are checked, because PRAGMA foreign_keys defaults to off and the DSN never turns it on.
// Rows referencing users, products or lists that do not exist insert happily today.
func TestForeignKeysAreNotEnforced_CurrentBehaviour(t *testing.T) {
	t.Parallel()

	a := newApp(t)

	_, err := a.sqlDB.Exec(`
		INSERT INTO product_list_members (id, user_id, list_id, created_at, updated_at, member_type)
		VALUES (?, ?, ?, ?, ?, ?)`,
		uuid.NewString(), uuid.NewString(), uuid.NewString(), time.Now(), time.Now(), 1)
	require.NoError(t, err, "a member pointing at a non-existent user and list is accepted")

	require.Equal(t, 1, a.count(t, `SELECT count(*) FROM product_list_members`))
}

func TestForeignKeysAreEnforced_Desired(t *testing.T) {
	t.Parallel()

	t.Skip("KNOWN BUG: PRAGMA foreign_keys is off; see TestForeignKeysAreNotEnforced_CurrentBehaviour")

	a := newApp(t)

	_, err := a.sqlDB.Exec(`
		INSERT INTO product_list_members (id, user_id, list_id, created_at, updated_at, member_type)
		VALUES (?, ?, ?, ?, ?, ?)`,
		uuid.NewString(), uuid.NewString(), uuid.NewString(), time.Now(), time.Now(), 1)
	require.Error(t, err)
}

// users is the one table with two shapes in the wild, and this is the whole of that story.
//
// user/repo/schema.sql has always declared `role int NOT NULL, login varchar(36) NOT NULL
// UNIQUE, hash text NOT NULL`. Before the migration, favorite/list AutoMigrate reached users
// through their member associations and rebuilt it with those NOT NULLs dropped and the
// uniqueness moved out into idx_users_login. CREATE TABLE IF NOT EXISTS cannot undo that, so a
// database that GORM ever touched keeps the loose shape for good while a fresh one now gets the
// strict DDL.
//
// That divergence is accepted rather than migrated, because the two enforce the same thing: the
// strict shape is a superset of the loose one, and the repo creates idx_users_login explicitly
// so uniqueness is indexed either way. What must never diverge is behaviour, which is what the
// two halves of this test check against the two shapes.
func TestUsersTableIsStrictOnAFreshDatabase(t *testing.T) {
	t.Parallel()

	a := newApp(t)

	info := a.tableInfo(t, "users")
	require.Len(t, info, 4)

	// SQLite lets a non-INTEGER PRIMARY KEY hold NULL unless NOT NULL is spelled out, and the
	// sqlc DDL does not spell it out.
	require.False(t, info["id"].notNull)
	require.True(t, info["role"].notNull)
	require.True(t, info["login"].notNull)
	require.True(t, info["hash"].notNull)

	requireLoginIsUnique(t, a)
}

// The loose shape is the one on every deployed disk. Uniqueness has to hold there too, and it
// does — through idx_users_login rather than through the column.
func TestUsersTableFromALegacyDatabaseStillEnforcesUniqueLogins(t *testing.T) {
	t.Parallel()

	a := loadLegacyDB(t)

	info := a.tableInfo(t, "users")
	require.Len(t, info, 4)
	require.False(t, info["role"].notNull, "the legacy shape is the one AutoMigrate left behind")
	require.False(t, info["login"].notNull)
	require.False(t, info["hash"].notNull)

	var ddl string
	require.NoError(t, a.sqlDB.QueryRow(
		`SELECT sql FROM sqlite_master WHERE type = 'table' AND name = 'users'`).Scan(&ddl))
	require.NotContains(t, strings.ToUpper(ddl), "UNIQUE",
		"the inline UNIQUE on login is gone here; it lives in idx_users_login")

	requireLoginIsUnique(t, a)
}

// requireLoginIsUnique asserts the index exists and actually rejects a duplicate login.
func requireLoginIsUnique(t *testing.T, a *app) {
	t.Helper()

	require.Equal(t, 1, a.count(t,
		`SELECT count(*) FROM sqlite_master WHERE type = 'index' AND name = 'idx_users_login'`))

	_, err := a.sqlDB.Exec(`INSERT INTO users (id, role, login, hash) VALUES (?, ?, ?, ?)`,
		uuid.NewString(), 2, "vasya", "hash")
	require.NoError(t, err)

	_, err = a.sqlDB.Exec(`INSERT INTO users (id, role, login, hash) VALUES (?, ?, ?, ?)`,
		uuid.NewString(), 2, "vasya", "hash")
	require.Error(t, err, "idx_users_login rejects duplicates")
}

// The columns GORM left nullable. The `notNull` tag works, but ProductList.Title spells it
// `gorm:"notNull,size:255"` with a comma instead of a semicolon, so the whole tag is one
// unrecognised key and both the constraint and the size are lost. A rewrite that "tidies up"
// by adding NOT NULL here would reject existing rows.
func TestNullableColumnsAreNullable(t *testing.T) {
	t.Parallel()

	a := newApp(t)

	lists := a.tableInfo(t, "product_lists")
	require.False(t, lists["title"].notNull, `gorm:"notNull,size:255" uses a comma and is ignored`)
	require.True(t, lists["status"].notNull)

	states := a.tableInfo(t, "product_list_states")
	nullableStateColumns := []string{
		"count", "form_idx",
		"replacement_count", "replacement_form_idx", "replacement_product_id",
	}

	for _, column := range nullableStateColumns {
		require.False(t, states[column].notNull, "%s must stay nullable", column)
	}

	products := a.tableInfo(t, "products")
	require.False(t, products["category_id"].notNull)
	require.True(t, products["name"].notNull)
}

// mo.None must reach the database as SQL NULL, not as a zero value. Distinguishing "no count"
// from "count is 0" is the whole point of the Option, and it is exactly what breaks when the
// write path stops using pointers.
func TestOptionalFieldsBecomeSQLNull(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	a := newApp(t)

	owner := a.newUser(t, "owner")
	listModel := a.newList(t, owner.ID, "groceries")
	milk := a.newProduct(t, "milk", "dairy")
	a.addProducts(t, listModel.ID, owner.ID, milk)

	require.Equal(t, 1, a.count(t, `
		SELECT count(*) FROM product_list_states
		WHERE count IS NULL AND form_idx IS NULL
		  AND replacement_count IS NULL AND replacement_form_idx IS NULL
		  AND replacement_product_id IS NULL`))

	// A present zero is stored as 0, not NULL.
	_, err := a.lists.UpdateProductState(ctx, listModel.ID, owner.ID, milk.ID, list.ProductStateOptions{
		Count:       mo.Some[int32](0),
		FormIndex:   mo.None[int32](),
		Status:      list.StateStatusWaiting,
		Replacement: mo.None[list.ProductStateReplacement](),
	})
	require.NoError(t, err)

	require.Equal(t, 1, a.count(t,
		`SELECT count(*) FROM product_list_states WHERE count = 0 AND form_idx IS NULL`))

	got, err := a.lists.GetByID(ctx, listModel.ID, owner.ID)
	require.NoError(t, err)
	require.Equal(t, mo.Some[int32](0), got.States[0].Count, "0 must not decay into None")
	require.True(t, got.States[0].FormIndex.IsAbsent())
}

// A timestamp written through a repo has to come back out of the raw handle intact, with its
// sub-second precision. This used to be a cross-pool check (GORM wrote, database/sql read); the
// cross-driver half of it now lives in TestLegacyTimestampsAreReadable, which reads timestamps
// that GORM really did write.
func TestTimestampsAreReadableByDatabaseSQL(t *testing.T) {
	t.Parallel()

	a := newApp(t)

	owner := a.newUser(t, "owner")
	before := time.Now()
	listModel := a.newList(t, owner.ID, "groceries")
	after := time.Now()

	var createdAt, updatedAt time.Time
	require.NoError(t, a.sqlDB.QueryRow(
		`SELECT created_at, updated_at FROM product_lists WHERE id = ?`, listModel.ID.String(),
	).Scan(&createdAt, &updatedAt))

	require.False(t, createdAt.IsZero())
	require.False(t, createdAt.Before(before.Add(-time.Second)))
	require.False(t, createdAt.After(after.Add(time.Second)))

	// Sub-second precision has to survive; the read path compares and sorts on these.
	require.WithinDuration(t, listModel.CreatedAt.Time, createdAt, time.Millisecond)
	require.WithinDuration(t, listModel.UpdatedAt.Time, updatedAt, time.Millisecond)
}

// The mirror image: shopmap writes through database/sql, and the value has to come back
// identical. Both pools have to agree on the layout or the migration silently shifts times.
func TestTimestampsWrittenByDatabaseSQLRoundTrip(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	a := newApp(t)

	owner := a.newUser(t, "owner")
	created := a.newShopMap(t, owner.ID, "corner shop", []product.Category{"dairy"}, nil)

	got, err := a.shopmaps.GetByID(ctx, created.ID)
	require.NoError(t, err)

	require.WithinDuration(t, created.CreatedAt.Time, got.CreatedAt.Time, time.Millisecond)
	require.Equal(t, created.CreatedAt.UTC(), got.CreatedAt.UTC())
}

// The stored text layout is a compatibility contract with every row already on disk, so pin it
// rather than trusting the driver to keep formatting the same way. The timestamps in
// testdata/legacy/gorm_v1.sql are in this exact layout, written by the pre-migration code.
func TestTimestampStorageLayout(t *testing.T) {
	t.Parallel()

	a := newApp(t)

	owner := a.newUser(t, "owner")
	listModel := a.newList(t, owner.ID, "groceries")
	created := a.newShopMap(t, owner.ID, "corner shop", []product.Category{"dairy"}, nil)

	const layout = "2006-01-02 15:04:05.999999999-07:00"

	for _, query := range []struct {
		sql string
		arg string
	}{
		{sql: `SELECT CAST(created_at AS TEXT) FROM product_lists WHERE id = ?`, arg: listModel.ID.String()},
		{sql: `SELECT CAST(created_at AS TEXT) FROM shop_maps WHERE id = ?`, arg: created.ID.String()},
	} {
		var written string
		require.NoError(t, a.sqlDB.QueryRow(query.sql, query.arg).Scan(&written))

		_, err := time.Parse(layout, written)
		require.NoError(t, err, "unexpected timestamp layout: %q", written)
	}
}

// IDs are canonical lowercase hyphenated UUID text everywhere. Switching to BLOB storage or
// to uppercase would break every existing row and every join.
func TestIdentifiersAreStoredAsCanonicalUUIDText(t *testing.T) {
	t.Parallel()

	a := newApp(t)

	owner := a.newUser(t, "owner")
	listModel := a.newList(t, owner.ID, "groceries")
	milk := a.newProduct(t, "milk", "dairy")
	a.addProducts(t, listModel.ID, owner.ID, milk)

	for _, query := range []string{
		`SELECT id FROM users`,
		`SELECT id FROM product_lists`,
		`SELECT id FROM products`,
		`SELECT list_id FROM product_list_states`,
		`SELECT product_id FROM product_list_states`,
		`SELECT user_id FROM product_list_members`,
	} {
		var value string
		require.NoError(t, a.sqlDB.QueryRow(query).Scan(&value), query)

		require.NoError(t, uuidLike(value), "%s returned %q", query, value)
		require.Equal(t, strings.ToLower(value), value, "%s returned %q", query, value)
		require.Len(t, value, len(uuid.Nil.String()))
	}
}

// Two writers contend for the same file. A write held open on one connection blocks the other
// for the driver's 5s busy timeout and then fails outright — the request does not queue, it
// errors.
//
// Collapsing the two connection pools into one did *not* remove this: database/sql hands the
// second writer a different connection from the same pool, and SQLite locks the file, not the
// pool. Only MaxOpenConns(1) would serialise it. Pinned here so that stays a deliberate choice.
//
// This test deliberately pays the full 5s timeout; it is the only slow test in the package.
func TestConcurrentWritersCollide(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	a := newApp(t)

	owner := a.newUser(t, "owner")

	tx, err := a.sqlDB.BeginTx(ctx, nil)
	require.NoError(t, err)

	defer func() { _ = tx.Rollback() }()

	// Take a RESERVED lock on the file.
	_, err = tx.ExecContext(ctx, `INSERT INTO users (id, role, login, hash) VALUES (?, ?, ?, ?)`,
		uuid.NewString(), 2, "concurrent", "hash")
	require.NoError(t, err)

	// No other connection can get in while that transaction is open.
	_, err = a.lists.Create(ctx, owner.ID, list.ListOptions{
		Status: list.ExecStatusPlanning,
		Title:  "blocked",
	})
	require.ErrorContains(t, err, "database is locked")
}

// The shopmap repo is the reference for how the migrated repos should handle IN-lists
// (sqlc.slice) and multi-table reads. Pin that a viewer lookup spanning several maps works.
func TestShopMapSliceQueries(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	a := newApp(t)

	owner := a.newUser(t, "owner")
	viewer := a.newUser(t, "viewer")

	first := a.newShopMap(t, owner.ID, "first",
		[]product.Category{"dairy", "bakery"}, []id.ID[user.User]{viewer.ID})
	second := a.newShopMap(t, owner.ID, "second",
		[]product.Category{"produce"}, []id.ID[user.User]{viewer.ID})

	maps, err := a.shopmaps.GetByUserID(ctx, viewer.ID)
	require.NoError(t, err)
	require.Len(t, maps, 2)

	byID := map[string]shopmap.ShopMap{}
	for _, m := range maps {
		byID[m.Title] = m
	}

	require.Equal(t, []product.Category{"dairy", "bakery"}, byID["first"].CategoryList)
	require.Equal(t, []product.Category{"produce"}, byID["second"].CategoryList)
	require.Equal(t, first.ID, byID["first"].ID)
	require.Equal(t, second.ID, byID["second"].ID)
}
