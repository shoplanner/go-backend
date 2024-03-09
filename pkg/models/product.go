package models

import (
	"time"

	"github.com/google/uuid"
)

type ProductResponse struct {
	ProductRequest `bson:"inline"`

	ID        uuid.UUID `json:"id" bson:"_id"`
	CreatedAt time.Time `json:"created_at" bson:"created_at"`
	UpdatedAt time.Time `json:"updated_at" bson:"updated_at"`
}

type ProductRequest struct {
	Name     string   `json:"name" bson:"name" binding:"required"`
	Category string   `bson:"category" json:"category"`
	Forms    []string `bson:"forms" json:"forms" binding:"dive,required"`
}
