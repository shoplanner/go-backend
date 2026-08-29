package repo

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/samber/lo"

	"go-backend/internal/backend/user"
	"go-backend/internal/backend/user/repo/sqlgen"
	"go-backend/pkg/id"
	"go-backend/pkg/myerr"
	"go-backend/pkg/mysqlite"
)

//go:generate python $SQLC_HELPER

// createIndexQuery duplicates the CREATE UNIQUE INDEX in schema.sql, because sqlc generates no
// query for a CREATE INDEX statement.
const createIndexQuery = `CREATE UNIQUE INDEX IF NOT EXISTS idx_users_login ON users(login)`

// ErrDuplicateLogins is what NewRepo fails with when the database it is handed already holds
// two accounts under one login, which no GORM-era database was stopped from doing. The index
// cannot be created over such rows, and the server cannot run without it, so this is fatal by
// design — the operator has to say which account keeps the login. cmd/dedup-logins does that.
var ErrDuplicateLogins = errors.New("users.login holds duplicates")

type Repo struct {
	queries *sqlgen.Queries
}

func NewRepo(ctx context.Context, conn sqlgen.DBTX) (*Repo, error) {
	q := sqlgen.New(conn)

	if err := q.InitUsers(ctx); err != nil {
		return nil, fmt.Errorf("can't init users table: %w", err)
	}

	if _, err := conn.ExecContext(ctx, createIndexQuery); err != nil {
		if !mysqlite.IsUniqueViolation(err) {
			return nil, fmt.Errorf("can't init users login index: %w", err)
		}

		// "UNIQUE constraint failed: users.login" on its own says nothing about which login
		// is at fault, so name them: this error is the only thing the operator gets, and it
		// is printed by a process that is about to exit.
		return nil, fmt.Errorf(
			"can't init users login index: %w: %s; run `shoplannerctl db dedup-logins` to resolve them",
			ErrDuplicateLogins, describeDuplicates(ctx, conn))
	}

	return &Repo{queries: q}, nil
}

// describeDuplicates renders the offending logins for ErrDuplicateLogins, e.g.
// `"alice" x2, "bob" x3`. It is best effort: it runs while a boot is already failing, so a
// second failure here must not replace the diagnosis with its own error.
func describeDuplicates(ctx context.Context, conn sqlgen.DBTX) string {
	rows, err := FindDuplicateLogins(ctx, conn)
	if err != nil {
		return fmt.Sprintf("(the duplicate logins could not be listed: %s)", err)
	}

	counts := map[user.Login]int{}
	for _, row := range rows {
		counts[row.Login]++
	}

	logins := lo.Keys(counts)
	sort.Slice(logins, func(i, j int) bool { return logins[i] < logins[j] })

	return strings.Join(lo.Map(logins, func(login user.Login, _ int) string {
		return fmt.Sprintf("%q x%d", string(login), counts[login])
	}), ", ")
}

// LoadLogins resolves user ids to logins.
//
// It takes a sqlgen.DBTX rather than a *Repo so the list repo can call it with its own *sql.Tx
// and read the logins inside its transaction, which is what GORM's Preload("Members.User") did.
func LoadLogins(ctx context.Context, db sqlgen.DBTX, ids []string) (map[string]user.Login, error) {
	rows, err := sqlgen.New(db).GetLoginsByIDList(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("can't get user logins from database: %w", err)
	}

	logins := make(map[string]user.Login, len(rows))
	for _, row := range rows {
		logins[row.ID] = user.Login(row.Login)
	}

	return logins, nil
}

// FindDuplicateLogins returns every account whose login is shared with another account,
// grouped together by the query's ORDER BY.
//
// Like LoadLogins it takes a sqlgen.DBTX rather than a *Repo, because both of its callers work
// on a database the repo cannot be constructed against: NewRepo itself, while it is failing,
// and cmd/dedup-logins, which runs before the server can start at all.
func FindDuplicateLogins(ctx context.Context, db sqlgen.DBTX) ([]user.User, error) {
	rows, err := sqlgen.New(db).FindDuplicateLogins(ctx)
	if err != nil {
		return nil, fmt.Errorf("can't read duplicate logins from database: %w", err)
	}

	return lo.Map(rows, sqlcToUser), nil
}

// SetLogin renames one account. Administrative: only cmd/dedup-logins calls it.
func SetLogin(ctx context.Context, db sqlgen.DBTX, userID id.ID[user.User], login user.Login) error {
	err := sqlgen.New(db).SetLogin(ctx, sqlgen.SetLoginParams{
		Login: string(login),
		ID:    userID.String(),
	})
	if mysqlite.IsUniqueViolation(err) {
		return fmt.Errorf("%w: login %q is taken", myerr.ErrAlreadyExists, string(login))
	}

	if err != nil {
		return fmt.Errorf("can't rename user %s: %w", userID, err)
	}

	return nil
}

// Delete removes one account, and nothing else: rows in other domains that reference it are
// left alone. Administrative — only cmd/dedup-logins calls it, and only for accounts it has
// established nothing references.
func Delete(ctx context.Context, db sqlgen.DBTX, userID id.ID[user.User]) error {
	if err := sqlgen.New(db).DeleteUser(ctx, userID.String()); err != nil {
		return fmt.Errorf("can't delete user %s: %w", userID, err)
	}

	return nil
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
		Role:  int64(model.Role),
	})
	if mysqlite.IsUniqueViolation(err) {
		return fmt.Errorf("%w: such user already exists", myerr.ErrAlreadyExists)
	}
	if err != nil {
		return fmt.Errorf("can't insert user in database: %w", err)
	}

	return nil
}

func (r *Repo) GetAll(ctx context.Context) ([]user.User, error) {
	models, err := r.queries.GetAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("can't get users from database: %w", err)
	}

	return lo.Map(models, sqlcToUser), nil
}

func (r *Repo) GetByID(ctx context.Context, userID id.ID[user.User]) (user.User, error) {
	model, err := r.queries.GetByID(ctx, userID.String())
	if err != nil {
		return sqlcToUser(model, 0), fmt.Errorf("can't get user %s from database: %w", userID, err)
	}

	return sqlcToUser(model, 0), nil
}

func sqlcToUser(item sqlgen.User, _ int) user.User {
	userID, _ := uuid.Parse(item.ID)
	return user.User{
		ID:           id.ID[user.User]{UUID: userID},
		//nolint:gosec // role is written by this package from a user.Role, so it always fits back into int32
		Role:         user.Role(item.Role),
		Login:        user.Login(item.Login),
		PasswordHash: user.Hash(item.Hash),
	}
}
