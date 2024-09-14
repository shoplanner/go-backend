package models

import (
	"time"

	"github.com/google/uuid"

	"go-backend/internal/product"
)

//go:generate go-enum --marshal --names --values

// ENUM(waiting, missing, taken, replaced).
type StateStatus int

// ENUM(planning, processing, archived).
type Status int

type ProductState struct {
	ProductID uuid.UUID       `bson:"product_id" json:"product_id"`
	Product   product.Request `bson:"product" json:"product"`
	Count     *int            `bson:"count" json:"count"`
	FormIndex *int            `bson:"form_index" json:"form_index"`
	Status    StateStatus     `bson:"status" json:"status"`
}

type ProductList struct {
	ID      uuid.UUID `bson:"_id" json:"id"`
	OwnerID uuid.UUID
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
	Status Status         `bson:"status" json:"status"`
	States []ProductState `bson:"states" json:"states"`
}
