package repo

import (
	"database/sql"

	"go-backend/internal/backend/product/repo/sqlgen"
)

//go:generate $SQLC_HELPER

type Repo struct {
	queries *sqlgen.Queries
	db      *sql.DB
}

func NewRepo(db *sql.DB) *Repo {
	return &Repo{
		queries: sqlgen.New(db),
		db:      db,
	}
}
