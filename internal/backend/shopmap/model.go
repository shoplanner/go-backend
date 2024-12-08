package shopmap

import (
	"time"

	"github.com/google/uuid"

	"go-backend/internal/backend/product"
	"go-backend/internal/backend/user"
	"go-backend/pkg/date"
	"go-backend/pkg/id"
)

type ShopMap struct {
	ID         id.ID[ShopMap]           `json:"id" bson:"_id"`
	OwnerID    id.ID[user.User]         `json:"user_id" bson:"user_id" validate:"user_id_exist"`
	ViewersID  []id.ID[user.User]       `json:"viewers_id" bson:"viewers_id" validate:"unique,dive,user_id_exist"`
	Categories []product.Category       `json:"categories" bson:"categories" validate:"dive,unique"`
	CreatedAt  date.CreateDate[ShopMap] `json:"created_at" bson:"created_at"`
	UpdatedAt  date.UpdateDate[ShopMap] `json:"updated_at" bson:"updated_at"`
}

func NewShopMap() ShopMap {
	return ShopMap{
		ID:        id.NewID[ShopMap](),
		CreatedAt: (),
		UpdatedAt: time.Now(),
	}
}
