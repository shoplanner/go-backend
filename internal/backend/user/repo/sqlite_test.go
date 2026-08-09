package repo_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/suite"

	"go-backend/internal/backend/user"
	"go-backend/internal/backend/user/repo"
	"go-backend/pkg/id"
	"go-backend/pkg/myerr"
)

type RepoSuite struct {
	suite.Suite

	repo *repo.Repo
}

func TestRepoSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(RepoSuite))
}

func (s *RepoSuite) SetupTest() {
	db, err := sql.Open("sqlite3", filepath.Join(s.T().TempDir(), "users.db"))
	s.Require().NoError(err)
	s.T().Cleanup(func() { s.Require().NoError(db.Close()) })

	userRepo, err := repo.NewRepo(context.Background(), db)
	s.Require().NoError(err)

	s.repo = userRepo
}

func (s *RepoSuite) newUser(login user.Login) user.User {
	return user.User{
		ID:           id.NewID[user.User](),
		Role:         user.RoleUser,
		Login:        login,
		PasswordHash: "hash",
	}
}

func (s *RepoSuite) TestCreateAndGet() {
	ctx := context.Background()
	model := s.newUser("vasya")

	s.Require().NoError(s.repo.Create(ctx, model))

	byLogin, err := s.repo.GetByLogin(ctx, model.Login)
	s.Require().NoError(err)
	s.Equal(model, byLogin)

	byID, err := s.repo.GetByID(ctx, model.ID)
	s.Require().NoError(err)
	s.Equal(model, byID)
}

// A duplicate login has to surface as myerr.ErrAlreadyExists, which only works if the
// SQLite UNIQUE violation is recognised by pkg/mysqlite.
func (s *RepoSuite) TestCreateDuplicateLogin() {
	ctx := context.Background()

	s.Require().NoError(s.repo.Create(ctx, s.newUser("vasya")))

	err := s.repo.Create(ctx, s.newUser("vasya"))
	s.Require().ErrorIs(err, myerr.ErrAlreadyExists)
}
