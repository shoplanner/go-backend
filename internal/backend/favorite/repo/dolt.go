package repo

import (
	"context"
	"fmt"
	"os/user"
	"time"

	"github.com/google/uuid"
	"github.com/samber/lo"
	"gorm.io/gorm"

	"go-backend/internal/backend/favorite"
	"go-backend/internal/backend/product"
	"go-backend/internal/backend/user/repo"
	"go-backend/pkg/bd"
	"go-backend/pkg/id"
)

//go:generate $SQLC_HELPER

type FavoriteList struct {
	ID        string `gorm:"primaryKey;size:36"`
	ListType  int32
	CreatedAt time.Time
	UpdatedAt time.Time
	Members   []FavoriteMember
	Products  []FavoriteProduct
}

type FavoriteMember struct {
	ID         string `gorm:"primaryKey;size:36"`
	UserID     string `gorm:"size:36;references:ID"`
	User       repo.User
	ListID     string `gorm:"size:36;references:ID"`
	CreatedAt  time.Time
	UpdatedAt  time.Time
	MemberType int
}

type FavoriteProduct struct {
	ID        string `gorm:"primaryKey;size:36"`
	ProductID string `gorm:"size:36,"`
}

type Repo struct {
	db *gorm.DB
}

func NewRepo(ctx context.Context, db *gorm.DB) (*Repo, error) {
	if err := db.AutoMigrate(new(FavoriteList), new(FavoriteMember), new(FavoriteProduct)); err != nil {
		return nil, fmt.Errorf("can't create favorites tables: %w", err)
	}

	return &Repo{db: db}, nil
}

func (r *Repo) AddProduct(ctx context.Context, model favorite.Favorite) error {
	r.db.Create(lo.ToPtr(modelToEntity(model)))
	return nil
}

func (r *Repo) CreateList(ctx context.Context, model favorite.List) error {
	r.db.Create(lo.ToPtr(modelToEntity(model)))
}

func (r *Repo) DeleteProduct(ctx context.Context, productID id.ID[product.Product], listID id.ID[favorite.List]) error {
	err := r.queries.DeleteProductByProductID(ctx, sqlgen.DeleteProductByProductIDParams{
		ListID:    listID.String(),
		ProductID: productID.String(),
	})
	if err != nil {
		return fmt.Errorf("can't delete favorite product %s from list %s: %w", productID, listID, err)
	}

	return nil
}

func (r *Repo) AddMember(ctx context.Context, listID id.ID[favorite.List], member favorite.Member) error {
	err := r.queries.InsertMembers(ctx, sqlgen.InsertMembersParams{
		UserID:     member.UserID.String(),
		ListID:     listID.String(),
		CreatedAt:  member.CreatedAt.Time,
		UpdatedAt:  member.UpdatedAt.Time,
		MemberType: int32(member.Type),
	})
	if err != nil {
		return fmt.Errorf("can't add user %s to members of list %s: %w", member.UserID, listID, err)
	}

	return nil
}

func (r *Repo) DeleteMember(ctx context.Context, listID id.ID[favorite.List], userID id.ID[user.User]) error {
	err := r.queries.DeleteMember(ctx, sqlgen.DeleteMemberParams{
		ListID: listID.String(),
		UserID: userID.String(),
	})
	if err != nil {
		return fmt.Errorf("can't delete member %s from list %s: %w", userID, listID, err)
	}

	return nil
}

func (r *Repo) UpdateMember(ctx context.Context, listID id.ID[favorite.List], model favorite.Member) error {
	err := r.queries.UpdateMember(ctx, sqlgen.UpdateMemberParams{
		UpdatedAt:  model.UpdatedAt.Time,
		MemberType: int32(model.Type),
		ListID:     listID.String(),
		UserID:     model.UserID.String(),
	})
	if err != nil {
		return fmt.Errorf("can't update member %s in list %s: %w", model.UserID, listID, err)
	}

	return nil
}

func (r *Repo) GetAndUpdate(ctx context.Context, listID id.ID[favorite.List], f func(favorite.List) (favorite.List, error)) (
	favorite.Favorite,
	error,
) {
	r.db.Tx(ctx, func(ctx context.Context, d *bd.DB) error {
		q := sqlgen.New(d)
	})
}

func entityToModel() {}

func modelToEntity(model favorite.List) FavoriteList {
	entity := FavoriteList{
		ID:        model.ID.String(),
		Type:      int32(model.Type),
		CreatedAt: model.CreatedAt.Time,
		UpdatedAt: model.UpdatedAt.Time,
		Members:   make([]FavoriteMember, 0, len(model.Members)),
		Products:  make([]FavoriteProduct, 0, len(model.Products)),
	}

	for _, member := range model.Members {
		entity.Members = append(entity.Members, FavoriteMember{
			ID:        "",
			UserID:    member.UserID.String(),
			ListID:    model.ID.String(),
			CreatedAt: member.CreatedAt.Time,
			UpdatedAt: member.UpdatedAt.Time,
		})
	}

	for _, product := range model.Products {
		entity.Products = append(entity.Products, FavoriteProduct{
			ID: "",
		})
	}
}
