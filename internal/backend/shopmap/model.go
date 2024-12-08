package shopmap

import (
	"go-backend/internal/backend/product"
	"go-backend/internal/backend/user"
	"go-backend/pkg/date"
	"go-backend/pkg/id"
)

type ShopMap struct {
	ShopMapConfig `bson:"inline"`

	ID        id.ID[ShopMap]           `json:"id" bson:"id"`
	OwnerID   id.ID[user.User]         `validate:"user_id_exist" json:"owner_id" bson:"ownerId"`
	CreatedAt date.CreateDate[ShopMap] `json:"created_at" bson:"createdAt"`
	UpdatedAt date.UpdateDate[ShopMap] `json:"updated_at" bson:"updatedAt"`
}

type ShopMapConfig struct {
	Categories []product.Category `validate:"dive,unique" json:"categories" bson:"categories"`
	ViewersID  []id.ID[user.User] `validate:"unique,dive,user_id_exist" json:"viewers_id" bson:"viewersId"`
}
