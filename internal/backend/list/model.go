package list

import (
	"fmt"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/samber/mo"

	"go-backend/internal/backend/product"
	"go-backend/internal/backend/user"
	"go-backend/pkg/date"
	"go-backend/pkg/id"
	"go-backend/pkg/myerr"
)

//go:generate python $GOENUM

// ENUM(waiting=1, missing, taken, replaced).
type StateStatus int

// ENUM(planning=1, processing, archived).
type ExecStatus int32

type ProductStateOptions struct {
	Count     mo.Option[int32] `json:"count" swaggertype:"number" extensions:"x-nullable"`
	FormIndex mo.Option[int32] `json:"form_index" swaggertype:"number" extensions:"x-nullable"`
	Status    StateStatus      `json:"status" swaggertype:"string"`
}

type ProductState struct {
	ProductStateOptions

	Product   product.Product               `json:"product"`
	CreatedAt date.CreateDate[ProductState] `json:"created_at"`
	UpdatedAt date.UpdateDate[ProductState] `json:"updated_at"`
}

// ENUM(owner=1,admin,editor,executing,viewer)
type MemberType int32

type MemberOptions struct {
	UserID id.ID[user.User] `json:"user_id" swaggertype:"string"`
	Role   MemberType       `json:"type" swaggertype:"string"`
}

type Member struct {
	MemberOptions

	UserName  user.Login              `json:"username"`
	CreatedAt date.CreateDate[Member] `json:"created_at"`
	UpdatedAt date.UpdateDate[Member] `json:"updated_at"`
}

func NewZeroMember() Member {
	return Member{
		MemberOptions: MemberOptions{UserID: id.ID[user.User]{UUID: uuid.Nil}, Role: 0},
		UserName:      "",
		CreatedAt:     date.CreateDate[Member]{Time: time.Time{}},
		UpdatedAt:     date.UpdateDate[Member]{Time: time.Time{}},
	}
}

// nolint:exported // here is another options
type ListOptions struct {
	Status ExecStatus `json:"status" swaggertype:"string"`
	Title  string     `json:"title"`
}

type ProductList struct {
	ListOptions

	States    []ProductState               `json:"states"`
	Members   []Member                     `json:"members"`
	ID        id.ID[ProductList]           `json:"id"`
	UpdatedAt date.UpdateDate[ProductList] `json:"updated_at"`
	CreatedAt date.CreateDate[ProductList] `json:"created_at"`
}

func (l ProductList) CheckRole(userID id.ID[user.User], role MemberType) (Member, error) {
	idx := slices.IndexFunc(l.Members, func(m Member) bool {
		return m.UserID == userID
	})

	if idx == -1 {
		return NewZeroMember(), fmt.Errorf("%w: user %s is not belongs to list %s", myerr.ErrForbidden, userID, l.ID)
	}

	if role < l.Members[idx].Role {
		return NewZeroMember(), fmt.Errorf("%w: role of user %s is not enough", myerr.ErrForbidden, userID)
	}

	return l.Members[idx], nil
}

type ProductsAddedChange struct {
	Products []ProductState `json:"products"`
}

type ProductsRemovedChange struct {
	IDs []id.ID[product.Product] `json:"ids"`
}

type ListOptionsChange struct {
	NewOptions ListOptions `json:"new_options"`
}

type ListDeletedChange struct{}

type MembersAddedChange struct {
	NewMembers []Member `json:"new_members"`
}

type MembersDeletedChange struct {
	UserIDs []id.ID[user.User] `json:"user_ids"`
}

type ListReorderChange struct {
	NewOrder map[uint64]id.ID[product.Product] `json:"new_order"`
}

// ENUM(full=1,productsAdded,productsRemoved,membersAdded,membersRemoved,optsUpdated,deleted)
type EventType int32

type Event struct {
	ListID id.ID[ProductList] `json:"list_id"`
	Member *Member            `json:"member"`
	Type   EventType          `json:"type"`
	Change any                `json:"change"`
}

type RoleCheckFunc func([]Member) error

func CheckRole(members []Member) func()
