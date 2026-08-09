package functest_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/samber/mo"
	"github.com/stretchr/testify/require"

	"go-backend/internal/backend/list"
	"go-backend/internal/backend/product"
	"go-backend/internal/backend/user"
	"go-backend/pkg/id"
)

const legacyDump = "testdata/legacy/gorm_v1.sql"

// Stable handles for everything the scenario creates. The dump's UUIDs and timestamps are
// regenerated every time, so legacy_test.go has to find entities by these instead.
const (
	legacyOwnerLogin  = "alice"
	legacyEditorLogin = "bob"
	legacyViewerLogin = "carol"

	legacyListTitle      = "weekly groceries"
	legacyEmptyListTitle = "hardware"

	legacyMapTitle      = "corner shop"
	legacyOtherMapTitle = "big store"
)

// TestGenerateLegacyDump is not a test — it is the generator for testdata/legacy/gorm_v1.sql,
// and it only runs under -update:
//
//	go test ./internal/backend/functest/ -run TestGenerateLegacyDump -update
//
// It drives a deliberately broad scenario through the *current* GORM code and freezes the
// resulting database as SQL. That file is the contract for the migration: after the repos are
// rewritten, legacy_test.go reads the very same bytes with the new code and must get the same
// domain models back. Once GORM is deleted this generator goes with it; the dump stays.
func TestGenerateLegacyDump(t *testing.T) {
	if !*update {
		t.Skip("generator; run with -update to regenerate " + legacyDump)
	}

	ctx := context.Background()
	a := newApp(t)

	seedLegacyScenario(ctx, t, a)

	require.NoError(t, os.MkdirAll(filepath.Dir(legacyDump), 0o755))
	require.NoError(t, os.WriteFile(legacyDump, []byte(dumpDatabase(t, a.sqlDB)), 0o644))
}

// seedLegacyScenario exercises every column that matters: nullable and populated state fields,
// a replacement product, a non-trivial ordering, several roles, an empty list, favorites with
// and without products, and shop maps with ordered categories and viewers.
func seedLegacyScenario(ctx context.Context, t *testing.T, a *app) {
	t.Helper()

	alice := a.newUser(t, legacyOwnerLogin)
	bob := a.newUser(t, legacyEditorLogin)
	carol := a.newUser(t, legacyViewerLogin)

	milk := a.newProduct(t, "milk", "dairy", "bottle", "carton")
	bread := a.newProduct(t, "bread", "bakery")
	cheese := a.newProduct(t, "cheese", "dairy", "block")
	oat := a.newProduct(t, "oat milk", "dairy")

	groceries := a.newList(t, alice.ID, legacyListTitle)
	a.addMember(t, groceries.ID, alice.ID, bob.ID, list.MemberTypeEditor)
	a.addMember(t, groceries.ID, alice.ID, carol.ID, list.MemberTypeViewer)

	a.addProducts(t, groceries.ID, alice.ID, milk)
	a.addProducts(t, groceries.ID, alice.ID, bread)
	a.addProducts(t, groceries.ID, alice.ID, cheese)

	// A fully populated state.
	_, err := a.lists.UpdateProductState(ctx, groceries.ID, alice.ID, milk.ID, list.ProductStateOptions{
		Count:       mo.Some[int32](2),
		FormIndex:   mo.Some[int32](1),
		Status:      list.StateStatusTaken,
		Replacement: mo.None[list.ProductStateReplacement](),
	})
	require.NoError(t, err)

	// A state carrying a replacement, so the replacement_* columns are populated too.
	_, err = a.lists.UpdateProductState(ctx, groceries.ID, alice.ID, cheese.ID, list.ProductStateOptions{
		Count:     mo.None[int32](),
		FormIndex: mo.None[int32](),
		Status:    list.StateStatusReplaced,
		Replacement: mo.Some(list.ProductStateReplacement{
			Count:     mo.Some[int32](1),
			FormIndex: mo.None[int32](),
			Product:   oat,
		}),
	})
	require.NoError(t, err)

	// bread keeps every optional field NULL.

	// A non-identity ordering, so the index column is not simply 0,1,2 in insertion order.
	require.NoError(t, a.lists.ReoderStates(ginCtx(), alice.ID, groceries.ID,
		[]id.ID[product.Product]{cheese.ID, milk.ID, bread.ID}))

	// A list with no states and a single member.
	a.newList(t, bob.ID, legacyEmptyListTitle)

	// Favorites: the personal lists were created by user registration.
	aliceFavorites := a.personalList(t, alice)
	_, err = a.favorites.AddProducts(ctx, aliceFavorites.ID, alice.ID,
		[]id.ID[product.Product]{milk.ID, cheese.ID})
	require.NoError(t, err)

	bobFavorites := a.personalList(t, bob)
	_, err = a.favorites.AddProducts(ctx, bobFavorites.ID, bob.ID, []id.ID[product.Product]{bread.ID})
	require.NoError(t, err)

	// carol's personal list stays empty.

	a.newShopMap(t, alice.ID, legacyMapTitle,
		[]product.Category{"dairy", "bakery", "produce"}, []id.ID[user.User]{bob.ID})
	a.newShopMap(t, bob.ID, legacyOtherMapTitle, []product.Category{"produce"}, nil)
}

// dumpDatabase renders the whole database as SQL: the DDL exactly as SQLite stored it, then
// one INSERT per row.
//
// Values go through SQLite's own quote(), which emits a correctly escaped literal for text,
// blobs, numbers and NULL alike. That keeps the dump byte-exact — no Go driver ever converts
// the values, so the timestamps in the file are the literal strings GORM wrote.
func dumpDatabase(t *testing.T, db *sql.DB) string {
	t.Helper()

	var out strings.Builder

	out.WriteString("-- Generated by TestGenerateLegacyDump (go test ./internal/backend/functest -update).\n")
	out.WriteString("-- A snapshot of a database written by the GORM-era code. Do not edit by hand.\n\n")

	for _, ddl := range legacySchema(t, db) {
		out.WriteString(ddl)
		out.WriteString(";\n")
	}

	out.WriteString("\n")

	for _, table := range legacyTables(t, db) {
		rows := dumpTable(t, db, table)
		if len(rows) == 0 {
			continue
		}

		out.WriteString("-- " + table + "\n")
		for _, row := range rows {
			out.WriteString(row)
			out.WriteString("\n")
		}

		out.WriteString("\n")
	}

	return out.String()
}

// legacySchema returns every CREATE statement, tables before indexes.
func legacySchema(t *testing.T, db *sql.DB) []string {
	t.Helper()

	rows, err := db.Query(`
		SELECT sql FROM sqlite_master
		WHERE sql IS NOT NULL AND name NOT LIKE 'sqlite_%'
		ORDER BY CASE type WHEN 'table' THEN 0 ELSE 1 END, name`)
	require.NoError(t, err)

	defer func() { require.NoError(t, rows.Close()) }()

	var res []string

	for rows.Next() {
		var ddl string
		require.NoError(t, rows.Scan(&ddl))

		res = append(res, ddl)
	}

	require.NoError(t, rows.Err())

	return res
}

func legacyTables(t *testing.T, db *sql.DB) []string {
	t.Helper()

	rows, err := db.Query(`
		SELECT name FROM sqlite_master
		WHERE type = 'table' AND name NOT LIKE 'sqlite_%'
		ORDER BY name`)
	require.NoError(t, err)

	defer func() { require.NoError(t, rows.Close()) }()

	var res []string

	for rows.Next() {
		var name string
		require.NoError(t, rows.Scan(&name))

		res = append(res, name)
	}

	require.NoError(t, rows.Err())

	return res
}

func dumpTable(t *testing.T, db *sql.DB, table string) []string {
	t.Helper()

	columns := tableColumns(t, db, table)
	require.NotEmpty(t, columns, "table %s has no columns", table)

	quoted := make([]string, 0, len(columns))
	selected := make([]string, 0, len(columns))

	for _, column := range columns {
		quoted = append(quoted, `"`+column+`"`)
		selected = append(selected, `quote("`+column+`")`)
	}

	// rowid gives a stable order for tables without a meaningful sort key.
	rows, err := db.Query(`SELECT ` + strings.Join(selected, ", ") + ` FROM "` + table + `" ORDER BY rowid`)
	require.NoError(t, err)

	defer func() { require.NoError(t, rows.Close()) }()

	var res []string

	for rows.Next() {
		values := make([]string, len(columns))
		targets := make([]any, len(columns))

		for i := range values {
			targets[i] = &values[i]
		}

		require.NoError(t, rows.Scan(targets...))

		res = append(res, fmt.Sprintf(`INSERT INTO "%s" (%s) VALUES (%s);`,
			table, strings.Join(quoted, ", "), strings.Join(values, ", ")))
	}

	require.NoError(t, rows.Err())

	return res
}

func tableColumns(t *testing.T, db *sql.DB, table string) []string {
	t.Helper()

	rows, err := db.Query(`SELECT name FROM pragma_table_info(?)`, table)
	require.NoError(t, err)

	defer func() { require.NoError(t, rows.Close()) }()

	var res []string

	for rows.Next() {
		var name string
		require.NoError(t, rows.Scan(&name))

		res = append(res, name)
	}

	require.NoError(t, rows.Err())

	return res
}
