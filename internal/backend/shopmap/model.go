package shopmap

import (
	"go-backend/internal/backend/product"
	"go-backend/internal/backend/user"
	"go-backend/pkg/date"
	"go-backend/pkg/id"
)

type ShopMap struct {
	Options

	ID        id.ID[ShopMap]           `json:"id"`
	OwnerID   id.ID[user.User]         `json:"owner_id"`
	CreatedAt date.CreateDate[ShopMap] `json:"created_at"`
	UpdatedAt date.UpdateDate[ShopMap] `json:"updated_at"`
}

type Options struct {
	CategoryList []product.Category `validate:"unique" json:"categories"`
	ViewerIDList []id.ID[user.User] `validate:"unique" json:"viewers_id"`
}
