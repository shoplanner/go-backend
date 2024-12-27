package repo

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/samber/lo"

	"go-backend/internal/backend/product"
	"go-backend/internal/backend/shopmap"
	"go-backend/internal/backend/shopmap/repo/sqlgen"
	"go-backend/internal/backend/user"
	"go-backend/pkg/date"
	"go-backend/pkg/god"
	"go-backend/pkg/id"
)

//go:generate $SQLC_HELPER

type ShopMapRepo struct {
	q  *sqlgen.Queries
	db *sql.DB
}

func NewShopMapRepo(ctx context.Context, db *sql.DB) (*ShopMapRepo, error) {
	return &ShopMapRepo{q: sqlgen.New(db), db: db}, nil
}

// Create implements service.repo.
func (s *ShopMapRepo) Create(ctx context.Context, shopMap shopmap.ShopMap) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("DoltDB: can't start transaction: %w", err)
	}
	defer tx.Rollback()

	qtx := s.q.WithTx(tx)

	err = qtx.CreateShopMap(ctx, sqlgen.CreateShopMapParams{
		ID:        shopMap.ID.String(),
		OwnerID:   shopMap.OwnerID.String(),
		CreatedAt: shopMap.CreatedAt.Time,
		UpdatedAt: shopMap.UpdatedAt.Time,
	})
	if err != nil {
		return fmt.Errorf("can't insert shop map to DoltDB", err)
	}

	_, err = qtx.InsertViewers(ctx, lo.Map(shopMap.ViewerIDList, func(userID id.ID[user.User], index int) sqlgen.InsertViewersParams {
		return sqlgen.InsertViewersParams{
			MapID:  shopMap.ID.String(),
			UserID: userID.String(),
		}
	}))
	_, err = qtx.InsertCategories(ctx, lo.Map(shopMap.CategoryList, func(category product.Category, index int) sqlgen.InsertCategoriesParams {
		return sqlgen.InsertCategoriesParams{
			MapID:    shopMap.ID.String(),
			Category: string(category),
		}
	}))

	return tx.Commit()
}

// Delete implements service.repo.
func (s *ShopMapRepo) Delete(ctx context.Context, mapID id.ID[shopmap.ShopMap]) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("can't start DoltDB transaction: %w", err)
	}
	defer tx.Rollback()

	qtx := s.q.WithTx(tx)

	if err := qtx.DeleteCategories(ctx, mapID.String()); err != nil {
		return fmt.Errorf("can't delete shop map %s categories: %w", mapID, err)
	}
	if err := qtx.DeleteViewers(ctx, mapID.String()); err != nil {
		return fmt.Errorf("can't delete shop map %s viewers: %w", mapID, err)
	}
	if err := qtx.DeleteShopMap(ctx, mapID.String()); err != nil {
		return fmt.Errorf("can't delete shop map %s: %w", mapID, err)
	}

	return tx.Commit()
}

// GetAndUpdate implements service.repo.
func (s *ShopMapRepo) GetAndUpdate(
	ctx context.Context,
	mapID id.ID[shopmap.ShopMap],
	updateFunc func(shopmap.ShopMap) (shopmap.ShopMap, error),
) (shopmap.ShopMap, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return shopmap.ShopMap{}, fmt.Errorf("can't start transaction: %w", err)
	}
	defer tx.Rollback()

	qtx := s.q.WithTx(tx)

	model, err := s.getById(ctx, qtx, mapID)
	if err != nil {
		return model, err
	}

	model, err = updateFunc(model)
	if err != nil {
		return model, err
	}
}

// GetByID implements service.repo.
func (s *ShopMapRepo) GetByID(ctx context.Context, mapID id.ID[shopmap.ShopMap]) (shopmap.ShopMap, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return shopmap.ShopMap{}, fmt.Errorf("can't start DoltDB transaction: %w", err)
	}
	defer tx.Rollback()

	qtx := s.q.WithTx(tx)

	model, err := s.getById(ctx, qtx, mapID)
	if err != nil {
		return model, err
	}

	return model, tx.Commit()
}

// GetByUserID implements service.repo.
func (s *ShopMapRepo) GetByUserID(ctx context.Context, userID id.ID[user.User]) ([]shopmap.ShopMap, error) {
}

func (s *ShopMapRepo) getById(ctx context.Context, qtx *sqlgen.Queries, mapID id.ID[shopmap.ShopMap]) (shopmap.ShopMap, error) {
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
		return shopmap.ShopMap{}, fmt.Errorf("can't get shop map %s viewers", mapID, err)
	}

	return entityToModel(shopMap, categories, viewers), nil
}

func (s *ShopMapRepo) update(ctx context.Context, qtx *sqlgen.Queries, model shopmap.ShopMap, oldModel shopmap.ShopMap) error {
	err := qtx.UpdateShopMap(ctx, sqlgen.UpdateShopMapParams{
		OwnerID:   model.OwnerID.String(),
		UpdatedAt: model.UpdatedAt.Time,
		CreatedAt: model.CreatedAt.Time,
		ID:        model.ID.String(),
	})
	if err != nil {
		return fmt.Errorf("can't update shop map %s: %w", model.ID, err)
	}


}

func entityToModel(shopMap sqlgen.ShopMap, categories []sqlgen.GetCategoriesByIDRow, viewers []string) shopmap.ShopMap {
	model := shopmap.ShopMap{
		Options: shopmap.Options{
			CategoryList: make([]product.Category, len(categories)),
			ViewerIDList: lo.Map(viewers, func(item string, _ int) id.ID[user.User] {
				return id.ID[user.User]{UUID: god.Believe(uuid.Parse(item))}
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

func modelToEntity(model shopmap.ShopMap) (sqlgen.ShopMap, []sqlgen.ShopMapCategory, []sqlgen.ShopMapViewer) {
	return sqlgen.ShopMap{
			ID:        model.ID.UUID.String(),
			OwnerID:   model.OwnerID.UUID.String(),
			CreatedAt: model.CreatedAt.Time,
			UpdatedAt: model.UpdatedAt.Time,
		},
		lo.Map(model.CategoryList, func(item product.Category, index int) sqlgen.ShopMapCategory {
			return sqlgen.ShopMapCategory{
				MapID:    model.ID.UUID.String(),
				Number:   uint32(index),
				Category: string(item),
			}
		}),
		lo.Map(model.ViewerIDList, func(item id.ID[user.User], _ int) sqlgen.ShopMapViewer {
			return sqlgen.ShopMapViewer{
				UserID: item.UUID.String(),
				MapID:  model.ID.UUID.String(),
			}
		})
}
