package repo

import (
	"context"
	"fmt"
	"go-backend/internal/backend/list"
	"go-backend/internal/backend/user"
	"go-backend/internal/backend/user/repo"
	"go-backend/pkg/date"
	"go-backend/pkg/god"
	"go-backend/pkg/id"
	"time"

	"github.com/google/uuid"
	"github.com/samber/lo"
	"github.com/samber/mo"
	"gorm.io/gorm"

	productRepo "go-backend/internal/backend/product/repo"
)

type ProductListState struct {
	ID        string              `gorm:"primaryKey;size:36;notNull"`
	ProductID string              `gorm:"notNull"`
	Product   productRepo.Product `gorm:"references:ID"`
	Count     *int32
	FormIdx   *int32
	Status    int       `gorm:"notNull"`
	ListID    string    `gorm:"size:36;notNull"`
	CreatedAt time.Time `gorm:"notNull"`
	UpdatedAt time.Time `gorm:"notNull"`
}

type ProductListMember struct {
	ID         string    `gorm:"primaryKey;size:36;notNull"`
	UserID     string    `gorm:"size:36;notNull"`
	User       repo.User `gorm:"references:ID"`
	ListID     string    `gorm:"size:36;notNull"`
	CreatedAt  time.Time `gorm:"notNull"`
	UpdatedAt  time.Time `gorm:"notNull"`
	MemberType int32     `gorm:"notNull"`
}

type ProductList struct {
	ID        string    `gorm:"primaryKey;size:36;notNull"`
	Status    int32     `gorm:"notNull"`
	UpdatedAt time.Time `gorm:"notNull"`
	CreatedAt time.Time `gorm:"notNull"`
	Title     string    `gorm:"notNull,size:255"`
	Members   []ProductListMember
	States    []ProductListState
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
	err := r.db.WithContext(ctx).
		Where(&ProductListMember{UserID: userID.String()}).Pluck("list_id", &relatedListIDs).Error //nolint:exhaustruct
	if err != nil {
		return nil, fmt.Errorf("can't find ids of lists related to user %s: %w", userID, err)
	}

	err = r.db.WithContext(ctx).
		Where("id in ?", relatedListIDs).
		Find(&lists).Error
	if err != nil {
		return nil, fmt.Errorf("can't select lists related to user %s: %w", userID, err)
	}

	return lo.Map(lists, func(item ProductList, _ int) list.ProductList { return entityToModel(item) }), nil
}

func (r *Repo) GetListByID(ctx context.Context, listID id.ID[list.ProductList]) (list.ProductList, error) {
	entity := &ProductList{ID: listID.String()} //nolint:exhaustruct

	err := r.db.WithContext(ctx).
		Preload("Members").
		Preload("States").
		Preload("States.Product.Category").
		Find(&entity).Error
	if err != nil {
		return list.ProductList{}, fmt.Errorf("can't select product list %s: %w", listID, err)
	}

	return entityToModel(*entity), nil
}

func (r *Repo) CreateList(ctx context.Context, model list.ProductList) error {
	err := r.db.WithContext(ctx).Create(lo.ToPtr(modelToEntity(model))).Error
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
		entity := ProductList{ID: listID.String()} //nolint:exhaustruct

		err = tx.WithContext(ctx).
			Preload("Members").
			Preload("States").
			Preload("States.Product.Category").
			Find(&entity).Error
		if err != nil {
			return fmt.Errorf("can't select product list %s: %w", listID, err)
		}

		model, err = updateFunc(entityToModel(entity))
		if err != nil {
			return err
		}
		entity = modelToEntity(model)
		query := ProductList{ID: listID.String()} //nolint:exhaustruct

		err = tx.WithContext(ctx).
			Model(&query).
			Association("Members").
			Unscoped().
			Replace(entity.Members)
		if err != nil {
			return fmt.Errorf("can't update members of list %s: %w", listID, err)
		}

		err = tx.WithContext(ctx).
			Model(&query).
			Association("States").
			Unscoped().
			Replace(entity.States)
		if err != nil {
			return fmt.Errorf("can't update states of list %s: %w", listID, err)
		}

		err = tx.WithContext(ctx).
			Model(&query).
			Updates(&entity).
			Error
		if err != nil {
			return fmt.Errorf("can't update product list %s: %w", listID, err)
		}

		return nil
	})
	if err != nil {
		return list.ProductList{}, fmt.Errorf("transaction failed: %w", err)
	}

	return model, nil
}

func entityToModel(entity ProductList) list.ProductList {
	return list.ProductList{
		States: lo.Map(entity.States, func(item ProductListState, _ int) list.ProductState {
			return stateToModel(item)
		}),
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
			UserID: id.ID[user.User]{UUID: god.Believe(uuid.Parse(entity.ID))},
			Role:   list.MemberType(entity.MemberType),
		},
		UserName:  user.Login(entity.User.Login),
		CreatedAt: date.CreateDate[list.Member]{Time: entity.CreatedAt},
		UpdatedAt: date.UpdateDate[list.Member]{Time: entity.UpdatedAt},
	}
}

func stateToModel(entity ProductListState) list.ProductState {
	return list.ProductState{
		ProductStateOptions: list.ProductStateOptions{
			Count:     mo.PointerToOption(entity.Count),
			FormIndex: mo.PointerToOption(entity.FormIdx),
			Status:    list.StateStatus(entity.Status),
		},
		Product:   productRepo.EntityToModel(entity.Product),
		CreatedAt: date.CreateDate[list.ProductState]{Time: entity.CreatedAt},
		UpdatedAt: date.UpdateDate[list.ProductState]{Time: entity.UpdatedAt},
	}
}

func modelToEntity(model list.ProductList) ProductList {
	return ProductList{
		ID:        model.ID.String(),
		Status:    int32(model.Status),
		UpdatedAt: model.CreatedAt.Time,
		CreatedAt: model.UpdatedAt.Time,
		Title:     model.Title,
		Members: lo.Map(model.Members, func(item list.Member, _ int) ProductListMember {
			return memberToEntity(model.ID, item)
		}),
		States: lo.Map(model.States, func(item list.ProductState, _ int) ProductListState {
			return stateToEntity(model.ID, item)
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

func stateToEntity(listID id.ID[list.ProductList], model list.ProductState) ProductListState {
	return ProductListState{
		ID:        uuid.NewString(),
		ProductID: model.Product.ID.String(),
		Product:   productRepo.Product{ID: model.Product.ID.String()}, //nolint:exhaustruct
		Count:     model.Count.ToPointer(),
		FormIdx:   model.FormIndex.ToPointer(),
		Status:    int(model.Status),
		ListID:    listID.String(),
		CreatedAt: model.CreatedAt.Time,
		UpdatedAt: model.UpdatedAt.Time,
	}
}
