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
	ProductID uuid.UUID       `json:"product_id"`
	Product   product.Product `json:"product"`
	Count     mo.Option[int]  `json:"count"`
	FormIndex mo.Option[int]  `json:"form_index"`
	Status    StateStatus     `json:"status"`
}

type ProductList struct {
	Options

	ID        id.ID[ProductList]           `json:"id"`
	UpdatedAt date.UpdateDate[ProductList] `json:"updated_at"`
	CreatedAt date.CreateDate[ProductList] `json:"created_at"`
	OwnerID   id.ID[user.User]             `json:"owner_id"`
}

type Options struct {
	States []ProductState `json:"states"`
	Status ExecStatus     `json:"status"`
}
