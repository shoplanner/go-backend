package functest_test

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/samber/lo"
	"github.com/samber/mo"
	"github.com/stretchr/testify/require"

	"go-backend/internal/backend/list"
	"go-backend/internal/backend/product"
	"go-backend/internal/backend/user"
	"go-backend/pkg/id"
)

// update regenerates every committed artifact under testdata/ instead of asserting against it:
//
//	go test ./internal/backend/functest/ -update
//
// Regenerated files must be reviewed by hand before committing — they are the contract the
// sqlc migration will be checked against.
var update = flag.Bool("update", false, "rewrite golden files and the legacy dump instead of comparing")

const (
	testPassword = "pas$w0rD"
	goldenDir    = "testdata/golden"
)

// --- domain builders -------------------------------------------------------------------
//
// Everything is created through the services, never through the repos directly: these tests
// are about the behaviour a caller observes, which is what has to survive the migration.

func (a *app) newUser(t *testing.T, login string) user.User {
	t.Helper()

	model, err := a.users.Create(context.Background(), user.CreateOptions{
		Login:    user.Login(login),
		Password: testPassword,
	})
	require.NoError(t, err)

	return model
}

// newProduct creates a product; an empty category means "no category" (mo.None).
func (a *app) newProduct(t *testing.T, name, category string, forms ...string) product.Product {
	t.Helper()

	model, err := a.products.Create(context.Background(), product.Options{
		Name:     product.Name(name),
		Category: mo.EmptyableToOption(product.Category(category)),
		Forms:    lo.Map(forms, func(f string, _ int) product.Form { return product.Form(f) }),
	})
	require.NoError(t, err)

	return model
}

func (a *app) newList(t *testing.T, owner id.ID[user.User], title string) list.ProductList {
	t.Helper()

	model, err := a.lists.Create(context.Background(), owner, list.ListOptions{
		Status: list.ExecStatusPlanning,
		Title:  title,
	})
	require.NoError(t, err)

	return model
}

// addMember appends a single member with the given role.
func (a *app) addMember(
	t *testing.T,
	listID id.ID[list.ProductList],
	actor, member id.ID[user.User],
	role list.MemberType,
) list.ProductList {
	t.Helper()

	model, err := a.lists.AppendMembers(context.Background(), listID, actor, []list.MemberOptions{
		{UserID: member, Role: role},
	})
	require.NoError(t, err)

	return model
}

// addProducts appends products with zero state options, in the given order.
func (a *app) addProducts(
	t *testing.T,
	listID id.ID[list.ProductList],
	actor id.ID[user.User],
	products ...product.Product,
) list.ProductList {
	t.Helper()

	states := make(map[id.ID[product.Product]]list.ProductStateOptions, len(products))
	for _, p := range products {
		states[p.ID] = zeroStateOptions()
	}

	model, err := a.lists.AppendProducts(context.Background(), listID, actor, states)
	require.NoError(t, err)

	return model
}

func zeroStateOptions() list.ProductStateOptions {
	return list.ProductStateOptions{
		Count:       mo.None[int32](),
		FormIndex:   mo.None[int32](),
		Status:      list.StateStatusWaiting,
		Replacement: mo.None[list.ProductStateReplacement](),
	}
}

// --- golden snapshots ------------------------------------------------------------------

// goldenJSON compares v against testdata/golden/<name>.json in a canonical, reproducible form.
//
// Hand-written field assertions systematically miss the things a storage rewrite actually
// breaks: nil versus empty slice, mo.None versus mo.Some(""), a dropped Preload, a lost time
// zone, a reordered collection. Snapshotting the whole marshalled model catches all of them.
func goldenJSON(t *testing.T, name string, v any) {
	t.Helper()

	got := canonicalJSON(t, v)
	path := filepath.Join(goldenDir, name+".json")

	if *update {
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, got, 0o644))

		return
	}

	want, err := os.ReadFile(path)
	require.NoError(t, err, "golden file missing; rerun with -update")
	require.Equal(t, string(want), string(got), "golden mismatch for %s", name)
}

// canonicalJSON marshals v, then rewrites the values that cannot be reproduced across runs:
// UUIDs become <uuid:N> and timestamps become <ts>. UUIDs keep their identity — the same UUID
// always gets the same label — so relationships between entities stay visible in the snapshot.
// The zero time is labelled separately, because "no timestamp" is meaningful in this codebase
// (list states carry a zero-valued embedded product).
func canonicalJSON(t *testing.T, v any) []byte {
	t.Helper()

	raw, err := json.Marshal(v)
	require.NoError(t, err)

	var tree any
	require.NoError(t, json.Unmarshal(raw, &tree))

	var buf bytes.Buffer

	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	// Keep the <uuid:N> / <ts> placeholders readable instead of <-escaped.
	enc.SetEscapeHTML(false)
	require.NoError(t, enc.Encode(normalize(tree, map[string]string{})))

	return buf.Bytes()
}

func normalize(node any, seen map[string]string) any {
	switch typed := node.(type) {
	case map[string]any:
		// Walk keys in sorted order so UUID labels are assigned deterministically.
		keys := lo.Keys(typed)
		sort.Strings(keys)

		res := make(map[string]any, len(typed))
		for _, k := range keys {
			res[k] = normalize(typed[k], seen)
		}

		return res
	case []any:
		res := make([]any, len(typed))
		for i, item := range typed {
			res[i] = normalize(item, seen)
		}

		return res
	case string:
		return normalizeString(typed, seen)
	default:
		return node
	}
}

func normalizeString(s string, seen map[string]string) string {
	if parsed, err := uuid.Parse(s); err == nil {
		if parsed == uuid.Nil {
			return "<uuid:nil>"
		}

		label, found := seen[s]
		if !found {
			label = "<uuid:" + strconv.Itoa(len(seen)) + ">"
			seen[s] = label
		}

		return label
	}

	if parsed, err := time.Parse(time.RFC3339Nano, s); err == nil {
		if parsed.IsZero() {
			return "<ts:zero>"
		}

		return "<ts>"
	}

	return s
}

// uuidLike reports whether a raw column value is a canonical UUID string. Used by the tests
// that inspect foreign key columns directly.
func uuidLike(s string) error {
	_, err := uuid.Parse(s)

	return err
}

// parseProductID turns a raw id column into the phantom-typed id the services expect.
func parseProductID(raw string) (id.ID[product.Product], error) {
	parsed, err := uuid.Parse(raw)
	if err != nil {
		return id.ID[product.Product]{}, fmt.Errorf("parsing product id %q: %w", raw, err)
	}

	return id.ID[product.Product]{UUID: parsed}, nil
}

// requireRecent asserts a timestamp was actually populated. goldenJSON erases timestamp
// values, so anything that cares about them has to check them separately.
func requireRecent(t *testing.T, ts time.Time) {
	t.Helper()

	require.False(t, ts.IsZero(), "timestamp is zero")
	require.WithinDuration(t, time.Now(), ts, time.Minute)
}

// requireRecent2000s is the weaker check for timestamps that came out of the frozen legacy
// dump: they were recorded whenever the generator last ran, so all that can be asserted is
// that they decoded into a plausible time rather than a zero value.
func requireRecent2000s(t *testing.T, ts time.Time) {
	t.Helper()

	require.False(t, ts.IsZero(), "timestamp is zero")
	require.True(t, ts.After(time.Date(2000, time.January, 1, 0, 0, 0, 0, time.UTC)),
		"timestamp %s did not decode into a plausible value", ts)
}

// dumpSchema returns the physical schema of the database as a stable, diffable string.
func dumpSchema(t *testing.T, a *app) string {
	t.Helper()

	rows, err := a.sqlDB.Query(`
		SELECT type, name, COALESCE(sql, '')
		FROM sqlite_master
		WHERE name NOT LIKE 'sqlite_%'
		ORDER BY type, name`)
	require.NoError(t, err)

	defer func() { require.NoError(t, rows.Close()) }()

	var out strings.Builder

	for rows.Next() {
		var objType, name, ddl string
		require.NoError(t, rows.Scan(&objType, &name, &ddl))

		fmt.Fprintf(&out, "-- %s %s\n%s;\n\n", objType, name, normalizeDDL(ddl))
	}

	require.NoError(t, rows.Err())

	return out.String()
}

// normalizeDDL makes a CREATE statement comparable across runs.
//
// Three things vary that should not: whitespace and identifier quoting (GORM backquotes every
// name and packs the statement onto one line, sqlc quotes nothing and indents), and the order
// of the trailing CONSTRAINT clauses — AutoMigrate walks its relations through a map, so the
// same model emits its foreign keys in a different order on every run. Normalising all three
// keeps the snapshot about structure instead of about which generator wrote the table, which
// is the only question the migration needs it to answer.
func normalizeDDL(ddl string) string {
	collapsed := strings.Join(strings.Fields(unquoteIdentifiers(ddl)), " ")

	for _, pair := range [][2]string{{"( ", "("}, {" )", ")"}, {" ,", ","}, {", ", ","}} {
		collapsed = strings.ReplaceAll(collapsed, pair[0], pair[1])
	}

	const marker = ",CONSTRAINT "

	start := strings.Index(collapsed, marker)
	if start == -1 {
		return collapsed
	}

	head, tail := collapsed[:start], collapsed[start+1:]

	suffix := ""
	if strings.HasSuffix(tail, ")") {
		tail, suffix = tail[:len(tail)-1], ")"
	}

	clauses := strings.Split(tail, marker)
	for i := 1; i < len(clauses); i++ {
		clauses[i] = strings.TrimPrefix(marker, ",") + clauses[i]
	}

	sort.Strings(clauses)

	return head + "," + strings.Join(clauses, ",") + suffix
}

// unquoteIdentifiers drops the backquotes GORM puts around every identifier and the double
// quotes SQLite adds when it rewrites a table. No value in this schema contains either
// character, so a blind strip is safe.
func unquoteIdentifiers(ddl string) string {
	return strings.NewReplacer("`", "", `"`, "").Replace(ddl)
}
