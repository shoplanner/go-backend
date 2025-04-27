package product

import (
	"github.com/samber/mo"

	"go-backend/pkg/date"
	"go-backend/pkg/id"
)

type Product struct {
	Options

	ID        id.ID[Product]           `json:"id" swaggertype:"string"`
	CreatedAt date.CreateDate[Product] `json:"created_at" swaggertype:"string"`
	UpdatedAt date.UpdateDate[Product] `json:"updated_at" swaggertype:"string"`
}

type Options struct {
	Name     Name                `json:"name" swaggertype:"string"`
	Category mo.Option[Category] `json:"category" swaggertype:"string"`
	Forms    []Form              `json:"forms" swaggertype:"array,string"`
}

func NewZeroOptions() Options {
	return Options{
		Name:     "",
		Category: mo.Option[Category]{},
		Forms:    []Form{},
	}
}

type Category string

type Form string

type Name string
