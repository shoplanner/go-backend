package models

import (
	"time"

	"github.com/google/uuid"
)

type Product struct {
	ProductInfo `bson:"inline"`

	ID        uuid.UUID `json:"id" bson:"_id"`
	CreatedAt time.Time `json:"created_at" bson:"created_at"`
	UpdatedAt time.Time `json:"updated_at" bson:"updated_at"`
}

type ProductInfo struct {
	Name     Name     `json:"name" bson:"name" binding:"required"`
	Category Category `bson:"category" json:"category"`
	Forms    []Form   `bson:"forms" json:"forms" binding:"dive,required"`
}

type Category string

type Form string

type Name string
