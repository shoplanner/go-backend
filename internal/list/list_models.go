package list

import (
	"time"

	"github.com/google/uuid"

	"go-backend/internal/product"
)

//go:generate go-enum --marshal --names --values

// ENUM(waiting, missing, taken, replaced)
type StateStatus int

// ENUM(planning, processing, archived)
type ListStatus int

type ProductState struct {
	ProductID uuid.UUID              `bson:"product_id" json:"product_id"`
	Product   product.ProductRequest `bson:"product" json:"product"`
	Count     *int                   `bson:"count" json:"count"`
	FormIndex *int                   `bson:"form_index" json:"form_index"`
	Status    StateStatus            `bson:"status" json:"status"`
}

type ProductListResponse struct {
	ProductListRequest `bson:"inline"`

	OwnerID      uuid.UUID   `bson:"user_id" json:"-"`
	ViewerIDList []uuid.UUID `bson:"view_id_list" json:"-"`
	CreatedAt    time.Time   `bson:"created_at" json:"created_at"`
	UpdatedAt    time.Time   `bson:"updated_at" json:"updated_at"`
}

type ProductListRequest struct {
	ID     uuid.UUID      `bson:"_id" json:"id"`
	Name   string         `bson:"name" json:"name"`
	Status ListStatus     `bson:"status" json:"status"`
	States []ProductState `bson:"states" json:"states"`
}
