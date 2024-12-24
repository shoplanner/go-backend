package repo

import (
	"context"
	"go-backend/internal/backend/shopmap"

	"github.com/uptrace/bun"
)

type ShopMapRepo struct {
	db *bun.DB
}

func NewShopMapRepo(db *bun.DB) *ShopMapRepo {
	return &ShopMapRepo{
		db: db,
	}
}


func (s *ShopMapRepo) update(ctx context.Context, model shopmap.ShopMap) error {
    
}
