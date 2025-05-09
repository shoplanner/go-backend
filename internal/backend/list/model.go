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

// ENUM(waiting=1, missed, taken, replaced).
type StateStatus int

// ENUM(planning=1, processing, archived).
type ExecStatus int32

type ProductStateReplacement struct {
	Count     mo.Option[int32] `json:"count" swaggertype:"number" extensions:"x-nullable"`
	FormIndex mo.Option[int32] `json:"form_idx" swaggertype:"number" extensions:"x-nullable"`
	Product   product.Product  `json:"product"`
}

type ProductStateOptions struct {
	Count       mo.Option[int32]                   `json:"count" swaggertype:"number" extensions:"x-nullable"`
	FormIndex   mo.Option[int32]                   `json:"form_idx" swaggertype:"number" extensions:"x-nullable"`
	Status      StateStatus                        `json:"status" swaggertype:"string"`
	Replacement mo.Option[ProductStateReplacement] `json:"replacement"`
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

type ListOptions struct { //nolint
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
	f, ch := CheckRole(userID, role)
	err := f(l.Members)
	member := <-ch
	if err != nil {
		return member, err
	}

	return member, nil
}

type ProductsAddedChange struct {
	Products []ProductState `json:"products"`
}

type ProductsRemovedChange struct {
	IDs []id.ID[product.Product] `json:"ids"`
}

type ListOptionsChange struct { //nolint:revive
	NewOptions ListOptions `json:"new_options"`
}

type ListDeletedChange struct{} //nolint:revive

type MembersAddedChange struct {
	NewMembers []Member `json:"new_members"`
}

type MembersDeletedChange struct {
	UserIDs []id.ID[user.User] `json:"user_ids"`
}

type StateUpdatedChange struct {
	ProductID id.ID[product.Product]
	State     ProductState
}

// ENUM(full=1,
// productsAdded,
// productsRemoved,
// membersAdded,
// membersRemoved,
// optsUpdated,
// deleted,
// statesReordered,
// stateUpdated)
type EventType int32

type Change struct {
	Data any       `json:"data"`
	Type EventType `json:"type"`
}

type Event struct {
	ListID id.ID[ProductList] `json:"list_id"`
	Member *Member            `json:"member"`
	Change Change             `json:"change"`
}

type RoleCheckFunc func([]Member) error

func CheckRole(userID id.ID[user.User], role MemberType) (RoleCheckFunc, <-chan Member) {
	returnCh := make(chan Member, 1)

	return func(members []Member) error {
		member := NewZeroMember()

		defer func() {
			returnCh <- member
			close(returnCh)
		}()

		idx := slices.IndexFunc(members, func(m Member) bool {
			return m.UserID == userID
		})

		if idx == -1 {
			return fmt.Errorf("%w: user %s is not belongs to list", myerr.ErrForbidden, userID)
		}

		if role < members[idx].Role {
			return fmt.Errorf("%w: role of user %s is not enough", myerr.ErrForbidden, userID)
		}

		return nil
	}, returnCh
}
