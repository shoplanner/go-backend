package repo

import (
	"context"
	"fmt"
	"os/user"
	"time"

	"github.com/samber/lo"
	"gorm.io/gorm"

	"go-backend/internal/backend/favorite"
	"go-backend/internal/backend/product"
	"go-backend/internal/backend/user/repo"
	"go-backend/pkg/id"
)

type FavoriteList struct {
	ID        string            `gorm:"primaryKey;size:36;notNull"`
	ListType  int32             `gorm:"notNull"`
	CreatedAt time.Time         `gorm:"notNull"`
	UpdatedAt time.Time         `gorm:"notNull"`
	Members   []FavoriteMember  `gorm:"notNull"`
	Products  []FavoriteProduct `gorm:"notNull"`
}

type FavoriteMember struct {
	ID         string    `gorm:"primaryKey;size:36;notNull"`
	UserID     string    `gorm:"size:36;references:ID;notNull"`
	User       repo.User `gorm:"references:ID;notNull"`
	ListID     string    `gorm:"size:36;notNull"`
	CreatedAt  time.Time `gorm:"notNull"`
	UpdatedAt  time.Time `gorm:"notNull"`
	MemberType int       `gorm:"notNull"`
}

type FavoriteProduct struct {
	ID        string    `gorm:"primaryKey;size:36;notNull"`
	ProductID string    `gorm:"size:36;notNull"`
	ListID    string    `gorm:"size:36;references:ID;notNull"`
	CreatedAt time.Time `gorm:"notNull"`
	UpdatedAt time.Time `gorm:"notNull"`
}

type Repo struct {
	db *gorm.DB
}

func NewRepo(ctx context.Context, db *gorm.DB) (*Repo, error) {
	if err := db.WithContext(ctx).AutoMigrate(new(FavoriteList), new(FavoriteMember), new(FavoriteProduct)); err != nil {
		return nil, fmt.Errorf("can't create favorites tables: %w", err)
	}

	return &Repo{db: db}, nil
}

func (r *Repo) AddProduct(ctx context.Context, listID id.ID[favorite.List], model favorite.Favorite) error {
	if err := r.db.Create(lo.ToPtr(productToEntity(listID, model))).Error; err != nil {
		return fmt.Errorf("can't add product %s to list %s: %w", model.Product.ID, listID, err)
	}
	return nil
}

func (r *Repo) CreateList(ctx context.Context, model favorite.List) error {
	if err := r.db.Create(lo.ToPtr(modelToEntity(model))).Error; err != nil {
		return fmt.Errorf("can't create new list %s: %w", model.ID, err)
	}
	return nil
}

func (r *Repo) DeleteProduct(ctx context.Context, productID id.ID[product.Product], listID id.ID[favorite.List]) error {
	entity := &FavoriteProduct{ProductID: productID.String(), ListID: listID.String()}
	if err := r.db.Where(entity).Delete(&entity); err != nil {
		return fmt.Errorf("can't delete favorite product %s from list %s: %w", productID, listID, err)
	}

	return nil
}

func (r *Repo) AddMember(ctx context.Context, listID id.ID[favorite.List], member favorite.Member) error {
	if err := r.db.WithContext(ctx).Create(lo.ToPtr(memberToEntity(listID, member))); err != nil {
		return fmt.Errorf("can't add member %s to list %s: %w", err)
	}

	return nil
}

func (r *Repo) DeleteMember(ctx context.Context, listID id.ID[favorite.List], userID id.ID[user.User]) error {
	entity := &FavoriteMember{ListID: listID.String(), UserID: userID.String()}
	if err := r.db.Where(entity).Delete(entity).Error; err != nil {
		return fmt.Errorf("can't delete member %s from list %s: %w", userID, listID, err)
	}

	return nil
}

func (r *Repo) UpdateMember(ctx context.Context, listID id.ID[favorite.List], model favorite.Member) error {
	query := &FavoriteMember{ListID: listID.String(), UserID: model.UserID.String()}
	if err := r.db.Where(query).Updates(lo.ToPtr(memberToEntity(listID, model))); err != nil {
		return fmt.Errorf("can't update member %s in list %s: %w", err)
	}

	return nil
}

func (r *Repo) DeleteList(ctx context.Context, listID id.ID[favorite.List]) error {
	if err := r.db.WithContext(ctx).Delete(&FavoriteList{ID: listID.String()}).Error; err != nil {
		return fmt.Errorf("can't delete favorites list %s: %w", listID, err)
	}

	return nil
}

func (r *Repo) GetAndUpdate(ctx context.Context, listID id.ID[favorite.List], f func(favorite.List) (favorite.List, error)) (
	favorite.Favorite,
	error,
) {
	var model favorite.List
	r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var err error

		entity := FavoriteList{ID: listID.String()}
		if err := tx.WithContext(ctx).Preload("FavoriteMembers").Preload("FavoriteProducts").First(&entity); err != nil {
			return fmt.Errorf("can't select favorites list %s: %w", listID, err)
		}

		model, err = f(entityToModel(entity))
		if err != nil {
			return err
		}

		tx.WithContext(ctx).Association("FavoriteMembers").
	})
}

func entityToModel(entity FavoriteList) favorite.List {}

func modelToEntity(model favorite.List) FavoriteList {
	entity := FavoriteList{
		ID:        model.ID.String(),
		ListType:  int32(model.Type),
		CreatedAt: model.CreatedAt.Time,
		UpdatedAt: model.UpdatedAt.Time,
		Members:   make([]FavoriteMember, 0, len(model.Members)),
		Products:  make([]FavoriteProduct, 0, len(model.Products)),
	}
	for _, member := range model.Members {
		entity.Members = append(entity.Members, memberToEntity(model.ID, member))
	}
	for _, productModel := range model.Products {
		entity.Products = append(entity.Products, productToEntity(model.ID, productModel))
	}

	return entity
}

func memberToEntity(listID id.ID[favorite.List], member favorite.Member) FavoriteMember {
	return FavoriteMember{
		ID:        "",
		UserID:    member.UserID.String(),
		ListID:    listID.String(),
		CreatedAt: member.CreatedAt.Time,
		UpdatedAt: member.UpdatedAt.Time,
	}
}

func productToEntity(listID id.ID[favorite.List], model favorite.Favorite) FavoriteProduct {
	return FavoriteProduct{
		ID:        "",
		ProductID: model.Product.ID.String(),
		ListID:    listID.String(),
		CreatedAt: model.CreatedAt.Time,
		UpdatedAt: model.UpdatedAt.Time,
	}
}
