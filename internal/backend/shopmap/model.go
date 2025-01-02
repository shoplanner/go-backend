package shopmap

import (
	"go-backend/internal/backend/product"
	"go-backend/internal/backend/user"
	"go-backend/pkg/date"
	"go-backend/pkg/id"
)

type ShopMap struct {
	Options

	ID        id.ID[ShopMap]           `json:"id" swaggertype:"string"`
	OwnerID   id.ID[user.User]         `json:"owner_id" swaggertype:"string"`
	CreatedAt date.CreateDate[ShopMap] `json:"created_at" swaggertype:"string"`
	UpdatedAt date.UpdateDate[ShopMap] `json:"updated_at" swaggertype:"string"`
}

type Options struct {
	CategoryList []product.Category `validate:"unique" json:"categories" swaggertype:"array,string"`
	ViewerIDList []id.ID[user.User] `validate:"unique" json:"viewers_id" swaggertype:"array,string"`
}
