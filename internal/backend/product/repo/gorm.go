package repo

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/samber/lo"
	"github.com/samber/mo"

	"go-backend/internal/backend/product"
	"go-backend/internal/backend/product/repo/sqlgen"
	"go-backend/pkg/date"
	"go-backend/pkg/god"
	"go-backend/pkg/id"
)

//go:generate python $SQLC_HELPER

type Product struct {
	ID         string
	CreatedAt  date.CreateDate[product.Product]
	UpdatedAt  date.UpdateDate[product.Product]
	Name       product.Name
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

type Repo struct {
	queries *sqlgen.Queries
	db      *sql.DB
}

func NewRepo(ctx context.Context, db *sql.DB) (*Repo, error) {
	q := sqlgen.New(db)

	if err := q.InitProductCategories(ctx); err != nil {
		return nil, wrapErr(fmt.Errorf("can't initialize product categories table: %w", err))
	}
	if err := q.InitProducts(ctx); err != nil {
		return nil, wrapErr(fmt.Errorf("can't initialize products table: %w", err))
	}
	if err := q.InitProductForms(ctx); err != nil {
		return nil, wrapErr(fmt.Errorf("can't initialize product forms table: %w", err))
	}

	return &Repo{queries: q, db: db}, nil
}

func (r *Repo) GetAndUpdate(
	ctx context.Context,
	productID id.ID[product.Product],
	updateFunc func(product.Product) (product.Product, error),
) (product.Product, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return product.Product{}, wrapErr(fmt.Errorf("can't start transaction: %w", err))
	}
	defer tx.Rollback()

	qtx := r.queries.WithTx(tx)
	model, err := getByID(ctx, qtx, productID)
	if err != nil {
		return product.Product{}, err
	}

	updated, err := updateFunc(model)
	if err != nil {
		return product.Product{}, err
	}
	if err = upsertProduct(ctx, qtx, updated); err != nil {
		return product.Product{}, err
	}

	if err = tx.Commit(); err != nil {
		return product.Product{}, wrapErr(fmt.Errorf("can't commit transaction: %w", err))
	}
	return updated, nil
}

func (r *Repo) GetByID(ctx context.Context, productID id.ID[product.Product]) (product.Product, error) {
	return getByID(ctx, r.queries, productID)
}

func (r *Repo) GetByListID(ctx context.Context, idList []id.ID[product.Product]) ([]product.Product, error) {
	if len(idList) == 0 {
		return []product.Product{}, nil
	}

	ids := lo.Map(idList, func(item id.ID[product.Product], _ int) string { return item.String() })
	models, err := r.queries.GetProductsByListID(ctx, ids)
	if err != nil {
		return nil, wrapErr(fmt.Errorf("can't select products %v: %w", idList, err))
	}

	forms, err := r.queries.GetFormsByProductListID(ctx, ids)
	if err != nil {
		return nil, wrapErr(fmt.Errorf("can't select forms of products %v: %w", idList, err))
	}

	formsMap := map[string][]ProductForm{}
	for _, f := range forms {
		formsMap[f.ProductID] = append(formsMap[f.ProductID], ProductForm{ID: f.ID, ProductID: f.ProductID, Name: f.Name})
	}

	return lo.Map(models, func(item sqlgen.GetProductsByListIDRow, _ int) product.Product {
		entity := Product{
			ID:         item.ID,
			Name:       product.Name(item.Name),
			CreatedAt:  date.CreateDate[product.Product]{Time: item.CreatedAt},
			UpdatedAt:  date.UpdateDate[product.Product]{Time: item.UpdatedAt},
			CategoryID: item.CategoryID,
			Forms:      formsMap[item.ID],
		}
		if item.CategoryName.Valid {
			entity.Category = &ProductCategory{ID: item.CategoryID.String, Name: item.CategoryName.String}
		}
		return EntityToModel(entity)
	}), nil
}

func (r *Repo) Create(ctx context.Context, model product.Product) error {
	return upsertProduct(ctx, r.queries, model)
}

func (r *Repo) Delete(ctx context.Context, productID id.ID[product.Product]) error {
	if err := r.queries.DeleteFormsByProductID(ctx, productID.String()); err != nil {
		return wrapErr(fmt.Errorf("can't delete product forms %s: %w", productID, err))
	}
	if err := r.queries.DeleteProductByID(ctx, productID.String()); err != nil {
		return wrapErr(fmt.Errorf("can't delete product %s: %w", productID, err))
	}
	return nil
}

func getByID(
	ctx context.Context,
	q interface {
		GetProductByID(context.Context, string) (sqlgen.GetProductByIDRow, error)
		GetFormsByProductID(context.Context, string) ([]sqlgen.ProductForm, error)
	},
	productID id.ID[product.Product],
) (product.Product, error) {
	item, err := q.GetProductByID(ctx, productID.String())
	if err != nil {
		return product.Product{}, wrapErr(fmt.Errorf("can't get product %s: %w", productID, err))
	}

	forms, err := q.GetFormsByProductID(ctx, productID.String())
	if err != nil {
		return product.Product{}, wrapErr(fmt.Errorf("can't get forms of product %s: %w", productID, err))
	}

	entity := Product{
		ID:         item.ID,
		Name:       product.Name(item.Name),
		CreatedAt:  date.CreateDate[product.Product]{Time: item.CreatedAt},
		UpdatedAt:  date.UpdateDate[product.Product]{Time: item.UpdatedAt},
		CategoryID: item.CategoryID,
		Forms: lo.Map(forms, func(f sqlgen.ProductForm, _ int) ProductForm {
			return ProductForm{ID: f.ID, ProductID: f.ProductID, Name: f.Name}
		}),
	}
	if item.CategoryName.Valid {
		entity.Category = &ProductCategory{ID: item.CategoryID.String, Name: item.CategoryName.String}
	}

	return EntityToModel(entity), nil
}

func upsertProduct(
	ctx context.Context,
	q interface {
		UpsertCategory(context.Context, sqlgen.UpsertCategoryParams) error
		UpsertProduct(context.Context, sqlgen.UpsertProductParams) error
		DeleteFormsByProductID(context.Context, string) error
		InsertProductForm(context.Context, sqlgen.InsertProductFormParams) error
	},
	model product.Product,
) error {
	if model.Category.IsPresent() {
		categoryName := string(model.Category.OrEmpty())
		if err := q.UpsertCategory(ctx, sqlgen.UpsertCategoryParams{ID: categoryName, Name: categoryName}); err != nil {
			return wrapErr(fmt.Errorf("can't save product category: %w", err))
		}
	}

	if err := q.UpsertProduct(ctx, sqlgen.UpsertProductParams{
		ID:         model.ID.String(),
		CreatedAt:  model.CreatedAt.Time,
		UpdatedAt:  model.UpdatedAt.Time,
		Name:       string(model.Name),
		CategoryID: sql.NullString{String: string(model.Category.OrEmpty()), Valid: model.Category.IsPresent()},
	}); err != nil {
		return wrapErr(fmt.Errorf("can't save product %s: %w", model.ID, err))
	}

	if err := q.DeleteFormsByProductID(ctx, model.ID.String()); err != nil {
		return wrapErr(fmt.Errorf("can't reset product forms %s: %w", model.ID, err))
	}

	for _, form := range model.Forms {
		if err := q.InsertProductForm(ctx, sqlgen.InsertProductFormParams{
			ID:        uuid.NewString(),
			ProductID: model.ID.String(),
			Name:      string(form),
		}); err != nil {
			return wrapErr(fmt.Errorf("can't save product form of %s: %w", model.ID, err))
		}
	}

	return nil
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
		CreatedAt: entity.CreatedAt,
		UpdatedAt: entity.UpdatedAt,
	}
}

func wrapErr(err error) error {
	if err != nil {
		return fmt.Errorf("product storage: %w", err)
	}
	return nil
}
