package favorite

import (
	"time"

	"go-backend/internal/backend/product"
	"go-backend/internal/backend/user"
	"go-backend/pkg/id"
)

type Favorite struct {
	ProductID id.ID[product.Product] `json:"product_id"`
	CreatedAt time.Time              `json:"created_at"`
	UpdatedAt time.Time              `json:"updated_at"`
}

type List struct {
	UserID    id.ID[user.User] `json:"user_id"`
	CreatedAt time.Time        `json:"created_at"`
	UpdatedAt time.Time        `json:"updated_at"`
	Products  []Favorite       `json:"products"`
}
