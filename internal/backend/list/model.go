package list

import (
	"time"

	"github.com/google/uuid"

	"go-backend/internal/backend/product"
)

//go:generate go-enum --marshal --names --values

// ENUM(waiting, missing, taken, replaced).
type StateStatus int

// ENUM(planning, processing, archived).
type ExecStatus int

type ProductState struct {
	ProductID uuid.UUID       `bson:"product_id" json:"product_id"`
	Product   product.Product `bson:"product" json:"product"`
	Count     *int            `bson:"count" json:"count"`
	FormIndex *int            `bson:"form_index" json:"form_index"`
	Status    StateStatus     `bson:"status" json:"status"`
}

type ProductList struct {
	ProductListOptions `bson:"inline"`

	ID        uuid.UUID `bson:"_id" json:"id"`
	UpdatedAt time.Time `bson:"updated_at" json:"updated_at"`
	CreatedAt time.Time `bson:"created_at" json:"created_at"`
}

type ProductListOptions struct {
	States  []ProductState `bson:"states" json:"states" binding:"dive"`
	Status  ExecStatus     `bson:"status" json:"status"`
	OwnerID uuid.UUID      `bson:"owner_id" json:"owner_id"`
}
