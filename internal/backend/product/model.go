package product

import (
	"go-backend/pkg/date"
	"go-backend/pkg/id"
)

type Product struct {
	Options

	ID        id.ID[Product]           `json:"id"`
	CreatedAt date.CreateDate[Product] `json:"created_at"`
	UpdatedAt date.UpdateDate[Product] `json:"updated_at"`
}

type Options struct {
	Name     Name     `json:"name"`
	Category Category `json:"category"`
	Forms    []Form   `json:"forms"`
}

type Category string

type Form string

type Name string
