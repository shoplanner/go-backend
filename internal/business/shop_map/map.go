package shop_map

import (
	"time"

	"github.com/google/uuid"
)

type ShopMap struct {
	ID         uuid.UUID
	CreatedAt  time.Time
	UpdatedAt  time.Time
	Categories []string
}

type Service struct {
}

func NewService() *Service {
	return nil
}
