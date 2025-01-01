package repo

import (
	"context"
	"database/sql"

	"go-backend/internal/backend/product"
	"go-backend/internal/backend/product/repo/sqlgen"
	"go-backend/pkg/id"
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

func (r *Repo) CreateProduct(ctx context.Context, model product.Product) error {
}

func (r *Repo) GetProduct(ctx context.Context, productID id.ID[product.Product]) (product.Product, error) {
}
