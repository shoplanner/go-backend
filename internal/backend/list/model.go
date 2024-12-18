package list

import (
	"github.com/google/uuid"
	"github.com/samber/mo"

	"go-backend/internal/backend/product"
	"go-backend/internal/backend/user"
	"go-backend/pkg/date"
	"go-backend/pkg/id"
)

//go:generate go-enum --marshal --names --values

// ENUM(waiting, missing, taken, replaced).
type StateStatus int

// ENUM(planning, processing, archived).
type ExecStatus int

type ProductState struct {
	ProductID uuid.UUID       `bson:"product_id" json:"product_id"`
	Product   product.Product `bson:"product" json:"product"`
	Count     mo.Option[int]  `bson:"count" json:"count"`
	FormIndex mo.Option[int]  `bson:"form_index" json:"form_index"`
	Status    StateStatus     `bson:"status" json:"status"`
}

type ProductList struct {
	Options `bson:"inline"`

	ID        id.ID[ProductList]           `bson:"_id" json:"id"`
	UpdatedAt date.UpdateDate[ProductList] `bson:"updated_at" json:"updated_at"`
	CreatedAt date.CreateDate[ProductList] `bson:"created_at" json:"created_at"`
	OwnerID   id.ID[user.User]             `bson:"owner_id" json:"owner_id"`
}

type Options struct {
	States []ProductState `bson:"states" json:"states"`
	Status ExecStatus     `bson:"status" json:"status"`
}
