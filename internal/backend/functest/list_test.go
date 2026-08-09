package functest_test

import (
	"context"
	"testing"

	"github.com/samber/lo"
	"github.com/samber/mo"
	"github.com/stretchr/testify/require"

	"go-backend/internal/backend/list"
	"go-backend/internal/backend/product"
	"go-backend/internal/backend/user"
	"go-backend/pkg/id"
	"go-backend/pkg/myerr"
)

// productIDs is the order in which states came back, by product.
func productIDs(model list.ProductList) []id.ID[product.Product] {
	return lo.Map(model.States, func(s list.ProductState, _ int) id.ID[product.Product] {
		return s.Product.ID
	})
}

func memberIDs(model list.ProductList) []id.ID[user.User] {
	return lo.Map(model.Members, func(m list.Member, _ int) id.ID[user.User] { return m.UserID })
}

func productIDsOf(products ...product.Product) []id.ID[product.Product] {
	return lo.Map(products, func(p product.Product, _ int) id.ID[product.Product] { return p.ID })
}

func memberIDsOf(ids ...id.ID[user.User]) []id.ID[user.User] { return ids }

func TestListCreate(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	a := newApp(t)

	owner := a.newUser(t, "owner")
	created := a.newList(t, owner.ID, "groceries")

	require.Equal(t, "groceries", created.Title)
	require.Equal(t, list.ExecStatusPlanning, created.Status)
	require.Empty(t, created.States)
	require.Len(t, created.Members, 1)
	require.Equal(t, list.MemberTypeOwner, created.Members[0].Role)

	got, err := a.lists.GetByID(ctx, created.ID, owner.ID)
	require.NoError(t, err)
	require.Equal(t, created.ID, got.ID)
	require.Equal(t, "groceries", got.Title)
	require.Len(t, got.Members, 1)
	// Create stores an empty UserName; the read path fills it in from the users table.
	require.Equal(t, user.Login("owner"), got.Members[0].UserName)
}

func TestListUpdateOptions(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	a := newApp(t)

	owner := a.newUser(t, "owner")
	created := a.newList(t, owner.ID, "groceries")

	updated, err := a.lists.Update(ctx, created.ID, owner.ID, list.ListOptions{
		Status: list.ExecStatusProcessing,
		Title:  "weekend groceries",
	})
	require.NoError(t, err)
	require.Equal(t, "weekend groceries", updated.Title)
	require.Equal(t, list.ExecStatusProcessing, updated.Status)

	got, err := a.lists.GetByID(ctx, created.ID, owner.ID)
	require.NoError(t, err)
	require.Equal(t, "weekend groceries", got.Title)
	require.Equal(t, list.ExecStatusProcessing, got.Status)
}

// BEHAVIOUR CHANGE, deliberate, introduced by the sqlc migration.
//
// The GORM write path ended in Updates(&entity), which skips zero-valued struct fields, so
// clearing a title was silently a no-op: the old title stayed on disk and came straight back.
// list.ListOptions.Title carries no `validate:"required"`, so that was reachable from the API.
// The replacement is a plain UPDATE, which writes every column it is given.
//
// Nothing pinned the old behaviour, and reproducing it would have meant teaching the repo that
// an empty string means "leave alone" — an ORM accident, not a rule anybody chose.
func TestListUpdateCanClearTheTitle(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	a := newApp(t)

	owner := a.newUser(t, "owner")
	created := a.newList(t, owner.ID, "groceries")

	updated, err := a.lists.Update(ctx, created.ID, owner.ID, list.ListOptions{
		Status: list.ExecStatusProcessing,
		Title:  "",
	})
	require.NoError(t, err)
	require.Empty(t, updated.Title)

	got, err := a.lists.GetByID(ctx, created.ID, owner.ID)
	require.NoError(t, err)
	require.Empty(t, got.Title, "an empty title is now persisted instead of being skipped")
}

func TestListGetByUserID(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	a := newApp(t)

	owner := a.newUser(t, "owner")
	other := a.newUser(t, "other")

	a.newList(t, owner.ID, "groceries")
	a.newList(t, owner.ID, "hardware")
	a.newList(t, other.ID, "not mine")

	got, err := a.lists.GetByUserID(ctx, owner.ID)
	require.NoError(t, err)
	require.Len(t, got, 2)
	require.ElementsMatch(t,
		[]string{"groceries", "hardware"},
		lo.Map(got, func(l list.ProductList, _ int) string { return l.Title }),
	)

	stranger := a.newUser(t, "stranger")
	empty, err := a.lists.GetByUserID(ctx, stranger.ID)
	require.NoError(t, err)
	require.Empty(t, empty)
}

func TestListAppendProductsKeepsInsertionOrder(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	a := newApp(t)

	owner := a.newUser(t, "owner")
	created := a.newList(t, owner.ID, "groceries")

	milk := a.newProduct(t, "milk", "dairy", "bottle")
	bread := a.newProduct(t, "bread", "bakery")

	// AppendProducts takes a map, so a single call has no defined order. Append one at a time
	// to pin the index assignment deterministically.
	a.addProducts(t, created.ID, owner.ID, milk)
	a.addProducts(t, created.ID, owner.ID, bread)

	got, err := a.lists.GetByID(ctx, created.ID, owner.ID)
	require.NoError(t, err)
	require.Equal(t, []id.ID[product.Product]{milk.ID, bread.ID}, productIDs(got))

	// The read path rebuilds order from the stored index column, which must stay dense.
	require.Equal(t, []int64{0, 1}, a.stateIndexes(t, created.ID))

	// States carry the fully preloaded product, not just its id.
	require.Equal(t, product.Name("milk"), got.States[0].Product.Name)
	require.Equal(t, mo.Some(product.Category("dairy")), got.States[0].Product.Category)
	require.Equal(t, []product.Form{"bottle"}, got.States[0].Product.Forms)

	goldenJSON(t, "list_with_two_products", got)
}

func TestListUpdateProductState(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	a := newApp(t)

	owner := a.newUser(t, "owner")
	created := a.newList(t, owner.ID, "groceries")
	milk := a.newProduct(t, "milk", "dairy", "bottle")
	a.addProducts(t, created.ID, owner.ID, milk)

	state, err := a.lists.UpdateProductState(ctx, created.ID, owner.ID, milk.ID, list.ProductStateOptions{
		Count:       mo.Some[int32](3),
		FormIndex:   mo.Some[int32](0),
		Status:      list.StateStatusTaken,
		Replacement: mo.None[list.ProductStateReplacement](),
	})
	require.NoError(t, err)
	require.Equal(t, mo.Some[int32](3), state.Count)
	require.Equal(t, list.StateStatusTaken, state.Status)

	got, err := a.lists.GetByID(ctx, created.ID, owner.ID)
	require.NoError(t, err)
	require.Len(t, got.States, 1)
	require.Equal(t, mo.Some[int32](3), got.States[0].Count)
	require.Equal(t, mo.Some[int32](0), got.States[0].FormIndex)
	require.Equal(t, list.StateStatusTaken, got.States[0].Status)
	require.True(t, got.States[0].Replacement.IsAbsent())

	require.Equal(t, 1, a.count(t, `
		SELECT count(*) FROM product_list_states
		WHERE count = 3 AND form_idx = 0 AND replacement_product_id IS NULL`))
}

func TestListProductStateReplacementRoundTrip(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	a := newApp(t)

	owner := a.newUser(t, "owner")
	created := a.newList(t, owner.ID, "groceries")
	milk := a.newProduct(t, "milk", "dairy")
	oat := a.newProduct(t, "oat milk", "dairy")
	a.addProducts(t, created.ID, owner.ID, milk)

	_, err := a.lists.UpdateProductState(ctx, created.ID, owner.ID, milk.ID, list.ProductStateOptions{
		Count:     mo.None[int32](),
		FormIndex: mo.None[int32](),
		Status:    list.StateStatusReplaced,
		Replacement: mo.Some(list.ProductStateReplacement{
			Count:     mo.Some[int32](2),
			FormIndex: mo.None[int32](),
			Product:   oat,
		}),
	})
	require.NoError(t, err)

	got, err := a.lists.GetByID(ctx, created.ID, owner.ID)
	require.NoError(t, err)
	require.Len(t, got.States, 1)

	replacement, present := got.States[0].Replacement.Get()
	require.True(t, present)
	require.Equal(t, oat.ID, replacement.Product.ID)
	require.Equal(t, product.Name("oat milk"), replacement.Product.Name)
	require.Equal(t, mo.Some[int32](2), replacement.Count)
	require.True(t, replacement.FormIndex.IsAbsent())

	require.Equal(t, 1, a.count(t, `
		SELECT count(*) FROM product_list_states
		WHERE replacement_product_id = ? AND replacement_count = 2 AND replacement_form_idx IS NULL`,
		oat.ID.String()))

	goldenJSON(t, "list_state_with_replacement", got.States[0])

	// Dropping the replacement must clear every replacement_* column, not just the id.
	_, err = a.lists.UpdateProductState(ctx, created.ID, owner.ID, milk.ID, zeroStateOptions())
	require.NoError(t, err)

	cleared, err := a.lists.GetByID(ctx, created.ID, owner.ID)
	require.NoError(t, err)
	require.True(t, cleared.States[0].Replacement.IsAbsent())
	require.Equal(t, 1, a.count(t, `
		SELECT count(*) FROM product_list_states
		WHERE replacement_product_id IS NULL
		  AND replacement_count IS NULL
		  AND replacement_form_idx IS NULL`))
}

func TestListUpdateProductStateMissingProduct(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	a := newApp(t)

	owner := a.newUser(t, "owner")
	created := a.newList(t, owner.ID, "groceries")

	_, err := a.lists.UpdateProductState(
		ctx, created.ID, owner.ID, id.NewID[product.Product](), zeroStateOptions(),
	)
	require.ErrorIs(t, err, myerr.ErrNotFound)
}

func TestListDeleteProducts(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	a := newApp(t)

	owner := a.newUser(t, "owner")
	created := a.newList(t, owner.ID, "groceries")

	milk := a.newProduct(t, "milk", "")
	bread := a.newProduct(t, "bread", "")
	cheese := a.newProduct(t, "cheese", "")

	a.addProducts(t, created.ID, owner.ID, milk)
	a.addProducts(t, created.ID, owner.ID, bread)
	a.addProducts(t, created.ID, owner.ID, cheese)

	_, err := a.lists.DeleteProducts(ctx, created.ID, owner.ID, []id.ID[product.Product]{bread.ID})
	require.NoError(t, err)

	got, err := a.lists.GetByID(ctx, created.ID, owner.ID)
	require.NoError(t, err)
	require.ElementsMatch(t, []id.ID[product.Product]{milk.ID, cheese.ID}, productIDs(got))

	// Whatever the order ends up being, the index column has to stay dense or the read path
	// (states[state.Index] = ...) starts writing outside the slice.
	require.Equal(t, []int64{0, 1}, a.stateIndexes(t, created.ID))
	require.Equal(t, 2, a.count(t, `SELECT count(*) FROM product_list_states WHERE list_id = ?`, created.ID.String()))
}

func TestListDeleteMissingProduct(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	a := newApp(t)

	owner := a.newUser(t, "owner")
	created := a.newList(t, owner.ID, "groceries")

	_, err := a.lists.DeleteProducts(ctx, created.ID, owner.ID, []id.ID[product.Product]{
		id.NewID[product.Product](),
	})
	require.ErrorIs(t, err, myerr.ErrNotFound)
}

// KNOWN BUG (current behaviour pinned). DeleteProducts and DeleteMembers rebuild their
// collections from a map (slices.Collect(maps.Values(...))), so the surviving elements come
// back in Go's randomised map order. Callers that render a shopping list see it reshuffle
// after every deletion.
func TestListDeleteProductsScramblesOrder_CurrentBehaviour(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	a := newApp(t)

	owner := a.newUser(t, "owner")
	created := a.newList(t, owner.ID, "groceries")

	const total = 8

	products := make([]product.Product, 0, total)
	for i := range total {
		p := a.newProduct(t, "p"+string(rune('a'+i)), "")
		products = append(products, p)
		a.addProducts(t, created.ID, owner.ID, p)
	}

	_, err := a.lists.DeleteProducts(ctx, created.ID, owner.ID, []id.ID[product.Product]{products[0].ID})
	require.NoError(t, err)

	got, err := a.lists.GetByID(ctx, created.ID, owner.ID)
	require.NoError(t, err)

	// The set is right; only the order is not guaranteed. Asserting set-equality is the
	// strongest thing that can honestly be claimed about the current implementation.
	require.ElementsMatch(t,
		lo.Map(products[1:], func(p product.Product, _ int) id.ID[product.Product] { return p.ID }),
		productIDs(got),
	)
}

func TestListDeleteProductsPreservesOrder_Desired(t *testing.T) {
	t.Parallel()

	t.Skip("KNOWN BUG: deletion reshuffles states via maps.Values; " +
		"see TestListDeleteProductsScramblesOrder_CurrentBehaviour")

	ctx := context.Background()
	a := newApp(t)

	owner := a.newUser(t, "owner")
	created := a.newList(t, owner.ID, "groceries")

	milk := a.newProduct(t, "milk", "")
	bread := a.newProduct(t, "bread", "")
	cheese := a.newProduct(t, "cheese", "")

	a.addProducts(t, created.ID, owner.ID, milk)
	a.addProducts(t, created.ID, owner.ID, bread)
	a.addProducts(t, created.ID, owner.ID, cheese)

	_, err := a.lists.DeleteProducts(ctx, created.ID, owner.ID, []id.ID[product.Product]{bread.ID})
	require.NoError(t, err)

	got, err := a.lists.GetByID(ctx, created.ID, owner.ID)
	require.NoError(t, err)
	require.Equal(t, []id.ID[product.Product]{milk.ID, cheese.ID}, productIDs(got))
}

func TestListReorderStates(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	a := newApp(t)

	owner := a.newUser(t, "owner")
	created := a.newList(t, owner.ID, "groceries")

	milk := a.newProduct(t, "milk", "")
	bread := a.newProduct(t, "bread", "")
	cheese := a.newProduct(t, "cheese", "")

	a.addProducts(t, created.ID, owner.ID, milk)
	a.addProducts(t, created.ID, owner.ID, bread)
	a.addProducts(t, created.ID, owner.ID, cheese)

	require.NoError(t, a.lists.ReoderStates(ginCtx(), owner.ID, created.ID, []id.ID[product.Product]{
		cheese.ID, milk.ID, bread.ID,
	}))

	got, err := a.lists.GetByID(ctx, created.ID, owner.ID)
	require.NoError(t, err)
	require.Equal(t, []id.ID[product.Product]{cheese.ID, milk.ID, bread.ID}, productIDs(got))
	require.Equal(t, []int64{0, 1, 2}, a.stateIndexes(t, created.ID))
}

func TestListReorderStatesRejectsEmptyOrder(t *testing.T) {
	t.Parallel()

	a := newApp(t)

	owner := a.newUser(t, "owner")
	created := a.newList(t, owner.ID, "groceries")

	err := a.lists.ReoderStates(ginCtx(), owner.ID, created.ID, []id.ID[product.Product]{})
	require.ErrorIs(t, err, myerr.ErrInvalidArgument)
}

// KNOWN BUG (current behaviour pinned). ApplyOrder writes `index` only for the ids it is given
// and leaves the rest untouched, so a partial reorder produces duplicate index values. The
// read path then does states[state.Index] = ... into a slice sized len(states), which silently
// drops entries (and would panic outright if any index exceeded the slice length).
func TestListReorderStatesPartial_CurrentBehaviour(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	a := newApp(t)

	owner := a.newUser(t, "owner")
	created := a.newList(t, owner.ID, "groceries")

	milk := a.newProduct(t, "milk", "")
	bread := a.newProduct(t, "bread", "")
	cheese := a.newProduct(t, "cheese", "")

	a.addProducts(t, created.ID, owner.ID, milk)
	a.addProducts(t, created.ID, owner.ID, bread)
	a.addProducts(t, created.ID, owner.ID, cheese)

	// Only two of the three products are mentioned.
	require.NoError(t, a.lists.ReoderStates(ginCtx(), owner.ID, created.ID, []id.ID[product.Product]{
		cheese.ID, milk.ID,
	}))

	require.Equal(t, []int64{0, 1, 1}, a.stateIndexes(t, created.ID),
		"cheese->0, milk->1, and the untouched bread keeps its old 1: no longer a permutation")

	got, err := a.lists.GetByID(ctx, created.ID, owner.ID)
	require.NoError(t, err)

	// Three rows in, three slots out, but one product is duplicated away: the collision on
	// index 1 means whichever row is read last wins and one slot stays zero-valued.
	require.Len(t, got.States, 3)
	require.Contains(t, productIDs(got), id.ID[product.Product]{}, "a state slot was left zero-valued")
}

func TestListReorderStatesPartial_Desired(t *testing.T) {
	t.Parallel()

	t.Skip("KNOWN BUG: a partial reorder corrupts the index column; " +
		"see TestListReorderStatesPartial_CurrentBehaviour")

	a := newApp(t)

	owner := a.newUser(t, "owner")
	created := a.newList(t, owner.ID, "groceries")

	milk := a.newProduct(t, "milk", "")
	bread := a.newProduct(t, "bread", "")

	a.addProducts(t, created.ID, owner.ID, milk)
	a.addProducts(t, created.ID, owner.ID, bread)

	err := a.lists.ReoderStates(ginCtx(), owner.ID, created.ID, []id.ID[product.Product]{milk.ID})
	require.ErrorIs(t, err, myerr.ErrInvalidArgument,
		"a reorder that is not a full permutation should be rejected")
}

func TestListAppendAndDeleteMembers(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	a := newApp(t)

	owner := a.newUser(t, "owner")
	editor := a.newUser(t, "editor")
	created := a.newList(t, owner.ID, "groceries")

	a.addMember(t, created.ID, owner.ID, editor.ID, list.MemberTypeEditor)

	got, err := a.lists.GetByID(ctx, created.ID, editor.ID)
	require.NoError(t, err)
	require.ElementsMatch(t, []id.ID[user.User]{owner.ID, editor.ID}, memberIDs(got))

	// The list now shows up for the new member too.
	editorLists, err := a.lists.GetByUserID(ctx, editor.ID)
	require.NoError(t, err)
	require.Len(t, editorLists, 1)

	_, err = a.lists.DeleteMembers(ctx, created.ID, owner.ID, []id.ID[user.User]{editor.ID})
	require.NoError(t, err)

	after, err := a.lists.GetByID(ctx, created.ID, owner.ID)
	require.NoError(t, err)
	require.Equal(t, []id.ID[user.User]{owner.ID}, memberIDs(after))

	_, err = a.lists.GetByID(ctx, created.ID, editor.ID)
	require.ErrorIs(t, err, myerr.ErrForbidden)
}

func TestListAppendDuplicateMemberRejected(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	a := newApp(t)

	owner := a.newUser(t, "owner")
	created := a.newList(t, owner.ID, "groceries")

	_, err := a.lists.AppendMembers(ctx, created.ID, owner.ID, []list.MemberOptions{
		{UserID: owner.ID, Role: list.MemberTypeEditor},
	})
	require.ErrorIs(t, err, myerr.ErrInvalidArgument)

	got, err := a.lists.GetByID(ctx, created.ID, owner.ID)
	require.NoError(t, err)
	require.Len(t, got.Members, 1, "the failed append must not have been persisted")
}

func TestListDeleteUnknownMember(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	a := newApp(t)

	owner := a.newUser(t, "owner")
	created := a.newList(t, owner.ID, "groceries")

	_, err := a.lists.DeleteMembers(ctx, created.ID, owner.ID, []id.ID[user.User]{id.NewID[user.User]()})
	require.ErrorIs(t, err, myerr.ErrNotFound)
}

// A member may always remove themselves, whatever their role — that is the one path that
// skips the admin check.
func TestListMemberCanRemoveThemselves(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	a := newApp(t)

	owner := a.newUser(t, "owner")
	viewer := a.newUser(t, "viewer")
	created := a.newList(t, owner.ID, "groceries")

	a.addMember(t, created.ID, owner.ID, viewer.ID, list.MemberTypeViewer)

	_, err := a.lists.DeleteMembers(ctx, created.ID, viewer.ID, []id.ID[user.User]{viewer.ID})
	require.NoError(t, err)

	got, err := a.lists.GetByID(ctx, created.ID, owner.ID)
	require.NoError(t, err)
	require.Equal(t, []id.ID[user.User]{owner.ID}, memberIDs(got))
}

// Nothing stops the owner from leaving, which strands the list: it still exists, still has its
// states, and nobody can reach it any more.
func TestListOwnerCanOrphanTheList(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	a := newApp(t)

	owner := a.newUser(t, "owner")
	created := a.newList(t, owner.ID, "groceries")

	_, err := a.lists.DeleteMembers(ctx, created.ID, owner.ID, []id.ID[user.User]{owner.ID})
	require.NoError(t, err)

	_, err = a.lists.GetByID(ctx, created.ID, owner.ID)
	require.ErrorIs(t, err, myerr.ErrForbidden)

	require.Equal(t, 1, a.count(t, `SELECT count(*) FROM product_lists WHERE id = ?`, created.ID.String()))
	require.Equal(t, 0, a.count(t,
		`SELECT count(*) FROM product_list_members WHERE list_id = ?`, created.ID.String()))
}

func TestListDelete(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	a := newApp(t)

	owner := a.newUser(t, "owner")
	created := a.newList(t, owner.ID, "groceries")
	milk := a.newProduct(t, "milk", "")
	a.addProducts(t, created.ID, owner.ID, milk)

	require.NoError(t, a.lists.DeleteList(ctx, owner.ID, created.ID))

	require.Equal(t, 0, a.count(t, `SELECT count(*) FROM product_lists WHERE id = ?`, created.ID.String()))

	got, err := a.lists.GetByUserID(ctx, owner.ID)
	require.NoError(t, err)
	require.Empty(t, got)
}

// Members are declared with constraint:OnDelete:CASCADE, states are not. With PRAGMA
// foreign_keys off neither constraint actually fires, so deleting a list leaves both tables
// behind. Whatever the sqlc rewrite does here, it has to be a deliberate choice.
func TestListDeleteLeavesOrphanRows_CurrentBehaviour(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	a := newApp(t)

	owner := a.newUser(t, "owner")
	created := a.newList(t, owner.ID, "groceries")
	milk := a.newProduct(t, "milk", "")
	a.addProducts(t, created.ID, owner.ID, milk)

	require.NoError(t, a.lists.DeleteList(ctx, owner.ID, created.ID))

	require.Equal(t, 1, a.count(t,
		`SELECT count(*) FROM product_list_states WHERE list_id = ?`, created.ID.String()),
		"states are orphaned: no cascade is declared and foreign keys are off anyway")
	require.Equal(t, 1, a.count(t,
		`SELECT count(*) FROM product_list_members WHERE list_id = ?`, created.ID.String()),
		"members declare ON DELETE CASCADE, but PRAGMA foreign_keys is off so it never fires")
}

func TestListDeleteCleansUpChildren_Desired(t *testing.T) {
	t.Parallel()

	t.Skip("KNOWN BUG: deleting a list orphans its states and members; " +
		"see TestListDeleteLeavesOrphanRows_CurrentBehaviour")

	ctx := context.Background()
	a := newApp(t)

	owner := a.newUser(t, "owner")
	created := a.newList(t, owner.ID, "groceries")
	a.addProducts(t, created.ID, owner.ID, a.newProduct(t, "milk", ""))

	require.NoError(t, a.lists.DeleteList(ctx, owner.ID, created.ID))

	require.Equal(t, 0, a.count(t,
		`SELECT count(*) FROM product_list_states WHERE list_id = ?`, created.ID.String()))
	require.Equal(t, 0, a.count(t,
		`SELECT count(*) FROM product_list_members WHERE list_id = ?`, created.ID.String()))
}

// KNOWN BUG (current behaviour pinned). getProductList uses Find, not First, so a missing list
// yields a zero-valued model with a nil UUID instead of gorm.ErrRecordNotFound. The service
// then runs its role check against an empty member list and reports ErrForbidden, which the
// REST layer turns into 403 where 404 is meant.
func TestListGetMissing_CurrentBehaviour(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	a := newApp(t)

	owner := a.newUser(t, "owner")

	_, err := a.lists.GetByID(ctx, id.NewID[list.ProductList](), owner.ID)
	require.ErrorIs(t, err, myerr.ErrForbidden)
	require.NotErrorIs(t, err, myerr.ErrNotFound)
}

func TestListGetMissing_Desired(t *testing.T) {
	t.Parallel()

	t.Skip("KNOWN BUG: a missing list reports ErrForbidden; see TestListGetMissing_CurrentBehaviour")

	ctx := context.Background()
	a := newApp(t)

	owner := a.newUser(t, "owner")

	_, err := a.lists.GetByID(ctx, id.NewID[list.ProductList](), owner.ID)
	require.ErrorIs(t, err, myerr.ErrNotFound)
}
