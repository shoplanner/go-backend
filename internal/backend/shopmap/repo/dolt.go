package repo

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/samber/lo"
	"github.com/uptrace/bun"

	"go-backend/internal/backend/shopmap"
	"go-backend/internal/backend/user"
	"go-backend/pkg/id"
)

type ShopMapDAO struct {
	bun.BaseModel

	shopmap.ShopMap
}

type ShopMapRepo struct {
	db *bun.DB
}

func NewShopMapRepo(ctx context.Context, db *bun.DB) (*ShopMapRepo, error) {
	_, err := db.NewCreateTable().Model((*ShopMapDAO)(nil)).IfNotExists().Exec(ctx)
	if err != nil {
		return nil, fmt.Errorf("can't create shop map table: %w", err)
	}
	return &ShopMapRepo{
		db: db,
	}, nil
}

// Create implements service.repo.
func (s *ShopMapRepo) Create(ctx context.Context, shopMap shopmap.ShopMap) error {
	_, err := s.db.NewInsert().Model(shopMap).Exec(ctx)
	if err != nil {
		return fmt.Errorf("can't insert new shop map %s to DoltDB: %w", shopMap.ID, err)
	}
	return nil
}

// Delete implements service.repo.
func (s *ShopMapRepo) Delete(ctx context.Context, mapID id.ID[shopmap.ShopMap]) (shopmap.ShopMap, error) {
	var model ShopMapDAO
	err := s.db.NewDelete().Model(ShopMapDAO{ShopMap: shopmap.ShopMap{ID: mapID}}).Returning("*").Scan(ctx, &model)
	if err != nil {
		return model.ShopMap, fmt.Errorf("can't delete shop map %s from DoltDB: %w", mapID, err)
	}
	return model.ShopMap, nil
}

// GetAndUpdate implements service.repo.
func (s *ShopMapRepo) GetAndUpdate(ctx context.Context, mapID id.ID[shopmap.ShopMap], updateFunc func(shopmap.ShopMap) shopmap.ShopMap) (shopmap.ShopMap, error) {
	var shopMap ShopMapDAO
	err := s.db.RunInTx(ctx, &sql.TxOptions{Isolation: sql.IsolationLevel(0)}, func(ctx context.Context, tx bun.Tx) error {
		err := tx.NewSelect().Model(&shopMap).Where("id = ?", mapID).Scan(ctx)
		if err != nil {
			return fmt.Errorf("can't get shop map %s from DoltDB: %w", mapID, err)
		}

		shopMap.ShopMap = updateFunc(shopMap.ShopMap)

		_, err = s.db.NewUpdate().Model(shopMap).Exec(ctx)
		if err != nil {
			return fmt.Errorf("can't update shop map %s in DoltDB: %w", mapID, err)
		}
		return nil
	})
	if err != nil {
		return shopMap.ShopMap, fmt.Errorf("DoltDB transaction failed: %w", err)
	}

	return shopMap.ShopMap, nil
}

// GetByID implements service.repo.
func (s *ShopMapRepo) GetByID(ctx context.Context, mapID id.ID[shopmap.ShopMap]) (shopmap.ShopMap, error) {
	var model ShopMapDAO
	err := s.db.NewSelect().Model(&model).Where("id = ?", mapID).Scan(ctx)
	if err != nil {
		return model.ShopMap, fmt.Errorf("can't get shop map %s from DoltDB: %w", mapID, err)
	}

	return model.ShopMap, nil
}

// GetByUserID implements service.repo.
func (s *ShopMapRepo) GetByUserID(ctx context.Context, userID id.ID[user.User]) ([]shopmap.ShopMap, error) {
	var daoList []ShopMapDAO
	err := s.db.NewSelect().Model(&daoList).Scan(ctx)

	models := lo.Map(daoList, func(item ShopMapDAO, index int) shopmap.ShopMap {
		return item.ShopMap
	})
	if err != nil {
		return models, fmt.Errorf("can't get shop map of user %s from DoltDB: %w", userID, err)
	}

	return models, nil
}
