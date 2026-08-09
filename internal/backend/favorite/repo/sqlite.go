package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/samber/lo"

	"go-backend/internal/backend/favorite"
	"go-backend/internal/backend/favorite/repo/sqlgen"
	"go-backend/internal/backend/product"
	productRepo "go-backend/internal/backend/product/repo"
	"go-backend/internal/backend/user"
	"go-backend/pkg/date"
	"go-backend/pkg/god"
	"go-backend/pkg/id"
	"go-backend/pkg/myerr"
)

//go:generate python $SQLC_HELPER

// createIndexQuery duplicates the CREATE UNIQUE INDEX in schema.sql, because sqlc generates no
// query for a CREATE INDEX statement and the index has to exist before the first ON CONFLICT.
const createIndexQuery = `CREATE UNIQUE INDEX IF NOT EXISTS idx_unique_product ` +
	`ON favorite_products(product_id, favorite_list_id)`

type Repo struct {
	queries *sqlgen.Queries
	db      *sql.DB
}

func NewRepo(ctx context.Context, db *sql.DB) (*Repo, error) {
	queries := sqlgen.New(db)

	if err := queries.InitFavoriteLists(ctx); err != nil {
		return nil, wrapErr(fmt.Errorf("can't init favorite lists table: %w", err))
	} else if err = queries.InitFavoriteMembers(ctx); err != nil {
		return nil, wrapErr(fmt.Errorf("can't init favorite members table: %w", err))
	} else if err = queries.InitFavoriteProducts(ctx); err != nil {
		return nil, wrapErr(fmt.Errorf("can't init favorite products table: %w", err))
	}

	if _, err := db.ExecContext(ctx, createIndexQuery); err != nil {
		return nil, wrapErr(fmt.Errorf("can't init favorite products index: %w", err))
	}

	return &Repo{queries: queries, db: db}, nil
}

func (r *Repo) CreateList(ctx context.Context, model favorite.List) error {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelDefault, ReadOnly: false})
	if err != nil {
		return wrapErr(fmt.Errorf("can't start transaction: %w", err))
	}

	defer func() { checkRollback(tx.Rollback()) }()

	qtx := r.queries.WithTx(tx)

	err = qtx.InsertList(ctx, sqlgen.InsertListParams{
		ID:        model.ID.String(),
		ListType:  int64(model.Type),
		CreatedAt: model.CreatedAt.Time,
		UpdatedAt: model.UpdatedAt.Time,
	})
	if err != nil {
		return wrapErr(fmt.Errorf("can't create new list %s: %w", model.ID, err))
	}

	if err = insertMembers(ctx, qtx, model.ID, model.Members); err != nil {
		return wrapErr(fmt.Errorf("can't create new list %s: %w", model.ID, err))
	}

	if err = insertProducts(ctx, qtx, model.ID, model.Products); err != nil {
		return wrapErr(fmt.Errorf("can't create new list %s: %w", model.ID, err))
	}

	return wrapErr(tx.Commit())
}

func (r *Repo) GetByID(ctx context.Context, listID id.ID[favorite.List]) (favorite.List, error) {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelDefault, ReadOnly: true})
	if err != nil {
		return favorite.List{}, wrapErr(fmt.Errorf("can't start transaction: %w", err))
	}

	defer func() { checkRollback(tx.Rollback()) }()

	qtx := r.queries.WithTx(tx)

	row, err := qtx.GetListByID(ctx, listID.String())
	if errors.Is(err, sql.ErrNoRows) {
		return favorite.List{}, wrapErr(fmt.Errorf("%w: favorites list %s", myerr.ErrNotFound, listID))
	} else if err != nil {
		return favorite.List{}, wrapErr(fmt.Errorf("can't select favorites list %s: %w", listID, err))
	}

	models, err := assemble(ctx, tx, qtx, []sqlgen.FavoriteList{row}, fullProductLoad)
	if err != nil {
		return favorite.List{}, wrapErr(fmt.Errorf("can't select favorites list %s: %w", listID, err))
	}

	return models[row.ID], wrapErr(tx.Commit())
}

func (r *Repo) GetByUserID(ctx context.Context, userID id.ID[user.User]) ([]favorite.List, error) {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelDefault, ReadOnly: true})
	if err != nil {
		return nil, wrapErr(fmt.Errorf("can't start transaction: %w", err))
	}

	defer func() { checkRollback(tx.Rollback()) }()

	qtx := r.queries.WithTx(tx)

	listIDs, err := qtx.GetListIDListByUserID(ctx, userID.String())
	if err != nil {
		return nil, wrapErr(fmt.Errorf("can't get list ids of user %s: %w", userID, err))
	}

	if len(listIDs) == 0 {
		return []favorite.List{}, wrapErr(tx.Commit())
	}

	rows, err := qtx.GetListsByIDList(ctx, listIDs)
	if err != nil {
		return nil, wrapErr(fmt.Errorf("can't get lists of user %s: %w", userID, err))
	}

	models, err := assemble(ctx, tx, qtx, rows, fullProductLoad)
	if err != nil {
		return nil, wrapErr(fmt.Errorf("can't get lists of user %s: %w", userID, err))
	}

	ordered := lo.Map(rows, func(row sqlgen.FavoriteList, _ int) favorite.List { return models[row.ID] })

	return ordered, wrapErr(tx.Commit())
}

func (r *Repo) GetAndUpdate(
	ctx context.Context,
	listID id.ID[favorite.List],
	updateFunc func(favorite.List) (favorite.List, error),
) (
	favorite.List,
	error,
) {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelDefault, ReadOnly: false})
	if err != nil {
		return favorite.List{}, wrapErr(fmt.Errorf("can't start transaction: %w", err))
	}

	defer func() { checkRollback(tx.Rollback()) }()

	qtx := r.queries.WithTx(tx)

	row, err := qtx.GetListByID(ctx, listID.String())
	if errors.Is(err, sql.ErrNoRows) {
		return favorite.List{}, wrapErr(fmt.Errorf("%w: favorites list %s", myerr.ErrNotFound, listID))
	} else if err != nil {
		return favorite.List{}, wrapErr(fmt.Errorf("can't select favorites list %s: %w", listID, err))
	}

	// The write path reads its products bare — no category, no forms. That is what the GORM
	// implementation preloaded here, and the model it returns to the caller is this one rather
	// than a re-read, so filling them in would change what AddProducts hands back.
	models, err := assemble(ctx, tx, qtx, []sqlgen.FavoriteList{row}, bareProductLoad)
	if err != nil {
		return favorite.List{}, wrapErr(fmt.Errorf("can't select favorites list %s: %w", listID, err))
	}

	model, err := updateFunc(models[row.ID])
	if err != nil {
		return model, err
	}

	if err = qtx.DeleteMembersByListID(ctx, listID.String()); err != nil {
		return model, wrapErr(fmt.Errorf("can't update favorites list %s members: %w", listID, err))
	}

	if err = insertMembers(ctx, qtx, listID, model.Members); err != nil {
		return model, wrapErr(fmt.Errorf("can't update favorites list %s members: %w", listID, err))
	}

	if err = qtx.DeleteProductsByListID(ctx, listID.String()); err != nil {
		return model, wrapErr(fmt.Errorf("can't update favorites list %s products: %w", listID, err))
	}

	if err = insertProducts(ctx, qtx, listID, model.Products); err != nil {
		return model, wrapErr(fmt.Errorf("can't update favorites list %s products: %w", listID, err))
	}

	err = qtx.UpdateList(ctx, sqlgen.UpdateListParams{
		ListType:  int64(model.Type),
		CreatedAt: model.CreatedAt.Time,
		UpdatedAt: model.UpdatedAt.Time,
		ID:        listID.String(),
	})
	if err != nil {
		return model, wrapErr(fmt.Errorf("can't update favorites list %s: %w", listID, err))
	}

	return model, wrapErr(tx.Commit())
}

// The two shapes the read paths need. Reading a list to display it pulls the whole product in;
// reading it to write it back does not.
//
//nolint:gochecknoglobals // immutable option sets, spelled out once instead of at five call sites
var (
	fullProductLoad = productRepo.LoadOptions{WithCategory: true, WithForms: true}
	bareProductLoad = productRepo.LoadOptions{WithCategory: false, WithForms: false}
)

// assemble loads the members and products of every list in rows and builds the domain models.
// It runs inside the caller's transaction, including the product lookup, which is what GORM's
// nested Preloads did.
func assemble(
	ctx context.Context,
	tx *sql.Tx,
	qtx *sqlgen.Queries,
	rows []sqlgen.FavoriteList,
	opts productRepo.LoadOptions,
) (
	map[string]favorite.List,
	error,
) {
	listIDs := lo.Map(rows, func(row sqlgen.FavoriteList, _ int) string { return row.ID })

	models := make(map[string]favorite.List, len(rows))
	if len(listIDs) == 0 {
		return models, nil
	}

	memberRows, err := qtx.GetMembersByListIDList(ctx, listIDs)
	if err != nil {
		return nil, fmt.Errorf("can't select favorites members: %w", err)
	}

	productRows, err := qtx.GetProductsByListIDList(ctx, listIDs)
	if err != nil {
		return nil, fmt.Errorf("can't select favorites products: %w", err)
	}

	productIDs := lo.Uniq(lo.Map(productRows, func(row sqlgen.FavoriteProduct, _ int) string {
		return row.ProductID
	}))

	products, err := productRepo.Load(ctx, tx, productIDs, opts)
	if err != nil {
		return nil, fmt.Errorf("can't select favorited products: %w", err)
	}

	membersByList := lo.GroupBy(memberRows, func(row sqlgen.FavoriteMember) string { return row.FavoriteListID })
	productsByList := lo.GroupBy(productRows, func(row sqlgen.FavoriteProduct) string { return row.FavoriteListID })

	for _, row := range rows {
		models[row.ID] = rowToModel(row, membersByList[row.ID], productsByList[row.ID], products)
	}

	return models, nil
}

func rowToModel(
	row sqlgen.FavoriteList,
	memberRows []sqlgen.FavoriteMember,
	productRows []sqlgen.FavoriteProduct,
	products map[string]product.Product,
) favorite.List {
	model := favorite.List{
		ID:        id.ID[favorite.List]{UUID: god.Believe(uuid.Parse(row.ID))},
		CreatedAt: date.CreateDate[favorite.List]{Time: row.CreatedAt},
		UpdatedAt: date.UpdateDate[favorite.List]{Time: row.UpdatedAt},
		Members:   make([]favorite.Member, 0, len(memberRows)),
		Products:  make([]favorite.Favorite, 0, len(productRows)),
		//nolint:gosec // list_type holds a ListType enum, which has two values
		Type: favorite.ListType(row.ListType),
	}

	for _, member := range memberRows {
		model.Members = append(model.Members, favorite.Member{
			UserID: id.ID[user.User]{UUID: god.Believe(uuid.Parse(member.UserID))},
			//nolint:gosec // member_type holds a MemberType enum, which has four values
			Type:      favorite.MemberType(member.MemberType),
			CreatedAt: date.CreateDate[favorite.Member]{Time: member.CreatedAt},
			UpdatedAt: date.UpdateDate[favorite.Member]{Time: member.UpdatedAt},
		})
	}

	for _, favourited := range productRows {
		// A product row that has gone missing still yields an entry carrying its id, which is
		// what the GORM read path produced from an unresolvable association.
		item, found := products[favourited.ProductID]
		if !found {
			item = product.Product{
				Options:   product.NewZeroOptions(),
				ID:        id.ID[product.Product]{UUID: god.Believe(uuid.Parse(favourited.ProductID))},
				CreatedAt: date.CreateDate[product.Product]{Time: time.Time{}},
				UpdatedAt: date.UpdateDate[product.Product]{Time: time.Time{}},
			}
		}

		model.Products = append(model.Products, favorite.Favorite{
			Product:   item,
			CreatedAt: date.CreateDate[favorite.Favorite]{Time: favourited.CreatedAt},
			UpdatedAt: date.UpdateDate[favorite.Favorite]{Time: favourited.UpdatedAt},
		})
	}

	return model
}

// SQLite has no bulk-load statement, so rows go in one INSERT at a time; every caller already
// runs inside a transaction, which keeps this atomic.
func insertMembers(
	ctx context.Context,
	qtx *sqlgen.Queries,
	listID id.ID[favorite.List],
	members []favorite.Member,
) error {
	for _, member := range members {
		err := qtx.InsertMember(ctx, sqlgen.InsertMemberParams{
			ID:             uuid.NewString(),
			UserID:         member.UserID.String(),
			FavoriteListID: listID.String(),
			CreatedAt:      member.CreatedAt.Time,
			UpdatedAt:      member.UpdatedAt.Time,
			MemberType:     int64(member.Type),
		})
		if err != nil {
			return fmt.Errorf("can't insert member %s: %w", member.UserID, err)
		}
	}

	return nil
}

func insertProducts(
	ctx context.Context,
	qtx *sqlgen.Queries,
	listID id.ID[favorite.List],
	products []favorite.Favorite,
) error {
	for _, item := range products {
		err := qtx.InsertProduct(ctx, sqlgen.InsertProductParams{
			ID:             uuid.NewString(),
			ProductID:      item.Product.ID.String(),
			FavoriteListID: listID.String(),
			CreatedAt:      item.CreatedAt.Time,
			UpdatedAt:      item.UpdatedAt.Time,
		})
		if err != nil {
			return fmt.Errorf("can't insert product %s: %w", item.Product.ID, err)
		}
	}

	return nil
}

func wrapErr(err error) error {
	if err != nil {
		return fmt.Errorf("favorites storage: %w", err)
	}

	return nil
}

func checkRollback(err error) {
	if wrapped := wrapErr(err); wrapped != nil && !errors.Is(err, sql.ErrTxDone) {
		log.Err(wrapped).Msg("rollback failed")
	}
}
