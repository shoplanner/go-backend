package models

import (
	"time"

	"github.com/google/uuid"
)

type Favorite struct {
	ProductID uuid.UUID `json:"product_id" db:"_id"`
	CreatedAt time.Time `json:"created_at" bson:"created_at"`
	UpdatedAt time.Time `json:"updated_at" bson:"updated_at"`
}

type List struct {
	UserID    uuid.UUID  `json:"user_id" bson:"user_id"`
	CreatedAt time.Time  `json:"created_at" bson:"created_at"`
	UpdatedAt time.Time  `json:"updated_at" bson:"updated_at"`
	Products  []Favorite `json:"products" bson:"products"`
}
