package functest_test

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/samber/lo"
	"github.com/samber/mo"
	"github.com/stretchr/testify/require"

	"go-backend/internal/backend/favorite"
	"go-backend/internal/backend/list"
	"go-backend/internal/backend/product"
	"go-backend/internal/backend/shopmap"
	"go-backend/internal/backend/user"
	"go-backend/pkg/id"
)

// This file is the point of the whole package.
//
// testdata/legacy/gorm_v1.sql is a byte-exact snapshot of a database written by the GORM-era
// code. Every test here loads it and reads it back through the services. Today that exercises
// the GORM repos against their own output, which is unremarkable; after the rewrite the same
// bytes will be read by sqlc, and these assertions are what proves nobody's data broke.
//
// Nothing here may depend on a UUID or a timestamp from the dump — those are regenerated
// whenever the generator runs. Entities are located by login, title or product name.

// loadLegacyDB materialises the frozen dump into a fresh file and brings the stack up on it.
func loadLegacyDB(t *testing.T) *app {
	t.Helper()

	script, err := os.ReadFile(legacyDump)
	require.NoError(t, err, "run: go test ./internal/backend/functest -run TestGenerateLegacyDump -update")

	path := filepath.Join(t.TempDir(), "legacy.db")

	raw, err := sql.Open("sqlite3", path)
	require.NoError(t, err)

	_, err = raw.Exec(string(script))
	require.NoError(t, err)
	require.NoError(t, raw.Close())

	return openApp(t, path)
}

func legacyUser(t *testing.T, a *app, login string) user.User {
	t.Helper()

	users, err := a.users.GetAllUsers(context.Background())
	require.NoError(t, err)

	found, ok := lo.Find(users, func(u user.User) bool { return u.Login == user.Login(login) })
	require.True(t, ok, "user %q is missing from the legacy dump", login)

	return found
}

func legacyList(t *testing.T, a *app, owner user.User, title string) list.ProductList {
	t.Helper()

	lists, err := a.lists.GetByUserID(context.Background(), owner.ID)
	require.NoError(t, err)

	found, ok := lo.Find(lists, func(l list.ProductList) bool { return l.Title == title })
	require.True(t, ok, "list %q is missing from the legacy dump", title)

	return found
}

func legacyMap(t *testing.T, a *app, owner user.User, title string) shopmap.ShopMap {
	t.Helper()

	maps, err := a.shopmaps.GetByUserID(context.Background(), owner.ID)
	require.NoError(t, err)

	found, ok := lo.Find(maps, func(m shopmap.ShopMap) bool { return m.Title == title })
	require.True(t, ok, "shop map %q is missing from the legacy dump", title)

	return found
}

func legacyProductID(t *testing.T, a *app, name string) id.ID[product.Product] {
	t.Helper()

	var raw string
	require.NoError(t, a.sqlDB.QueryRow(`SELECT id FROM products WHERE name = ?`, name).Scan(&raw))

	parsed, err := parseProductID(raw)
	require.NoError(t, err)

	return parsed
}

// Bringing the stack up on an existing database must be a no-op as far as the file is
// concerned. If a repo constructor ever starts altering the schema on boot, every deployed
// installation is being migrated silently on restart — and this is where that shows up.
func TestLegacyBootDoesNotAlterTheSchema(t *testing.T) {
	t.Parallel()

	a := loadLegacyDB(t)
	before := dumpSchema(t, a)

	second := openApp(t, a.path)
	require.Equal(t, before, dumpSchema(t, second))

	// And it matches the schema a fresh install produces, so there is only one shape to
	// support rather than two.
	require.Equal(t, dumpSchema(t, newApp(t)), before)
}

func TestLegacyUsersAreReadable(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	a := loadLegacyDB(t)

	users, err := a.users.GetAllUsers(ctx)
	require.NoError(t, err)
	require.Len(t, users, 3)

	alice := legacyUser(t, a, legacyOwnerLogin)
	require.Equal(t, user.RoleUser, alice.Role)
	require.NotEmpty(t, alice.PasswordHash)

	byID, err := a.users.GetByID(ctx, alice.ID)
	require.NoError(t, err)
	require.Equal(t, alice, byID)

	// The stored bcrypt hash still verifies, so credentials survive the migration.
	authorized, err := a.users.ValidatePassword(ctx, alice.Login, testPassword)
	require.NoError(t, err)
	require.Equal(t, alice.ID, authorized.ID)
}

// The richest read path in the codebase: a list plus its members (joined to users for the
// display name) plus its states plus each state's product with category and forms plus the
// replacement product. Everything below has to come out of a plain SQLite file that the new
// code did not write.
func TestLegacyListIsFullyReadable(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	a := loadLegacyDB(t)

	alice := legacyUser(t, a, legacyOwnerLogin)
	bob := legacyUser(t, a, legacyEditorLogin)
	carol := legacyUser(t, a, legacyViewerLogin)

	groceries := legacyList(t, a, alice, legacyListTitle)

	got, err := a.lists.GetByID(ctx, groceries.ID, alice.ID)
	require.NoError(t, err)

	require.Equal(t, legacyListTitle, got.Title)
	require.Equal(t, list.ExecStatusPlanning, got.Status)

	// Members, with their roles and their logins resolved through the users table.
	require.Len(t, got.Members, 3)
	roles := map[user.Login]list.MemberType{}

	for _, member := range got.Members {
		roles[member.UserName] = member.Role
	}

	require.Equal(t, map[user.Login]list.MemberType{
		legacyOwnerLogin:  list.MemberTypeOwner,
		legacyEditorLogin: list.MemberTypeEditor,
		legacyViewerLogin: list.MemberTypeViewer,
	}, roles)

	// The stored ordering was cheese, milk, bread — not insertion order.
	require.Equal(t, []product.Name{"cheese", "milk", "bread"},
		lo.Map(got.States, func(s list.ProductState, _ int) product.Name { return s.Product.Name }))

	cheese, milk, bread := got.States[0], got.States[1], got.States[2]

	// A state with a replacement.
	require.Equal(t, list.StateStatusReplaced, cheese.Status)
	replacement, present := cheese.Replacement.Get()
	require.True(t, present)
	require.Equal(t, product.Name("oat milk"), replacement.Product.Name)
	require.Equal(t, mo.Some[int32](1), replacement.Count)
	require.True(t, replacement.FormIndex.IsAbsent())

	// A fully populated state.
	require.Equal(t, list.StateStatusTaken, milk.Status)
	require.Equal(t, mo.Some[int32](2), milk.Count)
	require.Equal(t, mo.Some[int32](1), milk.FormIndex)
	require.Equal(t, mo.Some(product.Category("dairy")), milk.Product.Category)
	require.Equal(t, []product.Form{"bottle", "carton"}, milk.Product.Forms)

	// A state where every optional column is NULL.
	require.Equal(t, list.StateStatusWaiting, bread.Status)
	require.True(t, bread.Count.IsAbsent())
	require.True(t, bread.FormIndex.IsAbsent())
	require.True(t, bread.Replacement.IsAbsent())
	require.Empty(t, bread.Product.Forms)

	// Timestamps written by the old code must come back as real times, not zero values.
	requireRecent2000s(t, got.CreatedAt.Time)
	requireRecent2000s(t, milk.CreatedAt.Time)

	goldenJSON(t, "legacy_list", got)

	// Every member can read it, according to their role.
	for _, member := range []user.User{alice, bob, carol} {
		_, err = a.lists.GetByID(ctx, groceries.ID, member.ID)
		require.NoError(t, err, "%s must still be able to read the list", member.Login)
	}
}

func TestLegacyEmptyListIsReadable(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	a := loadLegacyDB(t)

	bob := legacyUser(t, a, legacyEditorLogin)
	hardware := legacyList(t, a, bob, legacyEmptyListTitle)

	got, err := a.lists.GetByID(ctx, hardware.ID, bob.ID)
	require.NoError(t, err)
	require.Empty(t, got.States)
	require.Len(t, got.Members, 1)
	require.Equal(t, list.MemberTypeOwner, got.Members[0].Role)
}

func TestLegacyListMembershipScoping(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	a := loadLegacyDB(t)

	alice := legacyUser(t, a, legacyOwnerLogin)
	bob := legacyUser(t, a, legacyEditorLogin)
	carol := legacyUser(t, a, legacyViewerLogin)

	aliceLists, err := a.lists.GetByUserID(ctx, alice.ID)
	require.NoError(t, err)
	require.Equal(t, []string{legacyListTitle},
		lo.Map(aliceLists, func(l list.ProductList, _ int) string { return l.Title }))

	bobLists, err := a.lists.GetByUserID(ctx, bob.ID)
	require.NoError(t, err)
	require.ElementsMatch(t, []string{legacyListTitle, legacyEmptyListTitle},
		lo.Map(bobLists, func(l list.ProductList, _ int) string { return l.Title }))

	carolLists, err := a.lists.GetByUserID(ctx, carol.ID)
	require.NoError(t, err)
	require.Len(t, carolLists, 1)
}

func TestLegacyFavoritesAreReadable(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	a := loadLegacyDB(t)

	alice := legacyUser(t, a, legacyOwnerLogin)
	carol := legacyUser(t, a, legacyViewerLogin)

	aliceLists, err := a.favorites.GetListsByUserID(ctx, alice.ID)
	require.NoError(t, err)
	require.Len(t, aliceLists, 1)
	require.Equal(t, favorite.ListTypePersonal, aliceLists[0].Type)

	names := lo.Map(aliceLists[0].Products, func(f favorite.Favorite, _ int) product.Name {
		return f.Product.Name
	})
	require.ElementsMatch(t, []product.Name{"milk", "cheese"}, names)

	// Preloaded product details survive.
	milk, ok := lo.Find(aliceLists[0].Products, func(f favorite.Favorite) bool {
		return f.Product.Name == "milk"
	})
	require.True(t, ok)
	require.Equal(t, mo.Some(product.Category("dairy")), milk.Product.Category)
	require.Equal(t, []product.Form{"bottle", "carton"}, milk.Product.Forms)

	goldenJSON(t, "legacy_favorites", aliceLists[0])

	// A list that was left empty stays empty.
	carolLists, err := a.favorites.GetListsByUserID(ctx, carol.ID)
	require.NoError(t, err)
	require.Len(t, carolLists, 1)
	require.Empty(t, carolLists[0].Products)
}

func TestLegacyProductsAreReadable(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	a := loadLegacyDB(t)

	milkID := legacyProductID(t, a, "milk")

	milk, err := a.products.ID(ctx, milkID)
	require.NoError(t, err)
	require.Equal(t, product.Name("milk"), milk.Name)
	require.Equal(t, mo.Some(product.Category("dairy")), milk.Category)
	require.Equal(t, []product.Form{"bottle", "carton"}, milk.Forms)

	breadID := legacyProductID(t, a, "bread")

	bread, err := a.products.ID(ctx, breadID)
	require.NoError(t, err)
	require.Equal(t, mo.Some(product.Category("bakery")), bread.Category)
	require.Empty(t, bread.Forms)

	// The two dairy products still share one category row.
	require.Equal(t, 1, a.count(t, `SELECT count(*) FROM product_categories WHERE name = 'dairy'`))
}

func TestLegacyShopMapsAreReadable(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	a := loadLegacyDB(t)

	alice := legacyUser(t, a, legacyOwnerLogin)
	bob := legacyUser(t, a, legacyEditorLogin)

	corner := legacyMap(t, a, alice, legacyMapTitle)

	got, err := a.shopmaps.GetByID(ctx, corner.ID)
	require.NoError(t, err)
	require.Equal(t, alice.ID, got.OwnerID)
	require.Equal(t, []product.Category{"dairy", "bakery", "produce"}, got.CategoryList,
		"the stored category order must be preserved")
	require.Equal(t, []id.ID[user.User]{bob.ID}, got.ViewerIDList)

	goldenJSON(t, "legacy_shopmap", got)

	// bob owns one map and views another.
	bobMaps, err := a.shopmaps.GetByUserID(ctx, bob.ID)
	require.NoError(t, err)
	require.ElementsMatch(t,
		[]string{legacyMapTitle, legacyOtherMapTitle},
		lo.Map(bobMaps, func(m shopmap.ShopMap, _ int) string { return m.Title }),
	)
}

// Reading legacy data is half the contract; writing on top of it is the other half. New rows
// have to coexist with old ones in the same tables, and the old rows have to survive.
func TestLegacyDataCanBeWrittenTo(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	a := loadLegacyDB(t)

	alice := legacyUser(t, a, legacyOwnerLogin)
	groceries := legacyList(t, a, alice, legacyListTitle)

	// A brand new product joins an old list.
	butter := a.newProduct(t, "butter", "dairy", "pack")
	a.addProducts(t, groceries.ID, alice.ID, butter)

	got, err := a.lists.GetByID(ctx, groceries.ID, alice.ID)
	require.NoError(t, err)
	require.Equal(t, []product.Name{"cheese", "milk", "bread", "butter"},
		lo.Map(got.States, func(s list.ProductState, _ int) product.Name { return s.Product.Name }),
		"the new state is appended without disturbing the stored order")
	require.Equal(t, []int64{0, 1, 2, 3}, a.stateIndexes(t, groceries.ID))

	// An old state can still be updated.
	_, err = a.lists.UpdateProductState(ctx, groceries.ID, alice.ID,
		legacyProductID(t, a, "bread"), list.ProductStateOptions{
			Count:       mo.Some[int32](5),
			FormIndex:   mo.None[int32](),
			Status:      list.StateStatusMissed,
			Replacement: mo.None[list.ProductStateReplacement](),
		})
	require.NoError(t, err)

	// A new member joins an old list.
	dave := a.newUser(t, "dave")
	a.addMember(t, groceries.ID, alice.ID, dave.ID, list.MemberTypeEditor)

	after, err := a.lists.GetByID(ctx, groceries.ID, dave.ID)
	require.NoError(t, err)
	require.Len(t, after.Members, 4)

	bread, ok := lo.Find(after.States, func(s list.ProductState) bool { return s.Product.Name == "bread" })
	require.True(t, ok)
	require.Equal(t, mo.Some[int32](5), bread.Count)
	require.Equal(t, list.StateStatusMissed, bread.Status)

	// A new favourite lands in an old favorites list.
	aliceFavorites, err := a.favorites.GetListsByUserID(ctx, alice.ID)
	require.NoError(t, err)
	require.Len(t, aliceFavorites, 1)

	_, err = a.favorites.AddProducts(ctx, aliceFavorites[0].ID, alice.ID, []id.ID[product.Product]{butter.ID})
	require.NoError(t, err)

	refreshed, err := a.favorites.GetListsByUserID(ctx, alice.ID)
	require.NoError(t, err)
	require.Len(t, refreshed[0].Products, 3)

	// And an old shop map can be reordered.
	corner := legacyMap(t, a, alice, legacyMapTitle)

	reordered, err := a.shopmaps.ReorderMap(ctx, corner.ID, alice.ID,
		[]product.Category{"produce", "dairy", "bakery"})
	require.NoError(t, err)
	require.Equal(t, []product.Category{"produce", "dairy", "bakery"}, reordered.CategoryList)
}
