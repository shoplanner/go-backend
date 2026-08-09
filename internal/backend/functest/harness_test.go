package functest_test

import (
	"context"
	"database/sql"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	_ "github.com/mattn/go-sqlite3"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	favoriteRepo "go-backend/internal/backend/favorite/repo"
	favoriteService "go-backend/internal/backend/favorite/service"
	"go-backend/internal/backend/list"
	listRepo "go-backend/internal/backend/list/repo"
	listService "go-backend/internal/backend/list/service"
	productRepo "go-backend/internal/backend/product/repo"
	productService "go-backend/internal/backend/product/service"
	shopMapRepo "go-backend/internal/backend/shopmap/repo"
	shopMapService "go-backend/internal/backend/shopmap/service"
	userRepo "go-backend/internal/backend/user/repo"
	userService "go-backend/internal/backend/user/service"
	"go-backend/pkg/bd"
	"go-backend/pkg/hashing"
	"go-backend/pkg/id"
)

// app is the whole backend minus the HTTP layer, wired exactly like cmd/backend/main.go: one
// database/sql pool over one SQLite file, opened with a bare path and no pragmas.
//
// Before the sqlc migration there were two pools here, a database/sql one and a GORM one over
// the same file, because the repos were split between them. That is gone; what it used to make
// observable — timestamp format skew between the two drivers, and SQLITE_BUSY between the two
// pools — is covered by TestTimestampStorageLayout and TestConcurrentWritersCollide.
type app struct {
	path  string
	sqlDB *sql.DB

	users     *userService.Service
	products  *productService.Service
	favorites *favoriteService.Service
	lists     *listService.Service
	shopmaps  *shopMapService.Service
}

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	os.Exit(m.Run())
}

// ginCtx builds the *gin.Context that list.Service.ReoderStates insists on. It is used purely
// as a context.Context there; nothing reads the request.
func ginCtx() *gin.Context {
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())

	return ctx
}

// newApp brings the stack up over an empty database in a fresh temp dir.
func newApp(t *testing.T) *app {
	t.Helper()

	return openApp(t, filepath.Join(t.TempDir(), "shoplanner.db"))
}

// openApp brings the stack up over dbPath, which may already contain data written by an
// earlier run (or by the frozen legacy dump). Every repo constructor is expected to be
// idempotent on an existing file.
func openApp(t *testing.T, dbPath string) *app {
	t.Helper()

	ctx := context.Background()
	log := zerolog.Nop()

	sqlDB, err := sql.Open("sqlite3", dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })

	// Construction order mirrors main.go: users first, because the member tables of the
	// favorites and list repos declare foreign keys against it.
	userStore, err := userRepo.NewRepo(ctx, bd.NewDB(sqlDB, log))
	require.NoError(t, err)

	shopMapStore, err := shopMapRepo.NewShopMapRepo(ctx, sqlDB)
	require.NoError(t, err)

	productStore, err := productRepo.NewRepo(ctx, sqlDB)
	require.NoError(t, err)

	favoriteStore, err := favoriteRepo.NewRepo(ctx, sqlDB)
	require.NoError(t, err)

	listStore, err := listRepo.NewRepo(ctx, sqlDB)
	require.NoError(t, err)

	users := userService.NewService(userStore, hashing.HashMaster{})

	return &app{
		path:  dbPath,
		sqlDB: sqlDB,

		users:    users,
		products: productService.NewService(productStore),
		// NewService registers itself as a user.Subscriber, so creating a user through
		// app.users side-effects a personal favorites list into existence.
		favorites: favoriteService.NewService(favoriteStore, users),
		lists:     listService.NewService(listStore, log),
		shopmaps:  shopMapService.NewService(log, users, shopMapStore),
	}
}

// stateIndexes returns the raw `index` column of a list's states, sorted. The read path
// rebuilds ordering by writing into states[index], so this has to stay a dense 0..n-1 range.
func (a *app) stateIndexes(t *testing.T, listID id.ID[list.ProductList]) []int64 {
	t.Helper()

	rows, err := a.sqlDB.Query(
		"SELECT `index` FROM product_list_states WHERE list_id = ? ORDER BY `index`", listID.String())
	require.NoError(t, err)

	defer func() { require.NoError(t, rows.Close()) }()

	var res []int64

	for rows.Next() {
		var idx int64
		require.NoError(t, rows.Scan(&idx))

		res = append(res, idx)
	}

	require.NoError(t, rows.Err())

	return res
}

// count runs a scalar COUNT(*)-shaped query against the raw database/sql handle. Raw SQL is
// how these tests inspect what actually landed on disk, independent of any ORM's read path.
func (a *app) count(t *testing.T, query string, args ...any) int {
	t.Helper()

	var n int
	require.NoError(t, a.sqlDB.QueryRow(query, args...).Scan(&n))

	return n
}
