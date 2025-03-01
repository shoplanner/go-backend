package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/samber/lo"

	"go-backend/internal/backend/product"
	"go-backend/internal/backend/shopmap"
	"go-backend/internal/backend/shopmap/repo/sqlgen"
	"go-backend/internal/backend/user"
	"go-backend/pkg/date"
	"go-backend/pkg/god"
	"go-backend/pkg/id"
)

//go:generate python $SQLC_HELPER

type ShopMapRepo struct {
	queries *sqlgen.Queries
	db      *sql.DB
}

func NewShopMapRepo(ctx context.Context, db *sql.DB) (*ShopMapRepo, error) {
	queries := sqlgen.New(db)

	if err := queries.InitShopMaps(ctx); err != nil {
		return nil, wrapErr(fmt.Errorf("can't init shop maps table: %w", err))
	} else if err = queries.InitShopMapCategories(ctx); err != nil {
		return nil, wrapErr(fmt.Errorf("can't init shop map categories: %w", err))
	} else if err = queries.InitShopMapViewers(ctx); err != nil {
		return nil, wrapErr(fmt.Errorf("can't init shop map viewers: %w", err))
	}

	return &ShopMapRepo{queries: queries, db: db}, nil
}

// Create implements service.repo.
func (s *ShopMapRepo) Create(ctx context.Context, model shopmap.ShopMap) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("DoltDB: can't start transaction: %w", err)
	}

	defer func() { checkRollback(tx.Rollback()) }()

	qtx := s.queries.WithTx(tx)

	err = qtx.CreateShopMap(ctx, sqlgen.CreateShopMapParams{
		ID:        model.ID.String(),
		OwnerID:   model.OwnerID.String(),
		CreatedAt: model.CreatedAt.Time,
		UpdatedAt: model.UpdatedAt.Time,
	})
	if err != nil {
		return fmt.Errorf("can't insert shop map to DoltDB: %w", err)
	}

	if len(model.ViewerIDList) != 0 {
		_, err = qtx.InsertViewers(
			ctx,
			lo.Map(model.ViewerIDList, func(userID id.ID[user.User], _ int) sqlgen.InsertViewersParams {
				return sqlgen.InsertViewersParams{
					MapID:  model.ID.String(),
					UserID: userID.String(),
				}
			}))
		if err != nil {
			return fmt.Errorf("can't insert viewers for shop map %s: %w", model.ID, err)
		}
	}

	if len(model.CategoryList) != 0 {
		_, err = qtx.InsertCategories(
			ctx,
			lo.Map(model.CategoryList, func(category product.Category, index int) sqlgen.InsertCategoriesParams {
				return sqlgen.InsertCategoriesParams{
					MapID:    model.ID.String(),
					Number:   uint32(index), //nolint:gosec // slice index can't be negative
					Category: string(category),
				}
			}))
		if err != nil {
			return wrapErr(fmt.Errorf("can't insert categories of shop map %s: %w", model.ID, err))
		}
	}

	return wrapErr(tx.Commit())
}

func (s *ShopMapRepo) Delete(ctx context.Context, mapID id.ID[shopmap.ShopMap]) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("can't start DoltDB transaction: %w", err)
	}
	defer func() { checkRollback(tx.Rollback()) }()

	qtx := s.queries.WithTx(tx)

	if err = qtx.DeleteCategoriesByMapID(ctx, mapID.String()); err != nil {
		return fmt.Errorf("can't delete shop map %s categories: %w", mapID, err)
	}
	if err = qtx.DeleteViewers(ctx, mapID.String()); err != nil {
		return fmt.Errorf("can't delete shop map %s viewers: %w", mapID, err)
	}
	if err = qtx.DeleteShopMap(ctx, mapID.String()); err != nil {
		return fmt.Errorf("can't delete shop map %s: %w", mapID, err)
	}

	return wrapErr(tx.Commit())
}

func (s *ShopMapRepo) GetAndUpdate(
	ctx context.Context,
	mapID id.ID[shopmap.ShopMap],
	updateFunc func(shopmap.ShopMap) (shopmap.ShopMap, error),
) (shopmap.ShopMap, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return shopmap.ShopMap{}, fmt.Errorf("can't start transaction: %w", err)
	}
	defer func() { checkRollback(tx.Rollback()) }()

	qtx := s.queries.WithTx(tx)

	oldModel, err := s.getByID(ctx, qtx, mapID)
	if err != nil {
		return oldModel, err
	}

	model, err := updateFunc(oldModel)
	if err != nil {
		return model, err
	}

	if err = update(ctx, qtx, model, oldModel); err != nil {
		return model, err
	}

	return model, wrapErr(tx.Commit())
}

// GetByID implements service.repo.
func (s *ShopMapRepo) GetByID(ctx context.Context, mapID id.ID[shopmap.ShopMap]) (shopmap.ShopMap, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return shopmap.ShopMap{}, fmt.Errorf("can't start DoltDB transaction: %w", err)
	}
	defer func() { checkRollback(tx.Rollback()) }()

	qtx := s.queries.WithTx(tx)

	model, err := s.getByID(ctx, qtx, mapID)
	if err != nil {
		return model, err
	}

	return model, wrapErr(tx.Commit())
}

// GetByUserID implements service.repo.
func (s *ShopMapRepo) GetByUserID(ctx context.Context, userID id.ID[user.User]) ([]shopmap.ShopMap, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("can't start transaction: %w", err)
	}

	defer func() { checkRollback(tx.Rollback()) }()

	qtx := s.queries.WithTx(tx)

	viewerMapIDList, err := qtx.GetMapsWithViewer(ctx, userID.String())
	if err != nil {
		return nil, fmt.Errorf("can't get maps of viewer %s: %w", userID, err)
	}

	shopMapList, err := qtx.GetByListID(ctx, viewerMapIDList)
	if err != nil {
		return nil, fmt.Errorf("can't get shop maps of user %s: %w", userID, err)
	}

	ownerMapList, err := qtx.GetByOwnerID(ctx, userID.String())
	if err != nil {
		return nil, fmt.Errorf("can't get owner maps of user %s: %w", userID, err)
	}

	shopMapList = append(shopMapList, ownerMapList...)

	mapListID := lo.Map(shopMapList, func(item sqlgen.ShopMap, _ int) string { return item.ID })

	categoryListDAO, err := qtx.GetCategoriesByListID(ctx, mapListID)
	if err != nil {
		return nil, fmt.Errorf("can't get shop map categories of user %s: %w", userID, err)
	}

	viewerListDAO, err := qtx.GetViewersByListID(ctx, mapListID)
	if err != nil {
		return nil, fmt.Errorf("can't get viewers of shop maps user %s: %w", userID, err)
	}

	idMapToCategories := make(map[string][]sqlgen.ShopMapCategory, len(mapListID))
	idMapToViewers := make(map[string][]sqlgen.ShopMapViewer, len(mapListID))

	for _, category := range categoryListDAO {
		idMapToCategories[category.MapID] = append(idMapToCategories[category.MapID], category)
	}

	for _, viewer := range viewerListDAO {
		idMapToViewers[viewer.MapID] = append(idMapToViewers[viewer.MapID], viewer)
	}

	models := make([]shopmap.ShopMap, 0, len(mapListID))
	for _, shopMap := range shopMapList {
		models = append(models, entityToModel(shopMap, idMapToCategories[shopMap.ID], idMapToViewers[shopMap.ID]))
	}

	return models, wrapErr(tx.Commit())
}

func (s *ShopMapRepo) getByID(
	ctx context.Context,
	qtx *sqlgen.Queries,
	mapID id.ID[shopmap.ShopMap],
) (shopmap.ShopMap, error) {
	shopMap, err := qtx.GetByID(ctx, mapID.String())
	if err != nil {
		return shopmap.ShopMap{}, fmt.Errorf("can't get shop map %s: %w", mapID, err)
	}

	categories, err := qtx.GetCategoriesByID(ctx, mapID.String())
	if err != nil {
		return shopmap.ShopMap{}, fmt.Errorf("can't get shop map %s categories: %w", mapID, err)
	}

	viewers, err := qtx.GetViewersByMapID(ctx, mapID.String())
	if err != nil {
		return shopmap.ShopMap{}, fmt.Errorf("can't get shop map %s viewers: %w", mapID, err)
	}

	return entityToModel(shopMap, categories, viewers), nil
}

func update(ctx context.Context, qtx *sqlgen.Queries, newModel shopmap.ShopMap, oldModel shopmap.ShopMap) error {
	err := qtx.UpdateShopMap(ctx, sqlgen.UpdateShopMapParams{
		OwnerID:   newModel.OwnerID.String(),
		UpdatedAt: newModel.UpdatedAt.Time,
		ID:        newModel.ID.String(),
	})
	if err != nil {
		return fmt.Errorf("can't update shop map %s: %w", newModel.ID, err)
	}

	// remove extra categories, then updates leaving
	if len(newModel.CategoryList) < len(oldModel.CategoryList) {
		if err = qtx.DeleteCategoriesAfterIndex(ctx, sqlgen.DeleteCategoriesAfterIndexParams{
			MapID:  newModel.ID.String(),
			Number: uint32(len(newModel.CategoryList)), //nolint:gosec // index can't be negative
		}); err != nil {
			return fmt.Errorf("doltdb: can't delete old categories %s: %w", newModel.ID, err)
		}
	} else if len(newModel.CategoryList) > len(oldModel.CategoryList) {
		_, err = qtx.InsertCategories(
			ctx,
			lo.Map(
				newModel.CategoryList[len(oldModel.CategoryList):],
				func(item product.Category, index int) sqlgen.InsertCategoriesParams {
					return sqlgen.InsertCategoriesParams{
						MapID:    newModel.ID.String(),
						Number:   uint32(index), //nolint:gosec // index can't be negative
						Category: string(item),
					}
				}))
		if err != nil {
			return fmt.Errorf("can't insert new categories in shop map %s: %w", newModel.ID, err)
		}
	}

	// update changed categories
	for i := range min(len(newModel.CategoryList), len(oldModel.CategoryList)) {
		if newModel.CategoryList[i] == oldModel.CategoryList[i] {
			continue
		}

		params := sqlgen.UpdateCategoriesParams{
			Number:   uint32(i), //nolint:gosec //index can't be negative
			MapID:    newModel.ID.String(),
			Category: string(newModel.CategoryList[i]),
		}

		err = qtx.UpdateCategories(ctx, params)
		if err != nil {
			return fmt.Errorf("can't update categories of shop map %s: %w", newModel.ID, err)
		}
	}

	added, deleted := lo.Difference(newModel.ViewerIDList, oldModel.ViewerIDList)
	if len(added) != 0 {
		_, err = qtx.InsertViewers(ctx, lo.Map(added, func(userID id.ID[user.User], _ int) sqlgen.InsertViewersParams {
			return sqlgen.InsertViewersParams{
				MapID:  newModel.ID.String(),
				UserID: userID.String(),
			}
		}))
		if err != nil {
			return wrapErr(fmt.Errorf("can't add new viewers to shop map %s: %w", newModel.ID, err))
		}
	}

	if len(deleted) != 0 {
		err = qtx.DeleteViewersByListID(ctx, lo.Map(deleted, func(userID id.ID[user.User], _ int) string {
			return userID.String()
		}))
		if err != nil {
			return wrapErr(fmt.Errorf("can't delete viewers from shop map %s: %w", newModel.ID, err))
		}
	}

	return nil
}

func entityToModel(
	shopMap sqlgen.ShopMap,
	categories []sqlgen.ShopMapCategory,
	viewers []sqlgen.ShopMapViewer,
) shopmap.ShopMap {
	model := shopmap.ShopMap{
		Options: shopmap.Options{
			CategoryList: make([]product.Category, len(categories)),
			ViewerIDList: lo.Map(viewers, func(item sqlgen.ShopMapViewer, _ int) id.ID[user.User] {
				return id.ID[user.User]{UUID: god.Believe(uuid.Parse(item.UserID))}
			}),
		},
		ID:        id.ID[shopmap.ShopMap]{UUID: god.Believe(uuid.Parse(shopMap.ID))},
		OwnerID:   id.ID[user.User]{UUID: god.Believe(uuid.Parse(shopMap.OwnerID))},
		CreatedAt: date.CreateDate[shopmap.ShopMap]{Time: shopMap.CreatedAt},
		UpdatedAt: date.UpdateDate[shopmap.ShopMap]{Time: shopMap.UpdatedAt},
	}

	for _, categoryDao := range categories {
		model.CategoryList[categoryDao.Number] = product.Category(categoryDao.Category)
	}

	return model
}

func wrapErr(err error) error {
	if err != nil {
		return fmt.Errorf("shopmap DoltDB repo: %w", err)
	}

	return nil
}

func checkRollback(err error) {
	if wrapped := wrapErr(err); wrapped != nil && !errors.Is(err, sql.ErrTxDone) {
		log.Err(wrapped).Msg("rollback failed")
	}
}
