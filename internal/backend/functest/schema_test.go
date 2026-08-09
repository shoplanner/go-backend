package functest_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

const schemaGolden = "testdata/schema_gorm.sql"

// TestSchemaSnapshot pins the physical schema the current repo constructors produce.
//
// There are no migration files in this project: the schema is whatever AutoMigrate and the
// sqlc Init* queries have ever created against a given file. That makes this snapshot the
// only written-down description of the on-disk format, and the loudest possible alarm for a
// storage rewrite that changes it. A diff here is not automatically a failure — it is a
// decision: either the new schema is compatible with existing files, or a migration is owed.
func TestSchemaSnapshot(t *testing.T) {
	t.Parallel()

	got := dumpSchema(t, newApp(t))

	if *update {
		require.NoError(t, os.MkdirAll(filepath.Dir(schemaGolden), 0o755))
		require.NoError(t, os.WriteFile(schemaGolden, []byte(got), 0o644))

		return
	}

	want, err := os.ReadFile(schemaGolden)
	require.NoError(t, err, "schema golden missing; rerun with -update")
	require.Equal(t, string(want), got)
}

// TestSchemaIsStableAcrossRestarts guards the property the legacy tests depend on: bringing
// the stack up a second time over an existing file must not change anything. If AutoMigrate
// ever starts adding a column or an index on reopen, every deployed database is silently
// being rewritten on boot.
func TestSchemaIsStableAcrossRestarts(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "shoplanner.db")

	first := dumpSchema(t, openApp(t, path))
	second := dumpSchema(t, openApp(t, path))

	require.Equal(t, first, second)
}
