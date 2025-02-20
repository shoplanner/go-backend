package list

import (
	"fmt"
	"slices"

	"github.com/samber/mo"

	"go-backend/internal/backend/product"
	"go-backend/internal/backend/user"
	"go-backend/pkg/date"
	"go-backend/pkg/id"
	"go-backend/pkg/myerr"
)

//go:generate go-enum --marshal --names --values

// ENUM(waiting=1, missing, taken, replaced).
type StateStatus int

// ENUM(planning=1, processing, archived).
type ExecStatus int32

type ProductState struct {
	Product   product.Product               `json:"product"`
	Count     mo.Option[int32]              `json:"count"`
	FormIndex mo.Option[int32]              `json:"form_index"`
	Status    StateStatus                   `json:"status"`
	CreatedAt date.CreateDate[ProductState] `json:"created_at"`
	UpdatedAt date.UpdateDate[ProductState] `json:"updated_at"`
}

// ENUM(owner=1,admin,editor,executing,viewer)
type MemberType int32

type MemberOptions struct {
	UserID id.ID[user.User] `json:"user_id"`
	Role   MemberType       `json:"type"`
}

type Member struct {
	MemberOptions

	UserName  user.Login              `json:"username"`
	CreatedAt date.CreateDate[Member] `json:"created_at"`
	UpdatedAt date.UpdateDate[Member] `json:"updated_at"`
}

type Options struct {
	States  []ProductState `json:"states"`
	Members []Member       `json:"members"`
	Status  ExecStatus     `json:"status"`
	Title   string         `json:"title"`
}

type ProductList struct {
	Options

	ID        id.ID[ProductList]           `json:"id"`
	UpdatedAt date.UpdateDate[ProductList] `json:"updated_at"`
	CreatedAt date.CreateDate[ProductList] `json:"created_at"`
}

func (l ProductList) CheckRole(userID id.ID[user.User], role MemberType) error {
	idx := slices.IndexFunc(l.Members, func(m Member) bool {
		return m.UserID == userID
	})

	if idx == -1 {
		return fmt.Errorf("%w: user %s is not belongs to list %s", myerr.ErrForbidden, userID, l.ID)
	}

	if role > l.Members[idx].Role {
		return fmt.Errorf("%w: role of user %s is not enough", myerr.ErrForbidden, userID)
	}

	return nil
}
