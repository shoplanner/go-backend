package favorite

import (
	"fmt"

	"go-backend/internal/backend/product"
	"go-backend/internal/backend/user"
	"go-backend/pkg/date"
	"go-backend/pkg/id"
	"go-backend/pkg/myerr"
)

//go:generate go-enum --marshal --names --values

type Favorite struct {
	ListID    id.ID[List]               `json:"list_id"`
	Product   product.Product           `json:"product"`
	CreatedAt date.CreateDate[Favorite] `json:"created_at"`
	UpdatedAt date.UpdateDate[Favorite] `json:"updated_at"`
}

// ENUM(admin=1,editor,viewer)
type MemberType int32

// ENUM(personal=1,group)
type ListType int32

type Member struct {
	UserID    id.ID[user.User]        `json:"user_id"`
	Type      MemberType              `json:"type"`
	CreatedAt date.CreateDate[Member] `json:"created_at"`
	UpdatedAt date.UpdateDate[Member] `json:"updated_at"`
}

type List struct {
	ID        id.ID[List]           `json:"id"`
	Members   []Member              `json:"members"`
	CreatedAt date.CreateDate[List] `json:"created_at"`
	UpdatedAt date.UpdateDate[List] `json:"updated_at"`
	Products  []Favorite            `json:"products"`
	Type      ListType              `json:"type"`
}

func (l List) AllowedToEdit(userID id.ID[user.User]) error {
	for _, member := range l.Members {
		if member.UserID == userID && member.Type <= MemberTypeEditor {
			return nil
		} else if member.UserID == userID {
			return fmt.Errorf("%w: member role is not enough", myerr.ErrForbidden)
		}
	}

	return fmt.Errorf("%w: user is not a member", myerr.ErrForbidden)
}
