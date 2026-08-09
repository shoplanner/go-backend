package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/samber/lo"
	"github.com/samber/mo"

	"go-backend/internal/backend/product"
	"go-backend/internal/backend/product/repo/sqlgen"
	"go-backend/pkg/date"
	"go-backend/pkg/god"
	"go-backend/pkg/id"
	"go-backend/pkg/myerr"
)

//go:generate python $SQLC_HELPER

// LoadOptions selects which associations Load pulls in alongside the product rows.
//
// The four combinations are not a generalisation for its own sake: every one of them is a read
// path that exists today. The list repo needs category+forms for a state's product but only the
// category for its replacement, the favorites write path reads products bare, and
// product.GetByListID deliberately skips the category (see TestProductIDListOmitsCategory).
type LoadOptions struct {
	WithCategory bool
	WithForms    bool
}

type Repo struct {
	queries *sqlgen.Queries
	db      *sql.DB
}

func NewRepo(ctx context.Context, db *sql.DB) (*Repo, error) {
	queries := sqlgen.New(db)

	if err := queries.InitProductCategories(ctx); err != nil {
		return nil, wrapErr(fmt.Errorf("can't init product categories table: %w", err))
	} else if err = queries.InitProducts(ctx); err != nil {
		return nil, wrapErr(fmt.Errorf("can't init products table: %w", err))
	} else if err = queries.InitProductForms(ctx); err != nil {
		return nil, wrapErr(fmt.Errorf("can't init product forms table: %w", err))
	}

	return &Repo{queries: queries, db: db}, nil
}

func (r *Repo) GetByID(ctx context.Context, productID id.ID[product.Product]) (product.Product, error) {
	models, err := Load(ctx, r.db, []string{productID.String()}, LoadOptions{
		WithCategory: true,
		WithForms:    true,
	})
	if err != nil {
		return product.Product{}, wrapErr(fmt.Errorf("can't get product %s: %w", productID, err))
	}

	model, found := models[productID.String()]
	if !found {
		return product.Product{}, wrapErr(fmt.Errorf("%w: product %s", myerr.ErrNotFound, productID))
	}

	return model, nil
}

// GetByListID reads several products at once, with their forms but *without* their category —
// only the list and favorites read paths preload that.
func (r *Repo) GetByListID(ctx context.Context, idList []id.ID[product.Product]) ([]product.Product, error) {
	queries := sqlgen.New(r.db)

	var (
		rows []sqlgen.Product
		err  error
	)

	// An empty filter returns the whole table. That is what GORM's Find(&entities, uuids) did
	// with an empty slice, and TestProductIDListWithNoIDs_CurrentBehaviour pins it.
	if len(idList) == 0 {
		rows, err = queries.GetAllProducts(ctx)
	} else {
		rows, err = queries.GetProductsByIDList(ctx, productIDStrings(idList))
	}

	if err != nil {
		return nil, wrapErr(fmt.Errorf("can't select products %v: %w", idList, err))
	}

	models, err := assemble(ctx, queries, rows, LoadOptions{WithCategory: false, WithForms: true})
	if err != nil {
		return nil, wrapErr(fmt.Errorf("can't select products %v: %w", idList, err))
	}

	return lo.Map(rows, func(row sqlgen.Product, _ int) product.Product { return models[row.ID] }), nil
}

func (r *Repo) Create(ctx context.Context, model product.Product) error {
	log.Info().Any("model", model).Msg("inserting new product")

	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelDefault, ReadOnly: false})
	if err != nil {
		return wrapErr(fmt.Errorf("can't start transaction: %w", err))
	}

	defer func() { checkRollback(tx.Rollback()) }()

	qtx := r.queries.WithTx(tx)

	categoryID, err := resolveCategoryID(ctx, qtx, model.Category)
	if err != nil {
		return wrapErr(fmt.Errorf("can't resolve category of product %s: %w", model.ID, err))
	}

	err = qtx.InsertProduct(ctx, sqlgen.InsertProductParams{
		ID:         model.ID.String(),
		CreatedAt:  model.CreatedAt.Time,
		UpdatedAt:  model.UpdatedAt.Time,
		Name:       string(model.Name),
		CategoryID: categoryID,
	})
	if err != nil {
		return wrapErr(fmt.Errorf("can't insert product %s: %w", model.ID, err))
	}

	if err = insertForms(ctx, qtx, model.ID, model.Forms); err != nil {
		return wrapErr(fmt.Errorf("can't insert forms of product %s: %w", model.ID, err))
	}

	return wrapErr(tx.Commit())
}

func (r *Repo) GetAndUpdate(
	ctx context.Context,
	productID id.ID[product.Product],
	updateFunc func(product.Product) (product.Product, error),
) (
	product.Product,
	error,
) {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelDefault, ReadOnly: false})
	if err != nil {
		return product.Product{}, wrapErr(fmt.Errorf("can't start transaction: %w", err))
	}

	defer func() { checkRollback(tx.Rollback()) }()

	qtx := r.queries.WithTx(tx)

	row, err := qtx.GetProductByID(ctx, productID.String())
	if errors.Is(err, sql.ErrNoRows) {
		return product.Product{}, wrapErr(fmt.Errorf("%w: product %s", myerr.ErrNotFound, productID))
	} else if err != nil {
		return product.Product{}, wrapErr(fmt.Errorf("can't get product %s: %w", productID, err))
	}

	// Deliberately without category or forms: the GORM implementation read the row with a bare
	// First() here, so updateFunc has never seen them. The service replaces Options wholesale.
	model, err := updateFunc(rowToModel(row, mo.None[product.Category](), nil))
	if err != nil {
		return model, err
	}

	categoryID, err := resolveCategoryID(ctx, qtx, model.Category)
	if err != nil {
		return model, wrapErr(fmt.Errorf("can't resolve category of product %s: %w", productID, err))
	}

	if err = qtx.DeleteFormsByProductID(ctx, productID.String()); err != nil {
		return model, wrapErr(fmt.Errorf("can't clear forms of product %s: %w", productID, err))
	}

	if err = insertForms(ctx, qtx, productID, model.Forms); err != nil {
		return model, wrapErr(fmt.Errorf("can't update forms of product %s: %w", productID, err))
	}

	err = qtx.UpdateProduct(ctx, sqlgen.UpdateProductParams{
		CreatedAt:  model.CreatedAt.Time,
		UpdatedAt:  model.UpdatedAt.Time,
		Name:       string(model.Name),
		CategoryID: categoryID,
		ID:         productID.String(),
	})
	if err != nil {
		return model, wrapErr(fmt.Errorf("can't update product %s: %w", productID, err))
	}

	return model, wrapErr(tx.Commit())
}

// Load reads products by id into a map keyed by the raw id string.
//
// It takes a sqlgen.DBTX rather than a *sql.DB so the list and favorites repos can call it with
// their own *sql.Tx and read products inside their transaction — which is what GORM's nested
// Preloads used to do for them.
func Load(
	ctx context.Context,
	db sqlgen.DBTX,
	ids []string,
	opts LoadOptions,
) (
	map[string]product.Product,
	error,
) {
	if len(ids) == 0 {
		return map[string]product.Product{}, nil
	}

	queries := sqlgen.New(db)

	rows, err := queries.GetProductsByIDList(ctx, ids)
	if err != nil {
		return nil, wrapErr(fmt.Errorf("can't select products: %w", err))
	}

	return assemble(ctx, queries, rows, opts)
}

// assemble turns product rows into domain models, pulling in the associations opts asks for in
// one query each rather than per row.
func assemble(
	ctx context.Context,
	queries *sqlgen.Queries,
	rows []sqlgen.Product,
	opts LoadOptions,
) (
	map[string]product.Product,
	error,
) {
	ids := lo.Map(rows, func(row sqlgen.Product, _ int) string { return row.ID })

	formsByProduct := map[string][]product.Form{}

	if opts.WithForms && len(ids) > 0 {
		forms, err := queries.GetFormsByProductIDList(ctx, ids)
		if err != nil {
			return nil, wrapErr(fmt.Errorf("can't select product forms: %w", err))
		}

		for _, form := range forms {
			formsByProduct[form.ProductID] = append(formsByProduct[form.ProductID], product.Form(form.Name))
		}
	}

	categoryNames := map[string]string{}

	if opts.WithCategory {
		categoryIDs := lo.FilterMap(rows, func(row sqlgen.Product, _ int) (string, bool) {
			return row.CategoryID.String, row.CategoryID.Valid
		})

		if len(categoryIDs) > 0 {
			categories, err := queries.GetCategoriesByIDList(ctx, lo.Uniq(categoryIDs))
			if err != nil {
				return nil, wrapErr(fmt.Errorf("can't select product categories: %w", err))
			}

			for _, category := range categories {
				categoryNames[category.ID] = category.Name
			}
		}
	}

	models := make(map[string]product.Product, len(rows))

	for _, row := range rows {
		category := mo.None[product.Category]()
		// A category row whose name is empty reads back as "no category": the GORM mapping ran
		// the name through mo.EmptyableToOption, and the legacy dump contains such rows.
		if name, found := categoryNames[row.CategoryID.String]; row.CategoryID.Valid && found {
			category = mo.EmptyableToOption(product.Category(name))
		}

		models[row.ID] = rowToModel(row, category, formsByProduct[row.ID])
	}

	return models, nil
}

func rowToModel(row sqlgen.Product, category mo.Option[product.Category], forms []product.Form) product.Product {
	return product.Product{
		Options: product.Options{
			Name:     product.Name(row.Name),
			Category: category,
			// A nil slice must surface as an empty one, exactly as lo.Map used to make it:
			// the golden snapshots distinguish [] from null.
			Forms: lo.Map(forms, func(item product.Form, _ int) product.Form { return item }),
		},
		ID:        id.ID[product.Product]{UUID: god.Believe(uuid.Parse(row.ID))},
		CreatedAt: date.CreateDate[product.Product]{Time: row.CreatedAt},
		UpdatedAt: date.UpdateDate[product.Product]{Time: row.UpdatedAt},
	}
}

// resolveCategoryID upserts the category by name and returns the foreign key to store.
//
// products.category_id holds the category's UUID, never its name — see
// TestProductCategoryIDHoldsUUIDNotName, which exists because Product.BeforeSave looked like it
// wrote the name there and was dead code.
func resolveCategoryID(
	ctx context.Context,
	qtx *sqlgen.Queries,
	category mo.Option[product.Category],
) (
	sql.NullString,
	error,
) {
	name, present := category.Get()
	if !present {
		return sql.NullString{String: "", Valid: false}, nil
	}

	existing, err := qtx.GetCategoryByName(ctx, string(name))
	if err == nil {
		return sql.NullString{String: existing.ID, Valid: true}, nil
	}

	if !errors.Is(err, sql.ErrNoRows) {
		return sql.NullString{String: "", Valid: false}, fmt.Errorf("can't look up category %q: %w", name, err)
	}

	created := uuid.NewString()
	if err = qtx.InsertCategory(ctx, sqlgen.InsertCategoryParams{ID: created, Name: string(name)}); err != nil {
		return sql.NullString{String: "", Valid: false}, fmt.Errorf("can't insert category %q: %w", name, err)
	}

	return sql.NullString{String: created, Valid: true}, nil
}

// SQLite has no bulk-load statement, so rows go in one INSERT at a time; every caller already
// runs inside a transaction, which keeps this atomic.
func insertForms(
	ctx context.Context,
	qtx *sqlgen.Queries,
	productID id.ID[product.Product],
	forms []product.Form,
) error {
	for _, form := range forms {
		err := qtx.InsertForm(ctx, sqlgen.InsertFormParams{
			ProductID: productID.String(),
			ID:        uuid.NewString(),
			Name:      string(form),
		})
		if err != nil {
			return fmt.Errorf("can't insert form %q: %w", form, err)
		}
	}

	return nil
}

func productIDStrings(idList []id.ID[product.Product]) []string {
	return lo.Map(idList, func(item id.ID[product.Product], _ int) string { return item.String() })
}

func wrapErr(err error) error {
	if err != nil {
		return fmt.Errorf("product storage: %w", err)
	}

	return nil
}

func checkRollback(err error) {
	if wrapped := wrapErr(err); wrapped != nil && !errors.Is(err, sql.ErrTxDone) {
		log.Err(wrapped).Msg("rollback failed")
	}
}
