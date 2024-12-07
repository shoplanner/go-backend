package models

import (
	"time"

	"github.com/google/uuid"

	productModel "go-backend/internal/product/models"
)

type ShopMap struct {
	ID         uuid.UUID               `json:"id" bson:"_id"`
	OwnerID    uuid.UUID               `json:"user_id" bson:"user_id" validate:"user_id_exist"`
	ViewersID  []uuid.UUID             `json:"viewers_id" bson:"viewers_id" validate:"unique,dive,user_id_exist"`
	Categories []productModel.Category `json:"categories" bson:"categories" validate:"dive,unique"`
	CreatedAt  time.Time               `json:"created_at" bson:"created_at"`
	UpdatedAt  time.Time               `json:"updated_at" bson:"updated_at"`
}

func NewShopMap() ShopMap {
	return ShopMap{
		ID:        uuid.New(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}
