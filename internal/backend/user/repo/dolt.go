package repo

import (
	"context"
	"fmt"

	"github.com/go-sql-driver/mysql"
	"github.com/google/uuid"
	"github.com/samber/lo"
	"gorm.io/gorm"

	"go-backend/internal/backend/user"
	"go-backend/internal/backend/user/repo/sqlgen"
	"go-backend/pkg/id"
	"go-backend/pkg/myerr"
	"go-backend/pkg/mymysql"
)

//go:generate $SQLC_HELPER

type User struct {
	ID    string `gorm:"primaryKey;size:36"`
	Login string `gorm:"size:255"`
	Hash  string
	Role  int32
}

type Repo struct {
	queries *sqlgen.Queries
	db      *gorm.DB
}

func NewRepo(ctx context.Context, conn sqlgen.DBTX, gormDB *gorm.DB) (*Repo, error) {
	q := sqlgen.New(conn)

	if err := gormDB.AutoMigrate(new(User)); err != nil {
		return nil, fmt.Errorf("can't create user tables: %w", err)
	}

	if err := q.InitUsers(ctx); err != nil {
		return nil, fmt.Errorf("can't init users table: %w", err)
	}
	return &Repo{
		queries: q,
		db:      gormDB,
	}, nil
}

func (r *Repo) GetByLogin(ctx context.Context, login user.Login) (user.User, error) {
	model, err := r.queries.GetByLogin(ctx, string(login))
	if err != nil {
		return user.User{}, fmt.Errorf("can't find user in database: %w", err)
	}

	return sqlcToUser(model, 0), nil
}

func (r *Repo) Create(ctx context.Context, model user.User) error {
	_, err := r.queries.CreateUser(ctx, sqlgen.CreateUserParams{
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
	models, err := r.queries.GetAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("can't get users from Dolt DB: %w", err)
	}

	return lo.Map(models, sqlcToUser), nil
}

func (r *Repo) GetByID(ctx context.Context, userID id.ID[user.User]) (user.User, error) {
	model, err := r.queries.GetByID(ctx, userID.String())
	if err != nil {
		return sqlcToUser(model, 0), fmt.Errorf("can't get user %s from DoltDB: %w", userID, err)
	}

	return sqlcToUser(model, 0), nil
}

func sqlcToUser(item sqlgen.User, _ int) user.User {
	userID, _ := uuid.Parse(item.ID)
	return user.User{
		ID:           id.ID[user.User]{UUID: userID},
		Role:         user.Role(item.Role),
		Login:        user.Login(item.Login),
		PasswordHash: user.Hash(item.Hash),
	}
}
