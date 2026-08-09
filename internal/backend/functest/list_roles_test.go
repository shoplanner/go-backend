package functest_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"go-backend/internal/backend/list"
	"go-backend/internal/backend/product"
	"go-backend/internal/backend/user"
	"go-backend/pkg/id"
	"go-backend/pkg/myerr"
)

// scene is a list that already has one product state and one member holding the role under
// test, so every mutation can be attempted from that member's point of view.
type scene struct {
	listID id.ID[list.ProductList]
	actor  id.ID[user.User]
	milk   product.Product
	bread  product.Product
}

// newScene builds the fixture. When role is MemberTypeOwner the actor *is* the owner;
// otherwise a second user joins with the requested role.
func (a *app) newScene(t *testing.T, role list.MemberType) scene {
	t.Helper()

	owner := a.newUser(t, "owner")
	listModel := a.newList(t, owner.ID, "groceries")

	milk := a.newProduct(t, "milk", "dairy")
	bread := a.newProduct(t, "bread", "bakery")
	a.addProducts(t, listModel.ID, owner.ID, milk)

	actor := owner.ID
	if role != list.MemberTypeOwner {
		member := a.newUser(t, "member")
		a.addMember(t, listModel.ID, owner.ID, member.ID, role)
		actor = member.ID
	}

	return scene{listID: listModel.ID, actor: actor, milk: milk, bread: bread}
}

// TestListRoleMatrix pins which role each mutation demands.
//
// list.CheckRole compares numerically — `required < member.Role` is a rejection — and the enum
// runs owner=1, admin=2, editor=3, executing=4, viewer=5, so a *lower* number means more
// power. That inversion is easy to get backwards in a rewrite, which is why the whole matrix
// is spelled out rather than spot-checked.
//
// Note the asymmetry it exposes: adding and removing products needs only an editor, but
// editing the state of a product already on the list needs an admin.
func TestListRoleMatrix(t *testing.T) {
	t.Parallel()

	operations := []struct {
		name     string
		required list.MemberType
		run      func(ctx context.Context, a *app, s scene) error
	}{
		{
			name:     "Update",
			required: list.MemberTypeAdmin,
			run: func(ctx context.Context, a *app, s scene) error {
				_, err := a.lists.Update(ctx, s.listID, s.actor, list.ListOptions{
					Status: list.ExecStatusProcessing,
					Title:  "renamed",
				})

				return err
			},
		},
		{
			name:     "UpdateProductState",
			required: list.MemberTypeAdmin,
			run: func(ctx context.Context, a *app, s scene) error {
				_, err := a.lists.UpdateProductState(ctx, s.listID, s.actor, s.milk.ID, zeroStateOptions())

				return err
			},
		},
		{
			name:     "AppendProducts",
			required: list.MemberTypeEditor,
			run: func(ctx context.Context, a *app, s scene) error {
				_, err := a.lists.AppendProducts(ctx, s.listID, s.actor,
					map[id.ID[product.Product]]list.ProductStateOptions{s.bread.ID: zeroStateOptions()})

				return err
			},
		},
		{
			name:     "DeleteProducts",
			required: list.MemberTypeEditor,
			run: func(ctx context.Context, a *app, s scene) error {
				_, err := a.lists.DeleteProducts(ctx, s.listID, s.actor, []id.ID[product.Product]{s.milk.ID})

				return err
			},
		},
		{
			name:     "ReoderStates",
			required: list.MemberTypeEditor,
			run: func(_ context.Context, a *app, s scene) error {
				return a.lists.ReoderStates(ginCtx(), s.actor, s.listID, []id.ID[product.Product]{s.milk.ID})
			},
		},
		{
			name:     "DeleteList",
			required: list.MemberTypeOwner,
			run: func(ctx context.Context, a *app, s scene) error {
				return a.lists.DeleteList(ctx, s.actor, s.listID)
			},
		},
		{
			name:     "GetByID",
			required: list.MemberTypeViewer,
			run: func(ctx context.Context, a *app, s scene) error {
				_, err := a.lists.GetByID(ctx, s.listID, s.actor)

				return err
			},
		},
	}

	roles := []list.MemberType{
		list.MemberTypeOwner,
		list.MemberTypeAdmin,
		list.MemberTypeEditor,
		list.MemberTypeExecuting,
		list.MemberTypeViewer,
	}

	for _, op := range operations {
		for _, role := range roles {
			t.Run(op.name+"/"+role.String(), func(t *testing.T) {
				t.Parallel()

				a := newApp(t)
				s := a.newScene(t, role)

				err := op.run(context.Background(), a, s)

				if role <= op.required {
					require.NoError(t, err, "%s is at least %s and must be allowed", role, op.required)

					return
				}

				require.ErrorIs(t, err, myerr.ErrForbidden,
					"%s is weaker than %s and must be rejected", role, op.required)
			})
		}
	}
}

func TestListNonMemberIsRejected(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	a := newApp(t)

	s := a.newScene(t, list.MemberTypeOwner)
	stranger := a.newUser(t, "stranger")

	_, err := a.lists.GetByID(ctx, s.listID, stranger.ID)
	require.ErrorIs(t, err, myerr.ErrForbidden)

	_, err = a.lists.Update(ctx, s.listID, stranger.ID, list.ListOptions{
		Status: list.ExecStatusArchived,
		Title:  "hijacked",
	})
	require.ErrorIs(t, err, myerr.ErrForbidden)

	require.ErrorIs(t, a.lists.DeleteList(ctx, stranger.ID, s.listID), myerr.ErrForbidden)
}

// KNOWN BUG (current behaviour pinned). AppendMembers has its role check commented out with a
// FIXME, so any member — even a viewer — and in fact any user at all can add members to any
// list, including granting themselves ownership.
func TestListAppendMembersSkipsRoleCheck_CurrentBehaviour(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	a := newApp(t)

	s := a.newScene(t, list.MemberTypeViewer)
	stranger := a.newUser(t, "stranger")

	// A viewer promotes an outsider to owner.
	_, err := a.lists.AppendMembers(ctx, s.listID, s.actor, []list.MemberOptions{
		{UserID: stranger.ID, Role: list.MemberTypeOwner},
	})
	require.NoError(t, err, "AppendMembers performs no authorization at all")

	got, err := a.lists.GetByID(ctx, s.listID, stranger.ID)
	require.NoError(t, err)
	require.Len(t, got.Members, 3)
}

func TestListAppendMembersRequiresAdmin_Desired(t *testing.T) {
	t.Parallel()

	t.Skip("KNOWN BUG: AppendMembers has its role check commented out (FIXME in list/service); " +
		"see TestListAppendMembersSkipsRoleCheck_CurrentBehaviour")

	ctx := context.Background()
	a := newApp(t)

	s := a.newScene(t, list.MemberTypeViewer)
	stranger := a.newUser(t, "stranger")

	_, err := a.lists.AppendMembers(ctx, s.listID, s.actor, []list.MemberOptions{
		{UserID: stranger.ID, Role: list.MemberTypeOwner},
	})
	require.ErrorIs(t, err, myerr.ErrForbidden)
}

// KNOWN BUG (current behaviour pinned). AppendProducts validates the *old* list instead of the
// one it just built, so the duplicate check in validate() never sees the new states. The same
// product can be added twice, and nothing at the storage level stops it either — unlike
// members, product states have no unique index.
func TestListAppendDuplicateProduct_CurrentBehaviour(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	a := newApp(t)

	owner := a.newUser(t, "owner")
	listModel := a.newList(t, owner.ID, "groceries")
	milk := a.newProduct(t, "milk", "dairy")

	a.addProducts(t, listModel.ID, owner.ID, milk)
	a.addProducts(t, listModel.ID, owner.ID, milk)

	require.Equal(t, 2, a.count(t,
		`SELECT count(*) FROM product_list_states WHERE list_id = ? AND product_id = ?`,
		listModel.ID.String(), milk.ID.String()))

	got, err := a.lists.GetByID(ctx, listModel.ID, owner.ID)
	require.NoError(t, err)
	require.Len(t, got.States, 2)
	require.Equal(t, []id.ID[product.Product]{milk.ID, milk.ID}, productIDs(got))
}

func TestListAppendDuplicateProduct_Desired(t *testing.T) {
	t.Parallel()

	t.Skip("KNOWN BUG: AppendProducts validates oldList instead of newList; " +
		"see TestListAppendDuplicateProduct_CurrentBehaviour")

	ctx := context.Background()
	a := newApp(t)

	owner := a.newUser(t, "owner")
	listModel := a.newList(t, owner.ID, "groceries")
	milk := a.newProduct(t, "milk", "dairy")

	a.addProducts(t, listModel.ID, owner.ID, milk)

	_, err := a.lists.AppendProducts(ctx, listModel.ID, owner.ID,
		map[id.ID[product.Product]]list.ProductStateOptions{milk.ID: zeroStateOptions()})
	require.ErrorIs(t, err, myerr.ErrInvalidArgument)
}
