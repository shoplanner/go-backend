package functest_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"go-backend/internal/backend/product"
	"go-backend/internal/backend/shopmap"
	"go-backend/internal/backend/user"
	"go-backend/pkg/id"
	"go-backend/pkg/myerr"
)

// shopmap is already sqlc-backed, so these tests are not about a pending rewrite — they are
// the baseline for the domain that shares the database file with everything else, and the
// reference for how the migrated repos are expected to look (explicit ordinal column, IN-list
// queries via sqlc.slice, per-row inserts inside a transaction).

func (a *app) newShopMap(
	t *testing.T,
	owner id.ID[user.User],
	title string,
	categories []product.Category,
	viewers []id.ID[user.User],
) shopmap.ShopMap {
	t.Helper()

	model, err := a.shopmaps.Create(context.Background(), owner, shopmap.Options{
		CategoryList: categories,
		ViewerIDList: viewers,
		Title:        title,
	})
	require.NoError(t, err)

	return model
}

func TestShopMapCreateAndRead(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	a := newApp(t)

	owner := a.newUser(t, "owner")
	viewer := a.newUser(t, "viewer")

	categories := []product.Category{"dairy", "bakery", "produce"}
	created := a.newShopMap(t, owner.ID, "corner shop", categories, []id.ID[user.User]{viewer.ID})

	got, err := a.shopmaps.GetByID(ctx, created.ID)
	require.NoError(t, err)
	require.Equal(t, created.ID, got.ID)
	require.Equal(t, owner.ID, got.OwnerID)
	require.Equal(t, "corner shop", got.Title)
	require.Equal(t, categories, got.CategoryList, "category order is significant and must round-trip")
	require.Equal(t, []id.ID[user.User]{viewer.ID}, got.ViewerIDList)

	requireRecent(t, got.CreatedAt.Time)
	requireRecent(t, got.UpdatedAt.Time)

	goldenJSON(t, "shopmap_with_viewer", got)
}

// Categories are stored with an explicit ordinal (shop_map_categories.number), which is what
// makes their order survive a round trip.
func TestShopMapCategoryOrderIsStoredExplicitly(t *testing.T) {
	t.Parallel()

	a := newApp(t)

	owner := a.newUser(t, "owner")
	created := a.newShopMap(t, owner.ID, "corner shop",
		[]product.Category{"zebra", "apple", "mango"}, nil)

	rows, err := a.sqlDB.Query(
		`SELECT number, category FROM shop_map_categories WHERE map_id = ? ORDER BY number`,
		created.ID.String())
	require.NoError(t, err)

	defer func() { require.NoError(t, rows.Close()) }()

	var (
		numbers    []int64
		categories []string
	)

	for rows.Next() {
		var (
			number   int64
			category string
		)

		require.NoError(t, rows.Scan(&number, &category))

		numbers = append(numbers, number)
		categories = append(categories, category)
	}

	require.NoError(t, rows.Err())
	require.Equal(t, []int64{0, 1, 2}, numbers)
	require.Equal(t, []string{"zebra", "apple", "mango"}, categories)
}

func TestShopMapGetByUserID(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	a := newApp(t)

	owner := a.newUser(t, "owner")
	viewer := a.newUser(t, "viewer")
	stranger := a.newUser(t, "stranger")

	a.newShopMap(t, owner.ID, "shared", []product.Category{"dairy"}, []id.ID[user.User]{viewer.ID})
	a.newShopMap(t, owner.ID, "private", []product.Category{"bakery"}, nil)

	ownerMaps, err := a.shopmaps.GetByUserID(ctx, owner.ID)
	require.NoError(t, err)
	require.Len(t, ownerMaps, 2)

	viewerMaps, err := a.shopmaps.GetByUserID(ctx, viewer.ID)
	require.NoError(t, err)
	require.Len(t, viewerMaps, 1)
	require.Equal(t, "shared", viewerMaps[0].Title)

	strangerMaps, err := a.shopmaps.GetByUserID(ctx, stranger.ID)
	require.NoError(t, err)
	require.Empty(t, strangerMaps)
}

func TestShopMapUpdate(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	a := newApp(t)

	owner := a.newUser(t, "owner")
	viewer := a.newUser(t, "viewer")
	created := a.newShopMap(t, owner.ID, "corner shop", []product.Category{"dairy"}, nil)

	updated, err := a.shopmaps.UpdateMap(ctx, created.ID, owner.ID, shopmap.Options{
		CategoryList: []product.Category{"bakery", "dairy"},
		ViewerIDList: []id.ID[user.User]{viewer.ID},
		Title:        "big shop",
	})
	require.NoError(t, err)
	require.Equal(t, "big shop", updated.Title)

	got, err := a.shopmaps.GetByID(ctx, created.ID)
	require.NoError(t, err)
	require.Equal(t, []product.Category{"bakery", "dairy"}, got.CategoryList)
	require.Equal(t, []id.ID[user.User]{viewer.ID}, got.ViewerIDList)

	// The rewrite replaces the child rows rather than appending to them.
	require.Equal(t, 2, a.count(t,
		`SELECT count(*) FROM shop_map_categories WHERE map_id = ?`, created.ID.String()))
	require.Equal(t, 1, a.count(t,
		`SELECT count(*) FROM shop_map_viewers WHERE map_id = ?`, created.ID.String()))
}

func TestShopMapUpdateRejectsNonMember(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	a := newApp(t)

	owner := a.newUser(t, "owner")
	stranger := a.newUser(t, "stranger")
	created := a.newShopMap(t, owner.ID, "corner shop", []product.Category{"dairy"}, nil)

	_, err := a.shopmaps.UpdateMap(ctx, created.ID, stranger.ID, shopmap.Options{
		CategoryList: []product.Category{"dairy"},
		ViewerIDList: nil,
		Title:        "hijacked",
	})
	require.ErrorIs(t, err, myerr.ErrForbidden)
}

func TestShopMapOwnerCannotBeAViewer(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	a := newApp(t)

	owner := a.newUser(t, "owner")

	_, err := a.shopmaps.Create(ctx, owner.ID, shopmap.Options{
		CategoryList: []product.Category{"dairy"},
		ViewerIDList: []id.ID[user.User]{owner.ID},
		Title:        "corner shop",
	})
	require.Error(t, err)
}

func TestShopMapCreateRequiresTitle(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	a := newApp(t)

	owner := a.newUser(t, "owner")

	_, err := a.shopmaps.Create(ctx, owner.ID, shopmap.Options{
		CategoryList: []product.Category{"dairy"},
		ViewerIDList: nil,
		Title:        "",
	})
	require.Error(t, err)
}

func TestShopMapReorder(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	a := newApp(t)

	owner := a.newUser(t, "owner")
	created := a.newShopMap(t, owner.ID, "corner shop",
		[]product.Category{"dairy", "bakery", "produce"}, nil)

	reordered, err := a.shopmaps.ReorderMap(ctx, created.ID, owner.ID,
		[]product.Category{"produce", "dairy", "bakery"})
	require.NoError(t, err)
	require.Equal(t, []product.Category{"produce", "dairy", "bakery"}, reordered.CategoryList)

	got, err := a.shopmaps.GetByID(ctx, created.ID)
	require.NoError(t, err)
	require.Equal(t, []product.Category{"produce", "dairy", "bakery"}, got.CategoryList)
}

// ReorderMap accepts permutations only — it compares value counts, so adding or dropping a
// category through this path is rejected.
func TestShopMapReorderRejectsNonPermutation(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	a := newApp(t)

	owner := a.newUser(t, "owner")
	created := a.newShopMap(t, owner.ID, "corner shop", []product.Category{"dairy", "bakery"}, nil)

	_, err := a.shopmaps.ReorderMap(ctx, created.ID, owner.ID, []product.Category{"dairy"})
	require.ErrorIs(t, err, myerr.ErrInvalidArgument)

	_, err = a.shopmaps.ReorderMap(ctx, created.ID, owner.ID,
		[]product.Category{"dairy", "bakery", "produce"})
	require.ErrorIs(t, err, myerr.ErrInvalidArgument)
}

func TestShopMapReorderRequiresOwner(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	a := newApp(t)

	owner := a.newUser(t, "owner")
	viewer := a.newUser(t, "viewer")
	created := a.newShopMap(t, owner.ID, "corner shop",
		[]product.Category{"dairy", "bakery"}, []id.ID[user.User]{viewer.ID})

	_, err := a.shopmaps.ReorderMap(ctx, created.ID, viewer.ID, []product.Category{"bakery", "dairy"})
	require.ErrorIs(t, err, myerr.ErrForbidden)
}

func TestShopMapDelete(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	a := newApp(t)

	owner := a.newUser(t, "owner")
	viewer := a.newUser(t, "viewer")
	created := a.newShopMap(t, owner.ID, "corner shop",
		[]product.Category{"dairy"}, []id.ID[user.User]{viewer.ID})

	_, err := a.shopmaps.DeleteMap(ctx, created.ID, owner.ID)
	require.NoError(t, err)

	require.Equal(t, 0, a.count(t, `SELECT count(*) FROM shop_maps WHERE id = ?`, created.ID.String()))
	require.Equal(t, 0, a.count(t,
		`SELECT count(*) FROM shop_map_categories WHERE map_id = ?`, created.ID.String()))
	require.Equal(t, 0, a.count(t,
		`SELECT count(*) FROM shop_map_viewers WHERE map_id = ?`, created.ID.String()))
}

func TestShopMapDeleteRequiresOwner(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	a := newApp(t)

	owner := a.newUser(t, "owner")
	viewer := a.newUser(t, "viewer")
	created := a.newShopMap(t, owner.ID, "corner shop",
		[]product.Category{"dairy"}, []id.ID[user.User]{viewer.ID})

	_, err := a.shopmaps.DeleteMap(ctx, created.ID, viewer.ID)
	require.ErrorIs(t, err, myerr.ErrForbidden)

	require.Equal(t, 1, a.count(t, `SELECT count(*) FROM shop_maps WHERE id = ?`, created.ID.String()))
}

func TestShopMapGetMissing(t *testing.T) {
	t.Parallel()

	a := newApp(t)

	_, err := a.shopmaps.GetByID(context.Background(), id.NewID[shopmap.ShopMap]())
	require.Error(t, err)
}
