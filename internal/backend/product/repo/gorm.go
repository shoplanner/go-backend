package repo

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/samber/lo"
	"github.com/samber/mo"

	"go-backend/internal/backend/product"
	"go-backend/pkg/date"
	"go-backend/pkg/god"
	"go-backend/pkg/id"
)

type Repo struct{ db *sql.DB }

func NewRepo(ctx context.Context, db *sql.DB) (*Repo, error) {
	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS products (
		id TEXT PRIMARY KEY,
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL,
		name TEXT NOT NULL,
		category TEXT,
		forms_json TEXT NOT NULL
	)`); err != nil {
		return nil, wrapErr(fmt.Errorf("can't initialize product tables: %w", err))
	}
	return &Repo{db: db}, nil
}

func (r *Repo) GetAndUpdate(ctx context.Context, productID id.ID[product.Product], updateFunc func(product.Product) (product.Product, error)) (product.Product, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return product.Product{}, wrapErr(err)
	}
	defer tx.Rollback()

	current, err := getByID(ctx, tx, productID)
	if err != nil {
		return product.Product{}, err
	}
	updated, err := updateFunc(current)
	if err != nil {
		return product.Product{}, err
	}
	if err = upsert(ctx, tx, updated); err != nil {
		return product.Product{}, err
	}
	if err = tx.Commit(); err != nil {
		return product.Product{}, wrapErr(err)
	}
	return updated, nil
}

func (r *Repo) GetByID(ctx context.Context, productID id.ID[product.Product]) (product.Product, error) {
	return getByID(ctx, r.db, productID)
}

func (r *Repo) GetByListID(ctx context.Context, idList []id.ID[product.Product]) ([]product.Product, error) {
	if len(idList) == 0 {
		return []product.Product{}, nil
	}
	args := make([]any, 0, len(idList))
	ph := make([]string, 0, len(idList))
	for _, pid := range idList {
		args = append(args, pid.String())
		ph = append(ph, "?")
	}
	rows, err := r.db.QueryContext(ctx, `SELECT id, created_at, updated_at, name, category, forms_json FROM products WHERE id IN (`+strings.Join(ph, ",")+`)`, args...)
	if err != nil {
		return nil, wrapErr(err)
	}
	defer rows.Close()

	products := []product.Product{}
	for rows.Next() {
		m, err := scanProduct(rows)
		if err != nil {
			return nil, err
		}
		products = append(products, m)
	}
	return products, rows.Err()
}

func (r *Repo) Create(ctx context.Context, model product.Product) error {
	return upsert(ctx, r.db, model)
}

func (r *Repo) Delete(ctx context.Context, productID id.ID[product.Product]) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM products WHERE id = ?`, productID.String())
	return wrapErr(err)
}

func getByID(ctx context.Context, db interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, productID id.ID[product.Product]) (product.Product, error) {
	row := db.QueryRowContext(ctx, `SELECT id, created_at, updated_at, name, category, forms_json FROM products WHERE id = ?`, productID.String())
	return scanProductRow(row)
}

func upsert(ctx context.Context, db interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}, model product.Product) error {
	forms, _ := json.Marshal(lo.Map(model.Forms, func(f product.Form, _ int) string { return string(f) }))
	_, err := db.ExecContext(ctx, `INSERT INTO products(id, created_at, updated_at, name, category, forms_json)
	VALUES(?,?,?,?,?,?)
	ON CONFLICT(id) DO UPDATE SET created_at=excluded.created_at, updated_at=excluded.updated_at, name=excluded.name, category=excluded.category, forms_json=excluded.forms_json`,
		model.ID.String(), model.CreatedAt.Time, model.UpdatedAt.Time, string(model.Name), sql.NullString{String: string(model.Category.OrEmpty()), Valid: model.Category.IsPresent()}, string(forms))
	return wrapErr(err)
}

func scanProduct(rows *sql.Rows) (product.Product, error) {
	var idS, name, formsJSON string
	var createdAt, updatedAt time.Time
	var category sql.NullString
	if err := rows.Scan(&idS, &createdAt, &updatedAt, &name, &category, &formsJSON); err != nil {
		return product.Product{}, wrapErr(err)
	}
	return rowToModel(idS, createdAt, updatedAt, name, category, formsJSON)
}
func scanProductRow(row *sql.Row) (product.Product, error) {
	var idS, name, formsJSON string
	var createdAt, updatedAt time.Time
	var category sql.NullString
	if err := row.Scan(&idS, &createdAt, &updatedAt, &name, &category, &formsJSON); err != nil {
		return product.Product{}, wrapErr(err)
	}
	return rowToModel(idS, createdAt, updatedAt, name, category, formsJSON)
}
func rowToModel(idS string, createdAt, updatedAt time.Time, name string, category sql.NullString, formsJSON string) (product.Product, error) {
	formsRaw := []string{}
	if err := json.Unmarshal([]byte(formsJSON), &formsRaw); err != nil {
		return product.Product{}, wrapErr(err)
	}
	cat := mo.None[product.Category]()
	if category.Valid {
		cat = mo.Some(product.Category(category.String))
	}
	return product.Product{Options: product.Options{Name: product.Name(name), Category: cat, Forms: lo.Map(formsRaw, func(v string, _ int) product.Form { return product.Form(v) })}, ID: id.ID[product.Product]{UUID: god.Believe(uuid.Parse(idS))}, CreatedAt: date.CreateDate[product.Product]{Time: createdAt}, UpdatedAt: date.UpdateDate[product.Product]{Time: updatedAt}}, nil
}

func wrapErr(err error) error {
	if err != nil {
		return fmt.Errorf("product storage: %w", err)
	}
	return nil
}
