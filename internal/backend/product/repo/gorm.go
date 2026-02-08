package repo

import (
	"context"
	"database/sql"
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

type Product struct {
	ID         string
	CreatedAt  time.Time
	UpdatedAt  time.Time
	Name       string
	CategoryID sql.NullString
	Category   *ProductCategory
	Forms      []ProductForm
}

type ProductCategory struct {
	ID   string
	Name string
}

type ProductForm struct {
	ProductID string
	ID        string
	Name      string
}

type Repo struct{ db *sql.DB }

func NewRepo(ctx context.Context, db *sql.DB) (*Repo, error) {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS product_categories (id TEXT PRIMARY KEY, name TEXT NOT NULL UNIQUE)`,
		`CREATE TABLE IF NOT EXISTS products (
			id TEXT PRIMARY KEY,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL,
			name TEXT NOT NULL,
			category_id TEXT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS product_forms (
			id TEXT PRIMARY KEY,
			product_id TEXT NOT NULL,
			name TEXT NOT NULL
		)`,
	}

	for _, stmt := range stmts {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return nil, wrapErr(fmt.Errorf("can't initialize product tables: %w", err))
		}
	}

	return &Repo{db: db}, nil
}

func (r *Repo) GetAndUpdate(
	ctx context.Context,
	productID id.ID[product.Product],
	updateFunc func(product.Product) (product.Product, error),
) (
	product.Product,
	error,
) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return product.Product{}, wrapErr(fmt.Errorf("can't start transaction: %w", err))
	}
	defer tx.Rollback()

	model, err := getByID(ctx, tx, productID)
	if err != nil {
		return product.Product{}, err
	}

	updated, err := updateFunc(model)
	if err != nil {
		return product.Product{}, err
	}

	if err = upsertProduct(ctx, tx, updated); err != nil {
		return product.Product{}, err
	}

	if err = tx.Commit(); err != nil {
		return product.Product{}, wrapErr(fmt.Errorf("can't commit transaction: %w", err))
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
	placeholders := make([]string, 0, len(idList))
	for _, v := range idList {
		args = append(args, v.String())
		placeholders = append(placeholders, "?")
	}

	query := `SELECT p.id, p.created_at, p.updated_at, p.name, p.category_id, pc.name
		FROM products p
		LEFT JOIN product_categories pc ON p.category_id = pc.id OR p.category_id = pc.name
		WHERE p.id IN (` + strings.Join(placeholders, ",") + `)`

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, wrapErr(fmt.Errorf("can't select products %v: %w", idList, err))
	}
	defer rows.Close()

	products := make([]product.Product, 0, len(idList))
	for rows.Next() {
		entity, err := scanProductBase(rows)
		if err != nil {
			return nil, err
		}
		entity.Forms, err = getForms(ctx, r.db, entity.ID)
		if err != nil {
			return nil, err
		}
		products = append(products, EntityToModel(entity))
	}

	if err = rows.Err(); err != nil {
		return nil, wrapErr(err)
	}

	return products, nil
}

func (r *Repo) Create(ctx context.Context, model product.Product) error {
	return upsertProduct(ctx, r.db, model)
}

func (r *Repo) Delete(ctx context.Context, productID id.ID[product.Product]) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM product_forms WHERE product_id = ?`, productID.String())
	if err != nil {
		return wrapErr(fmt.Errorf("can't delete product forms %s: %w", productID, err))
	}

	_, err = r.db.ExecContext(ctx, `DELETE FROM products WHERE id = ?`, productID.String())
	if err != nil {
		return wrapErr(fmt.Errorf("can't delete product %s: %w", productID, err))
	}

	return nil
}

func getByID(
	ctx context.Context,
	db interface {
		QueryRowContext(context.Context, string, ...any) *sql.Row
		QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	},
	productID id.ID[product.Product],
) (product.Product, error) {
	entity := Product{}
	var categoryName sql.NullString

	err := db.QueryRowContext(
		ctx,
		`SELECT p.id, p.created_at, p.updated_at, p.name, p.category_id, pc.name
		 FROM products p
		 LEFT JOIN product_categories pc ON p.category_id = pc.id OR p.category_id = pc.name
		 WHERE p.id = ?`,
		productID.String(),
	).Scan(&entity.ID, &entity.CreatedAt, &entity.UpdatedAt, &entity.Name, &entity.CategoryID, &categoryName)
	if err != nil {
		return product.Product{}, wrapErr(fmt.Errorf("can't get product %s: %w", productID, err))
	}

	if categoryName.Valid {
		entity.Category = &ProductCategory{ID: entity.CategoryID.String, Name: categoryName.String}
	}

	var forms []ProductForm
	forms, err = getForms(ctx, db, entity.ID)
	if err != nil {
		return product.Product{}, err
	}
	entity.Forms = forms

	return EntityToModel(entity), nil
}

func getForms(
	ctx context.Context,
	db interface {
		QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	},
	productID string,
) ([]ProductForm, error) {
	rows, err := db.QueryContext(ctx, `SELECT id, product_id, name FROM product_forms WHERE product_id = ?`, productID)
	if err != nil {
		return nil, wrapErr(fmt.Errorf("can't get forms of product %s: %w", productID, err))
	}
	defer rows.Close()

	forms := []ProductForm{}
	for rows.Next() {
		var f ProductForm
		if err = rows.Scan(&f.ID, &f.ProductID, &f.Name); err != nil {
			return nil, wrapErr(err)
		}
		forms = append(forms, f)
	}

	return forms, rows.Err()
}

func upsertProduct(
	ctx context.Context,
	db interface {
		ExecContext(context.Context, string, ...any) (sql.Result, error)
	},
	model product.Product,
) error {
	if model.Category.IsPresent() {
		categoryName := string(model.Category.OrEmpty())
		_, err := db.ExecContext(
			ctx,
			`INSERT INTO product_categories(id, name) VALUES(?, ?)
			 ON CONFLICT(name) DO UPDATE SET name=excluded.name`,
			categoryName,
			categoryName,
		)
		if err != nil {
			return wrapErr(fmt.Errorf("can't save product category: %w", err))
		}
	}

	_, err := db.ExecContext(
		ctx,
		`INSERT INTO products(id, created_at, updated_at, name, category_id)
		 VALUES(?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET
			created_at=excluded.created_at,
			updated_at=excluded.updated_at,
			name=excluded.name,
			category_id=excluded.category_id`,
		model.ID.String(),
		model.CreatedAt.Time,
		model.UpdatedAt.Time,
		string(model.Name),
		sql.NullString{String: string(model.Category.OrEmpty()), Valid: model.Category.IsPresent()},
	)
	if err != nil {
		return wrapErr(fmt.Errorf("can't save product %s: %w", model.ID, err))
	}

	_, err = db.ExecContext(ctx, `DELETE FROM product_forms WHERE product_id = ?`, model.ID.String())
	if err != nil {
		return wrapErr(fmt.Errorf("can't reset product forms %s: %w", model.ID, err))
	}

	for _, form := range model.Forms {
		_, err = db.ExecContext(
			ctx,
			`INSERT INTO product_forms(id, product_id, name) VALUES(?, ?, ?)`,
			uuid.NewString(),
			model.ID.String(),
			string(form),
		)
		if err != nil {
			return wrapErr(fmt.Errorf("can't save product form of %s: %w", model.ID, err))
		}
	}

	return nil
}

func scanProductBase(rows *sql.Rows) (Product, error) {
	entity := Product{}
	var categoryName sql.NullString
	if err := rows.Scan(
		&entity.ID,
		&entity.CreatedAt,
		&entity.UpdatedAt,
		&entity.Name,
		&entity.CategoryID,
		&categoryName,
	); err != nil {
		return Product{}, wrapErr(err)
	}

	if categoryName.Valid {
		entity.Category = &ProductCategory{ID: entity.CategoryID.String, Name: categoryName.String}
	}

	return entity, nil
}

func EntityToModel(entity Product) product.Product {
	category := mo.None[product.Category]()
	if entity.Category != nil {
		category = mo.EmptyableToOption(product.Category(entity.Category.Name))
	}

	return product.Product{
		Options: product.Options{
			Name:     product.Name(entity.Name),
			Category: category,
			Forms: lo.Map(entity.Forms, func(item ProductForm, _ int) product.Form {
				return product.Form(item.Name)
			}),
		},
		ID:        id.ID[product.Product]{UUID: god.Believe(uuid.Parse(entity.ID))},
		CreatedAt: date.CreateDate[product.Product]{Time: entity.CreatedAt},
		UpdatedAt: date.UpdateDate[product.Product]{Time: entity.UpdatedAt},
	}
}

func wrapErr(err error) error {
	if err != nil {
		return fmt.Errorf("product storage: %w", err)
	}
	return nil
}
