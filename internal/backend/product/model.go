package product

import (
	"github.com/samber/mo"

	"go-backend/pkg/date"
	"go-backend/pkg/id"
)

// Product is a product
type Product struct {
	Options

	ID        id.ID[Product]           `json:"id" swaggertype:"string"`
	CreatedAt date.CreateDate[Product] `json:"created_at" swaggertype:"string"`
	UpdatedAt date.UpdateDate[Product] `json:"updated_at" swaggertype:"string"`
}

// Options are options of a Product
type Options struct {
	Name     Name                `json:"name" swaggertype:"string"`
	Category mo.Option[Category] `json:"category" swaggertype:"string"`
	Forms    []Form              `json:"forms" swaggertype:"array,string"`
}

// NewZeroOptions returns zero options
func NewZeroOptions() Options {
	return Options{
		Name:     "",
		Category: mo.Option[Category]{},
		Forms:    []Form{},
	}
}

// Category is a category of a Product
type Category string

// Form is a form of a Product
type Form string

// Name is a name of a Product
type Name string
