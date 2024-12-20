package repo

import (
	"context"
	"fmt"

	"github.com/go-sql-driver/mysql"
	"github.com/google/uuid"
	"github.com/samber/lo"

	"go-backend/internal/backend/user"
	"go-backend/internal/backend/user/repo/sqlc"
	"go-backend/pkg/id"
	"go-backend/pkg/myerr"
	"go-backend/pkg/mymysql"
)

type Repo struct {
	db *sqlc.Queries
}

func NewRepo(conn sqlc.DBTX) *Repo {
	return &Repo{db: sqlc.New(conn)}
}

func (r *Repo) GetByLogin(ctx context.Context, login user.Login) (user.User, error) {
	model, err := r.db.GetByLogin(ctx, string(login))
	if err != nil {
		return user.User{}, fmt.Errorf("can't find user in database: %w", err)
	}

	return sqlcToUser(model, 0), nil
}

func (r *Repo) Create(ctx context.Context, model user.User) error {
	_, err := r.db.CreateUser(ctx, sqlc.CreateUserParams{
		ID:    model.ID.String(),
		Login: string(model.Login),
		Hash:  string(model.PasswordHash),
		Role:  int32(model.Role),
	})
	if sqlErr, casted := lo.ErrorsAs[*mysql.MySQLError](err); casted {
		if sqlErr.Number == mymysql.DublicateEntryNumber {
			return fmt.Errorf("%w: such user already exists", myerr.ErrAlreadyExists)
		}
	}
	if err != nil {
		return fmt.Errorf("can't insert user in database: %w", err)
	}

	return nil
}

func (r *Repo) GetAll(ctx context.Context) ([]user.User, error) {
	models, err := r.db.GetAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("can't get users from Dolt DB: %w", err)
	}

	return lo.Map(models, sqlcToUser), nil
}

func sqlcToUser(item sqlc.User, _ int) user.User {
	return user.User{
		ID:           id.ID[user.User]{UUID: uuid.MustParse(item.ID)},
		Role:         user.Role(item.Role),
		Login:        user.Login(item.Login),
		PasswordHash: user.Hash(item.Hash),
	}
}
