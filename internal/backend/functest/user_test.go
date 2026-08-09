package functest_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"go-backend/internal/backend/favorite"
	"go-backend/internal/backend/user"
	"go-backend/pkg/id"
	"go-backend/pkg/myerr"
)

func TestUserCreateAndRead(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	a := newApp(t)

	created := a.newUser(t, "vasya")
	require.Equal(t, user.Login("vasya"), created.Login)
	require.Equal(t, user.RoleUser, created.Role)
	require.NotEmpty(t, created.PasswordHash)
	require.NotEqual(t, testPassword, string(created.PasswordHash), "password must be hashed")

	byID, err := a.users.GetByID(ctx, created.ID)
	require.NoError(t, err)
	require.Equal(t, created, byID)

	all, err := a.users.GetAllUsers(ctx)
	require.NoError(t, err)
	require.Len(t, all, 1)
	require.Equal(t, created, all[0])
}

// The service checks for an existing login before inserting, so the duplicate is reported
// before the UNIQUE index is ever consulted.
func TestUserCreateDuplicateLogin(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	a := newApp(t)

	a.newUser(t, "vasya")

	_, err := a.users.Create(ctx, user.CreateOptions{Login: "vasya", Password: testPassword})
	require.ErrorIs(t, err, myerr.ErrAlreadyExists)
}

func TestUserCreateRejectsOverlongPassword(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	a := newApp(t)

	const tooLong = 73

	_, err := a.users.Create(ctx, user.CreateOptions{
		Login:    "vasya",
		Password: string(make([]byte, tooLong)),
	})
	require.ErrorIs(t, err, myerr.ErrInvalidArgument)
}

func TestUserValidatePassword(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	a := newApp(t)

	created := a.newUser(t, "vasya")

	authorized, err := a.users.ValidatePassword(ctx, created.Login, testPassword)
	require.NoError(t, err)
	require.Equal(t, created, authorized)

	_, err = a.users.ValidatePassword(ctx, created.Login, testPassword+"x")
	require.ErrorIs(t, err, user.ErrAuthorizationFailure)

	_, err = a.users.ValidatePassword(ctx, "nobody", testPassword)
	require.Error(t, err)
}

func TestUserGetByIDMissing(t *testing.T) {
	t.Parallel()

	a := newApp(t)

	_, err := a.users.GetByID(context.Background(), id.NewID[user.User]())
	require.Error(t, err)
}

// Registering a user fans out to every user.Subscriber. favorite/service is the only one, and
// it creates a personal favorites list owned by the new user. This cross-domain side effect is
// wired in main.go through NewService(repo, users), so it has to keep working after the
// favorites repo is rewritten.
func TestUserCreationCreatesPersonalFavoritesList(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	a := newApp(t)

	created := a.newUser(t, "vasya")

	lists, err := a.favorites.GetListsByUserID(ctx, created.ID)
	require.NoError(t, err)
	require.Len(t, lists, 1)

	personal := lists[0]
	require.Equal(t, favorite.ListTypePersonal, personal.Type)
	require.Empty(t, personal.Products)
	require.Len(t, personal.Members, 1)
	require.Equal(t, created.ID, personal.Members[0].UserID)
	require.Equal(t, favorite.MemberTypeOwner, personal.Members[0].Type)

	requireRecent(t, personal.CreatedAt.Time)
	requireRecent(t, personal.UpdatedAt.Time)

	goldenJSON(t, "user_personal_favorites_list", personal)
}

// Each user gets exactly one list, and users do not see each other's.
func TestPersonalFavoritesListsAreIsolated(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	a := newApp(t)

	first := a.newUser(t, "vasya")
	second := a.newUser(t, "petya")

	firstLists, err := a.favorites.GetListsByUserID(ctx, first.ID)
	require.NoError(t, err)
	require.Len(t, firstLists, 1)

	secondLists, err := a.favorites.GetListsByUserID(ctx, second.ID)
	require.NoError(t, err)
	require.Len(t, secondLists, 1)

	require.NotEqual(t, firstLists[0].ID, secondLists[0].ID)
	require.Equal(t, 2, a.count(t, `SELECT count(*) FROM favorite_lists`))
	require.Equal(t, 2, a.count(t, `SELECT count(*) FROM favorite_members`))
}
