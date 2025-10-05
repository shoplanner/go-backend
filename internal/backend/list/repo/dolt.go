package repo

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/samber/lo"
	"github.com/samber/mo"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"go-backend/internal/backend/list"
	"go-backend/internal/backend/product"
	productRepo "go-backend/internal/backend/product/repo"
	"go-backend/internal/backend/user"
	"go-backend/internal/backend/user/repo"
	"go-backend/pkg/date"
	"go-backend/pkg/god"
	"go-backend/pkg/id"
	"go-backend/pkg/myerr"
	"go-backend/pkg/mymysql"
)

type ProductListState struct {
	ID                   string              `gorm:"primaryKey;size:36;notNull"`
	ProductID            string              `gorm:"notNull"`
	Product              productRepo.Product `gorm:"references:ID"`
	ListID               string              `gorm:"size:36;notNull"`
	CreatedAt            time.Time           `gorm:"notNull"`
	UpdatedAt            time.Time           `gorm:"notNull"`
	Index                int64               `gorm:"notNull"`
	Count                *int32
	FormIdx              *int32
	Status               int `gorm:"notNull"`
	ReplacementCount     *int32
	ReplacementFormIdx   *int32
	ReplacementProduct   *productRepo.Product `gorm:"references:ID"`
	ReplacementProductID *string
}

type ProductListMember struct {
	ID         string    `gorm:"primaryKey;size:36;notNull"`
	UserID     string    `gorm:"size:36;notNull;uniqueIndex:idx_list_user"`
	User       repo.User `gorm:"references:ID"`
	ListID     string    `gorm:"size:36;notNull;uniqueIndex:idx_list_user"`
	CreatedAt  time.Time `gorm:"notNull"`
	UpdatedAt  time.Time `gorm:"notNull"`
	MemberType int32     `gorm:"notNull"`
}

type ProductList struct {
	ID        string              `gorm:"primaryKey;size:36;notNull"`
	Status    int32               `gorm:"notNull"`
	UpdatedAt time.Time           `gorm:"notNull"`
	CreatedAt time.Time           `gorm:"notNull"`
	Title     string              `gorm:"notNull,size:255"`
	Members   []ProductListMember `gorm:"foreignKey:ListID;constraint:OnDelete:CASCADE"`
	States    []ProductListState  `gorm:"foreignKey:ListID"`
}

type Repo struct {
	db *gorm.DB
}

func NewRepo(ctx context.Context, db *gorm.DB) (*Repo, error) {
	err := db.WithContext(ctx).AutoMigrate(new(ProductList), new(ProductListMember), new(ProductListState))
	if err != nil {
		return nil, fmt.Errorf("can't create product list tables: %w", err)
	}

	return &Repo{db: db}, nil
}

func (r *Repo) GetListMetaByUserID(ctx context.Context, userID id.ID[user.User]) ([]list.ProductList, error) {
	var relatedListIDs []string
	var lists []ProductList

	//nolint:exhaustruct
	err := r.db.WithContext(ctx).Model(&ProductListMember{}).
		Where(&ProductListMember{UserID: userID.String()}).
		Pluck("list_id", &relatedListIDs).Error
	if err != nil {
		return nil, fmt.Errorf("can't find ids of lists related to user %s: %w", userID, err)
	}

	err = r.db.WithContext(ctx).
		Preload("Members").
		Preload("Members.User").
		Preload("States").
		Preload("States.Product.Forms").
		Preload("States.Product.Category").
		Preload("States.ReplacementProduct").
		Preload("States.ReplacementProduct.Category").
		Where("id in ?", relatedListIDs).
		Find(&lists).Error
	if err != nil {
		return nil, fmt.Errorf("can't select lists related to user %s: %w", userID, err)
	}

	return lo.Map(lists, func(item ProductList, _ int) list.ProductList { return entityToModel(item) }), nil
}

func (r *Repo) GetByListID(ctx context.Context, listID id.ID[list.ProductList]) (list.ProductList, error) {
	return r.getProductList(ctx, r.db, listID)
}

func (r *Repo) CreateList(ctx context.Context, model list.ProductList) error {
	err := r.db.WithContext(ctx).Create(lo.ToPtr(listToEntity(model))).Error
	if err != nil {
		return fmt.Errorf("can't insert new product list %s: %w", model.ID, err)
	}

	return nil
}

func (r *Repo) GetAndUpdate(
	ctx context.Context,
	listID id.ID[list.ProductList],
	updateFunc func(list.ProductList) (list.ProductList, error),
) (
	list.ProductList,
	error,
) {
	var model list.ProductList
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var err error
		model, err = r.getProductList(ctx, tx, listID)
		if err != nil {
			return err
		}

		model, err = updateFunc(model)
		if err != nil {
			return err
		}

		entity := listToEntity(model)
		query := ProductList{ID: listID.String()} //nolint:exhaustruct

		err = tx.WithContext(ctx).Model(&query).Association("Members").Unscoped().Replace(entity.Members)
		if err != nil {
			return fmt.Errorf("can't update members of list %s: %w", listID, err)
		}

		err = tx.WithContext(ctx).Model(&query).Association("States").Unscoped().Replace(entity.States)
		if err != nil {
			return fmt.Errorf("can't update states of list %s: %w", listID, err)
		}

		err = tx.WithContext(ctx).Model(&query).Updates(&entity).Error
		if err != nil {
			return fmt.Errorf("can't update product list %s: %w", listID, err)
		}

		model, err = r.getProductList(ctx, tx, listID)
		return err
	})
	if err != nil {
		return list.ProductList{}, fmt.Errorf("%w: transaction failed: %w", mymysql.GetType(err), err)
	}

	return model, nil
}

func (r *Repo) GetAndDeleteList(
	ctx context.Context,
	listID id.ID[list.ProductList],
	validateFunc func(list.ProductList) error,
) error {
	err := r.db.Transaction(func(tx *gorm.DB) error {
		model, err := r.getProductList(ctx, tx, listID)
		if err != nil {
			return err
		}

		if err = validateFunc(model); err != nil {
			return err
		}

		//nolint:exhaustruct
		err = tx.WithContext(ctx).Delete(&ProductList{ID: listID.String()}).Error
		if err != nil {
			return fmt.Errorf("can't delete product list %s: %w", listID, err)
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("transaction failed: %w", err)
	}

	return nil
}

func (r *Repo) ApplyOrder(
	ctx context.Context,
	validateFunc list.RoleCheckFunc,
	listID id.ID[list.ProductList],
	ids []id.ID[product.Product],
) error {
	err := r.db.Transaction(func(tx *gorm.DB) error {
		members, err := r.getMembers(ctx, tx, listID)
		if err != nil {
			return fmt.Errorf("failed to get product list members: %w", err)
		}

		if err = validateFunc(members); err != nil {
			return err
		}

		if len(ids) == 0 {
			return fmt.Errorf("%w: order list is empty", myerr.ErrInvalidArgument)
		}

		args := make([]any, 0, 3*len(ids)+1)
		for i, id := range ids {
			args = append(args, id.String(), i)
		}
		for _, id := range ids {
			args = append(args, id)
		}

		query := buildApplyOrderQeuery(ids)
		args = append(args, listID)

		err = tx.WithContext(ctx).Exec(query, args...).Error
		if err != nil {
			return fmt.Errorf("can't apply order to DoltDB: %w", err)
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("transaction failed: %w", err)
	}

	return nil
}

func buildApplyOrderQeuery(ids []id.ID[product.Product]) string {
	var builder strings.Builder

	builder.WriteString("UPDATE product_list_states SET `index` = CASE product_id ")

	for range ids {
		builder.WriteString("WHEN ? THEN ? ")
	}

	builder.WriteString("END WHERE `product_id` in (")
	for i := range ids {
		if i != len(ids)-1 {
			builder.WriteString("?, ")
		} else {
			builder.WriteString("?) ")
		}
	}

	builder.WriteString(" AND `list_id` = ?")

	return builder.String()
}

func (r *Repo) getMembers(ctx context.Context, tx *gorm.DB, listID id.ID[list.ProductList]) ([]list.Member, error) {
	var members []ProductListMember

	err := tx.WithContext(ctx).
		Preload("User").
		Where(&ProductListMember{ListID: listID.String()}). // nolint:exhaustruct
		Find(&members).Error

	if err != nil {
		return nil, fmt.Errorf("failed to query members of list %s: %w", listID, err)
	}

	return lo.Map(members, func(m ProductListMember, _ int) list.Member {
		return memberToModel(m)
	}), nil
}

func (r *Repo) getProductList(ctx context.Context, tx *gorm.DB, listID id.ID[list.ProductList]) (
	list.ProductList, error,
) {
	entity := ProductList{ID: listID.String()} //nolint:exhaustruct

	err := tx.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).
		Preload("Members").
		Preload("Members.User").
		Preload("States").
		Preload("States.Product.Forms").
		Preload("States.Product.Category").
		Preload("States.ReplacementProduct").
		Preload("States.ReplacementProduct.Category").
		Find(&entity).Error
	if err != nil {
		return list.ProductList{}, fmt.Errorf("can't select product list %s: %w", listID, err)
	}

	return entityToModel(entity), nil
}

func entityToModel(entity ProductList) list.ProductList {
	states := make([]list.ProductState, len(entity.States))
	for _, state := range entity.States {
		states[state.Index] = stateToModel(state)
	}

	return list.ProductList{
		States: states,
		Members: lo.Map(entity.Members, func(item ProductListMember, _ int) list.Member {
			return memberToModel(item)
		}),
		ListOptions: list.ListOptions{
			Status: list.ExecStatus(entity.Status),
			Title:  entity.Title,
		},
		ID:        id.ID[list.ProductList]{UUID: god.Believe(uuid.Parse(entity.ID))},
		UpdatedAt: date.UpdateDate[list.ProductList]{Time: entity.UpdatedAt},
		CreatedAt: date.CreateDate[list.ProductList]{Time: entity.CreatedAt},
	}
}

func memberToModel(entity ProductListMember) list.Member {
	return list.Member{
		MemberOptions: list.MemberOptions{
			UserID: id.ID[user.User]{UUID: god.Believe(uuid.Parse(entity.UserID))},
			Role:   list.MemberType(entity.MemberType),
		},
		UserName:  user.Login(entity.User.Login),
		CreatedAt: date.CreateDate[list.Member]{Time: entity.CreatedAt},
		UpdatedAt: date.UpdateDate[list.Member]{Time: entity.UpdatedAt},
	}
}

func stateToModel(entity ProductListState) list.ProductState {
	var replacement *list.ProductStateReplacement
	if entity.ReplacementProductID != nil {
		replacement = &list.ProductStateReplacement{
			Count:     mo.PointerToOption(entity.ReplacementCount),
			FormIndex: mo.PointerToOption(entity.ReplacementFormIdx),
			Product:   productRepo.EntityToModel(*entity.ReplacementProduct),
		}
	}

	return list.ProductState{
		ProductStateOptions: list.ProductStateOptions{
			Count:       mo.PointerToOption(entity.Count),
			FormIndex:   mo.PointerToOption(entity.FormIdx),
			Status:      list.StateStatus(entity.Status),
			Replacement: mo.PointerToOption(replacement),
		},
		Product:   productRepo.EntityToModel(entity.Product),
		CreatedAt: date.CreateDate[list.ProductState]{Time: entity.CreatedAt},
		UpdatedAt: date.UpdateDate[list.ProductState]{Time: entity.UpdatedAt},
	}
}

func listToEntity(model list.ProductList) ProductList {
	return ProductList{
		ID:        model.ID.String(),
		Status:    int32(model.Status),
		UpdatedAt: model.UpdatedAt.Time,
		CreatedAt: model.CreatedAt.Time,
		Title:     model.Title,
		Members: lo.Map(model.Members, func(item list.Member, _ int) ProductListMember {
			return memberToEntity(model.ID, item)
		}),
		States: lo.Map(model.States, func(item list.ProductState, index int) ProductListState {
			return stateToEntity(model.ID, item, int64(index))
		}),
	}
}

func memberToEntity(listID id.ID[list.ProductList], model list.Member) ProductListMember {
	return ProductListMember{
		ID:         uuid.NewString(),
		UserID:     model.UserID.String(),
		User:       repo.User{ID: model.UserID.String()}, //nolint:exhaustruct
		ListID:     listID.String(),
		CreatedAt:  model.CreatedAt.Time,
		UpdatedAt:  model.UpdatedAt.Time,
		MemberType: int32(model.Role),
	}
}

func stateToEntity(listID id.ID[list.ProductList], model list.ProductState, index int64) ProductListState {
	return ProductListState{
		ID:                 uuid.NewString(),
		ProductID:          model.Product.ID.String(),
		Product:            productRepo.Product{ID: model.Product.ID.String()}, //nolint:exhaustruct
		Count:              model.Count.ToPointer(),
		FormIdx:            model.FormIndex.ToPointer(),
		Status:             int(model.Status),
		ListID:             listID.String(),
		CreatedAt:          model.CreatedAt.Time,
		UpdatedAt:          model.UpdatedAt.Time,
		Index:              index,
		ReplacementCount:   model.Replacement.OrEmpty().Count.ToPointer(),
		ReplacementFormIdx: model.Replacement.OrEmpty().FormIndex.ToPointer(),
		ReplacementProductID: lo.If(model.Replacement.IsAbsent(), (*string)(nil)).
			Else(lo.ToPtr(model.Replacement.OrEmpty().Product.ID.String())),
	}
}
