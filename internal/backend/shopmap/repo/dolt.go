package repo

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/samber/lo"
	"github.com/uptrace/bun"

	"go-backend/internal/backend/product"
	"go-backend/internal/backend/shopmap"
	"go-backend/internal/backend/shopmap/repo/sqlgen"
	"go-backend/internal/backend/user"
	"go-backend/pkg/date"
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
}

// GetByID implements service.repo.
func (s *ShopMapRepo) GetByID(ctx context.Context, mapID id.ID[shopmap.ShopMap]) (shopmap.ShopMap, error) {
}

// GetByUserID implements service.repo.
func (s *ShopMapRepo) GetByUserID(ctx context.Context, userID id.ID[user.User]) ([]shopmap.ShopMap, error) {
}

func entityToModelIdx(dao sqlgen.ShopMap, _ int) shopmap.ShopMap {
	return shopmap.ShopMap{
		Options: shopmap.Options{
			CategoryList: lo.Map(dao.CategoryList, func(item CategoryDAO, index int) product.Category {
				return product.Category(item.Category)
			}),
			ViewerIDList: lo.Map(dao.ViewerList, func(item ShopMapViewer, _ int) id.ID[user.User] {
				return id.ID[user.User]{UUID: item.UserID}
			}),
		},
		ID:        id.ID[shopmap.ShopMap]{UUID: dao.ID},
		OwnerID:   id.ID[user.User]{UUID: dao.OwnerID},
		CreatedAt: date.CreateDate[shopmap.ShopMap]{Time: dao.CreatedAt},
		UpdatedAt: date.UpdateDate[shopmap.ShopMap]{Time: dao.UpdatedAt},
	}
}

func modelToEntityIdx(model shopmap.ShopMap, _ int) ShopMapDAO {
	return ShopMapDAO{
		BaseModel: bun.BaseModel{},
		ID:        model.ID.UUID,
		OwnerID:   model.OwnerID.UUID,
		CreatedAt: model.CreatedAt.Time,
		UpdatedAt: model.UpdatedAt.Time,
		CategoryList: lo.Map(model.CategoryList, func(item product.Category, index int) CategoryDAO {
			return CategoryDAO{
				BaseModel: bun.BaseModel{},
				MapID:     model.ID.UUID,
				Category:  string(item),
			}
		}),
		ViewerList: lo.Map(model.ViewerIDList, func(item id.ID[user.User], _ int) ShopMapViewer {
			return ShopMapViewer{
				UserID: item.UUID,
				MapID:  model.ID.UUID,
			}
		}),
	}
}

func modelToEntity(model shopmap.ShopMap) ShopMapDAO {
	return modelToEntityIdx(model, 0)
}

func entityToModel(dao ShopMapDAO) shopmap.ShopMap {
	return entityToModelIdx(dao, 0)
}
