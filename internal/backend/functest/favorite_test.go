package functest_test

import (
	"context"
	"testing"

	"github.com/samber/mo"
	"github.com/stretchr/testify/require"

	"go-backend/internal/backend/favorite"
	"go-backend/internal/backend/product"
	"go-backend/internal/backend/user"
	"go-backend/pkg/id"
	"go-backend/pkg/myerr"
)

// personalList returns the favorites list created as a side effect of registering owner.
func (a *app) personalList(t *testing.T, owner user.User) favorite.List {
	t.Helper()

	lists, err := a.favorites.GetListsByUserID(context.Background(), owner.ID)
	require.NoError(t, err)
	require.Len(t, lists, 1)

	return lists[0]
}

func favoriteProductIDs(model favorite.List) []id.ID[product.Product] {
	res := make([]id.ID[product.Product], 0, len(model.Products))
	for _, item := range model.Products {
		res = append(res, item.Product.ID)
	}

	return res
}

func TestFavoriteAddProducts(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	a := newApp(t)

	owner := a.newUser(t, "owner")
	personal := a.personalList(t, owner)

	milk := a.newProduct(t, "milk", "dairy", "bottle")
	bread := a.newProduct(t, "bread", "bakery")

	updated, err := a.favorites.AddProducts(ctx, personal.ID, owner.ID, []id.ID[product.Product]{
		milk.ID, bread.ID,
	})
	require.NoError(t, err)
	require.ElementsMatch(t, []id.ID[product.Product]{milk.ID, bread.ID}, favoriteProductIDs(updated))

	require.Equal(t, 2, a.count(t,
		`SELECT count(*) FROM favorite_products WHERE favorite_list_id = ?`, personal.ID.String()))
}

// The favorites read path preloads the product together with its category and forms, so a
// client can render the list without a second round trip.
func TestFavoriteReadPreloadsProductDetails(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	a := newApp(t)

	owner := a.newUser(t, "owner")
	personal := a.personalList(t, owner)
	milk := a.newProduct(t, "milk", "dairy", "bottle", "carton")

	_, err := a.favorites.AddProducts(ctx, personal.ID, owner.ID, []id.ID[product.Product]{milk.ID})
	require.NoError(t, err)

	lists, err := a.favorites.GetListsByUserID(ctx, owner.ID)
	require.NoError(t, err)
	require.Len(t, lists, 1)
	require.Len(t, lists[0].Products, 1)

	got := lists[0].Products[0].Product
	require.Equal(t, milk.ID, got.ID)
	require.Equal(t, product.Name("milk"), got.Name)
	require.Equal(t, mo.Some(product.Category("dairy")), got.Category)
	require.Equal(t, []product.Form{"bottle", "carton"}, got.Forms)

	goldenJSON(t, "favorite_list_with_product", lists[0])
}

// Regression guard for the most dangerous shape in the favorites repo: productToEntity builds
// a fully zeroed repo.Product (empty name, empty category, empty forms) and hands it to
// Create(entity.Products) alongside the association. If that ever starts upserting the parent
// row, adding a product to favorites would silently blank out the product itself.
func TestFavoriteAddDoesNotBlankTheProduct(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	a := newApp(t)

	owner := a.newUser(t, "owner")
	personal := a.personalList(t, owner)
	milk := a.newProduct(t, "milk", "dairy", "bottle")

	_, err := a.favorites.AddProducts(ctx, personal.ID, owner.ID, []id.ID[product.Product]{milk.ID})
	require.NoError(t, err)

	got, err := a.products.ID(ctx, milk.ID)
	require.NoError(t, err)
	require.Equal(t, product.Name("milk"), got.Name)
	require.Equal(t, mo.Some(product.Category("dairy")), got.Category)
	require.Equal(t, []product.Form{"bottle"}, got.Forms)

	require.Equal(t, 1, a.count(t, `SELECT count(*) FROM products WHERE name = 'milk'`))
	require.Equal(t, 0, a.count(t, `SELECT count(*) FROM products WHERE name = ''`))
	require.Equal(t, 1, a.count(t, `SELECT count(*) FROM product_forms WHERE product_id = ?`, milk.ID.String()))
}

func TestFavoriteDeleteProducts(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	a := newApp(t)

	owner := a.newUser(t, "owner")
	personal := a.personalList(t, owner)

	milk := a.newProduct(t, "milk", "dairy")
	bread := a.newProduct(t, "bread", "bakery")

	_, err := a.favorites.AddProducts(ctx, personal.ID, owner.ID, []id.ID[product.Product]{milk.ID, bread.ID})
	require.NoError(t, err)

	updated, err := a.favorites.DeleteProducts(ctx, personal.ID, owner.ID, []id.ID[product.Product]{milk.ID})
	require.NoError(t, err)
	require.Equal(t, []id.ID[product.Product]{bread.ID}, favoriteProductIDs(updated))

	require.Equal(t, 1, a.count(t,
		`SELECT count(*) FROM favorite_products WHERE favorite_list_id = ?`, personal.ID.String()))

	// The product itself survives being unfavourited.
	require.Equal(t, 2, a.count(t, `SELECT count(*) FROM products`))
}

func TestFavoriteDeleteUnknownProduct(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	a := newApp(t)

	owner := a.newUser(t, "owner")
	personal := a.personalList(t, owner)

	_, err := a.favorites.DeleteProducts(ctx, personal.ID, owner.ID, []id.ID[product.Product]{
		id.NewID[product.Product](),
	})
	require.ErrorIs(t, err, myerr.ErrNotFound)
}

// favorite_products carries a unique index on (product_id, favorite_list_id) and the writer
// uses ON CONFLICT DO UPDATE, so adding the same product twice must not duplicate the row.
func TestFavoriteAddSameProductTwice(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	a := newApp(t)

	owner := a.newUser(t, "owner")
	personal := a.personalList(t, owner)
	milk := a.newProduct(t, "milk", "dairy")

	_, err := a.favorites.AddProducts(ctx, personal.ID, owner.ID, []id.ID[product.Product]{milk.ID})
	require.NoError(t, err)

	_, err = a.favorites.AddProducts(ctx, personal.ID, owner.ID, []id.ID[product.Product]{milk.ID})
	require.NoError(t, err)

	require.Equal(t, 1, a.count(t,
		`SELECT count(*) FROM favorite_products WHERE favorite_list_id = ? AND product_id = ?`,
		personal.ID.String(), milk.ID.String()))
}

func TestFavoriteEditRequiresMembership(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	a := newApp(t)

	owner := a.newUser(t, "owner")
	stranger := a.newUser(t, "stranger")
	personal := a.personalList(t, owner)
	milk := a.newProduct(t, "milk", "dairy")

	_, err := a.favorites.AddProducts(ctx, personal.ID, stranger.ID, []id.ID[product.Product]{milk.ID})
	require.ErrorIs(t, err, myerr.ErrForbidden)

	require.Equal(t, 0, a.count(t, `SELECT count(*) FROM favorite_products`))
}

// KNOWN BUG (current behaviour pinned). favorite.List.AllowedToView is inverted: it returns an
// error when the caller *is* a member and nil when they are not. GetListByID is therefore
// exactly backwards — the owner is locked out of their own list and any stranger can read it.
func TestFavoriteGetListByIDIsInverted_CurrentBehaviour(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	a := newApp(t)

	owner := a.newUser(t, "owner")
	stranger := a.newUser(t, "stranger")
	personal := a.personalList(t, owner)

	_, err := a.favorites.GetListByID(ctx, personal.ID, owner.ID)
	require.ErrorIs(t, err, myerr.ErrForbidden, "the owner is refused their own list")

	got, err := a.favorites.GetListByID(ctx, personal.ID, stranger.ID)
	require.NoError(t, err, "a non-member is let through")
	require.Equal(t, personal.ID, got.ID)
}

func TestFavoriteGetListByID_Desired(t *testing.T) {
	t.Parallel()

	t.Skip("KNOWN BUG: favorite.List.AllowedToView has its condition inverted; " +
		"see TestFavoriteGetListByIDIsInverted_CurrentBehaviour")

	ctx := context.Background()
	a := newApp(t)

	owner := a.newUser(t, "owner")
	stranger := a.newUser(t, "stranger")
	personal := a.personalList(t, owner)

	got, err := a.favorites.GetListByID(ctx, personal.ID, owner.ID)
	require.NoError(t, err)
	require.Equal(t, personal.ID, got.ID)

	_, err = a.favorites.GetListByID(ctx, personal.ID, stranger.ID)
	require.ErrorIs(t, err, myerr.ErrForbidden)
}

func TestFavoriteGetListsByUserIDIsScoped(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	a := newApp(t)

	owner := a.newUser(t, "owner")
	other := a.newUser(t, "other")

	ownerList := a.personalList(t, owner)
	otherList := a.personalList(t, other)
	require.NotEqual(t, ownerList.ID, otherList.ID)

	milk := a.newProduct(t, "milk", "dairy")
	_, err := a.favorites.AddProducts(ctx, ownerList.ID, owner.ID, []id.ID[product.Product]{milk.ID})
	require.NoError(t, err)

	otherLists, err := a.favorites.GetListsByUserID(ctx, other.ID)
	require.NoError(t, err)
	require.Len(t, otherLists, 1)
	require.Empty(t, otherLists[0].Products, "another user's favourites must not leak in")
}

func TestFavoriteAddToMissingList(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	a := newApp(t)

	owner := a.newUser(t, "owner")
	milk := a.newProduct(t, "milk", "dairy")

	_, err := a.favorites.AddProducts(ctx, id.NewID[favorite.List](), owner.ID, []id.ID[product.Product]{
		milk.ID,
	})
	require.Error(t, err)
}
