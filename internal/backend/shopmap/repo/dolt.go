package repo

import (
	"context"
	"fmt"
	"go-backend/internal/backend/shopmap"
	"go-backend/internal/backend/user"
	"go-backend/pkg/id"

	"github.com/uptrace/bun"
)

type ShopMapDAO struct {
	bun.BaseModel

	shopmap.ShopMap
}

type ShopMapRepo struct {
	db *bun.DB
}

// Create implements service.repo.
func (s *ShopMapRepo) Create(ctx context.Context, shopMap shopmap.ShopMap) error {
	s.db.NewInsert().DB().Table()
}

// Delete implements service.repo.
func (s *ShopMapRepo) Delete(context.Context, id.ID[shopmap.ShopMap]) (shopmap.ShopMap, error) {
	panic("unimplemented")
}

// GetAndUpdate implements service.repo.
func (s *ShopMapRepo) GetAndUpdate(ctx context.Context, id id.ID[shopmap.ShopMap], updateFunc func(context.Context, shopmap.ShopMap) (shopmap.ShopMap, error)) (shopmap.ShopMap, error) {
	panic("unimplemented")
}

// GetByID implements service.repo.
func (s *ShopMapRepo) GetByID(context.Context, id.ID[shopmap.ShopMap]) (shopmap.ShopMap, error) {
	panic("unimplemented")
}

// GetByUserID implements service.repo.
func (s *ShopMapRepo) GetByUserID(context.Context, id.ID[user.User]) ([]shopmap.ShopMap, error) {
	panic("unimplemented")
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

func (s *ShopMapRepo) update(ctx context.Context, model shopmap.ShopMap) error {
	return nil
}
