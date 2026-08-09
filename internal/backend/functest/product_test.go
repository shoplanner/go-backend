package functest_test

import (
	"context"
	"testing"

	"github.com/samber/mo"
	"github.com/stretchr/testify/require"

	"go-backend/internal/backend/product"
	"go-backend/pkg/id"
)

func TestProductCreateWithoutCategory(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	a := newApp(t)

	created := a.newProduct(t, "milk", "")
	require.Equal(t, product.Name("milk"), created.Name)
	require.True(t, created.Category.IsAbsent())
	require.Empty(t, created.Forms)
	requireRecent(t, created.CreatedAt.Time)

	got, err := a.products.ID(ctx, created.ID)
	require.NoError(t, err)
	require.Equal(t, created.ID, got.ID)
	require.Equal(t, product.Name("milk"), got.Name)
	require.True(t, got.Category.IsAbsent())

	require.Equal(t, 0, a.count(t, `SELECT count(*) FROM product_categories`))
	require.Equal(t, 1, a.count(t, `SELECT count(*) FROM products WHERE category_id IS NULL`))
}

func TestProductCreateWithCategoryAndForms(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	a := newApp(t)

	created := a.newProduct(t, "milk", "dairy", "bottle", "carton")

	got, err := a.products.ID(ctx, created.ID)
	require.NoError(t, err)
	require.Equal(t, mo.Some(product.Category("dairy")), got.Category)
	require.Equal(t, []product.Form{"bottle", "carton"}, got.Forms)

	goldenJSON(t, "product_with_category_and_forms", got)
}

// products.category_id must hold the category's UUID, not its name.
//
// Product.BeforeSave looks like it rewrites category_id to the category *name*, but it is dead
// code: ModelToEntity always hands it CategoryID{Valid:false}, so the hook returns early and
// GORM's belongs-to upsert writes the real foreign key. A rewrite that faithfully ports the
// hook instead of the observed behaviour would corrupt the column.
func TestProductCategoryIDHoldsUUIDNotName(t *testing.T) {
	t.Parallel()

	a := newApp(t)

	created := a.newProduct(t, "milk", "dairy")

	var categoryID, categoryName string
	require.NoError(t, a.sqlDB.QueryRow(`
		SELECT p.category_id, c.name
		FROM products AS p
		JOIN product_categories AS c ON c.id = p.category_id
		WHERE p.id = ?`, created.ID.String()).Scan(&categoryID, &categoryName))

	require.Equal(t, "dairy", categoryName)
	require.NotEqual(t, "dairy", categoryID)
	require.NoError(t, uuidLike(categoryID))
}

// ProductCategory.BeforeSave upserts by name, so a category is stored once no matter how many
// products reference it.
func TestProductCategoriesAreDeduplicatedByName(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	a := newApp(t)

	first := a.newProduct(t, "milk", "dairy")
	second := a.newProduct(t, "cheese", "dairy")
	a.newProduct(t, "bread", "bakery")

	require.Equal(t, 2, a.count(t, `SELECT count(*) FROM product_categories`))
	require.Equal(t, 1, a.count(t, `SELECT count(*) FROM product_categories WHERE name = 'dairy'`))

	firstGot, err := a.products.ID(ctx, first.ID)
	require.NoError(t, err)
	secondGot, err := a.products.ID(ctx, second.ID)
	require.NoError(t, err)

	require.Equal(t, firstGot.Category, secondGot.Category)
}

func TestProductUpdateName(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	a := newApp(t)

	created := a.newProduct(t, "milk", "dairy", "bottle")

	updated, err := a.products.Update(ctx, created.ID, product.Options{
		Name:     "oat milk",
		Category: mo.Some(product.Category("dairy")),
		Forms:    []product.Form{"bottle"},
	})
	require.NoError(t, err)
	require.Equal(t, product.Name("oat milk"), updated.Name)

	got, err := a.products.ID(ctx, created.ID)
	require.NoError(t, err)
	require.Equal(t, product.Name("oat milk"), got.Name)
	require.Equal(t, mo.Some(product.Category("dairy")), got.Category)
	require.Equal(t, []product.Form{"bottle"}, got.Forms)
}

// Forms are replaced wholesale through Association("Forms").Unscoped().Replace. The rows that
// drop out must not linger in product_forms — a rewrite that issues an INSERT-only update
// would leave the old ones behind and GetByID would start returning both sets.
func TestProductUpdateReplacesFormsWithoutOrphans(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	a := newApp(t)

	created := a.newProduct(t, "milk", "", "bottle", "carton", "can")
	require.Equal(t, 3, a.count(t, `SELECT count(*) FROM product_forms`))

	_, err := a.products.Update(ctx, created.ID, product.Options{
		Name:     "milk",
		Category: mo.None[product.Category](),
		Forms:    []product.Form{"pouch"},
	})
	require.NoError(t, err)

	got, err := a.products.ID(ctx, created.ID)
	require.NoError(t, err)
	require.Equal(t, []product.Form{"pouch"}, got.Forms)

	require.Equal(t, 1, a.count(t, `SELECT count(*) FROM product_forms`))
	require.Equal(t, 0, a.count(t, `SELECT count(*) FROM product_forms WHERE product_id IS NULL`))
}

func TestProductUpdateDropsCategory(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	a := newApp(t)

	created := a.newProduct(t, "milk", "dairy")

	_, err := a.products.Update(ctx, created.ID, product.Options{
		Name:     "milk",
		Category: mo.None[product.Category](),
		Forms:    []product.Form{},
	})
	require.NoError(t, err)

	got, err := a.products.ID(ctx, created.ID)
	require.NoError(t, err)
	require.True(t, got.Category.IsAbsent())
}

func TestProductIDList(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	a := newApp(t)

	milk := a.newProduct(t, "milk", "dairy", "bottle")
	bread := a.newProduct(t, "bread", "bakery")
	a.newProduct(t, "cheese", "dairy")

	got, err := a.products.IDList(ctx, []id.ID[product.Product]{milk.ID, bread.ID})
	require.NoError(t, err)
	require.Len(t, got, 2)

	names := make([]string, 0, len(got))
	for _, p := range got {
		names = append(names, string(p.Name))
	}

	require.ElementsMatch(t, []string{"milk", "bread"}, names)
}

// IDList does not Preload("Category"), so products come back without one even when the row has
// a category_id. Only the list/favorite read paths preload it.
func TestProductIDListOmitsCategory(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	a := newApp(t)

	milk := a.newProduct(t, "milk", "dairy", "bottle")

	got, err := a.products.IDList(ctx, []id.ID[product.Product]{milk.ID})
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.True(t, got[0].Category.IsAbsent(), "IDList intentionally skips Preload(\"Category\")")
	require.Equal(t, []product.Form{"bottle"}, got[0].Forms, "Forms are preloaded, Category is not")
}

// KNOWN BUG (current behaviour pinned). GetByListID passes the id slice straight to
// Find(&entities, uuids); an empty slice means "no condition", so an empty request returns the
// entire products table instead of nothing.
func TestProductIDListWithNoIDs_CurrentBehaviour(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	a := newApp(t)

	a.newProduct(t, "milk", "")
	a.newProduct(t, "bread", "")

	got, err := a.products.IDList(ctx, []id.ID[product.Product]{})
	require.NoError(t, err)
	require.Len(t, got, 2, "empty filter currently degrades into a full table scan")
}

func TestProductIDListWithNoIDs_Desired(t *testing.T) {
	t.Parallel()

	t.Skip("KNOWN BUG: empty id slice returns every product; see TestProductIDListWithNoIDs_CurrentBehaviour")

	ctx := context.Background()
	a := newApp(t)

	a.newProduct(t, "milk", "")

	got, err := a.products.IDList(ctx, []id.ID[product.Product]{})
	require.NoError(t, err)
	require.Empty(t, got)
}

func TestProductGetMissing(t *testing.T) {
	t.Parallel()

	a := newApp(t)

	_, err := a.products.ID(context.Background(), id.NewID[product.Product]())
	require.Error(t, err)
}

func TestProductUpdateMissing(t *testing.T) {
	t.Parallel()

	a := newApp(t)

	_, err := a.products.Update(context.Background(), id.NewID[product.Product](), product.Options{
		Name:     "ghost",
		Category: mo.None[product.Category](),
		Forms:    []product.Form{},
	})
	require.Error(t, err)
}
