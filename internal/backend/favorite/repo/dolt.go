package repo

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/samber/lo"
	"github.com/samber/mo"
	"gorm.io/gorm"

	"go-backend/internal/backend/favorite"
	"go-backend/internal/backend/product"
	"go-backend/internal/backend/product/repo"
	"go-backend/internal/backend/user"
	userRepo "go-backend/internal/backend/user/repo"
	"go-backend/pkg/date"
	"go-backend/pkg/god"
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
	ID             string        `gorm:"primaryKey;size:36;notNull"`
	UserID         string        `gorm:"size:36;references:ID;notNull"`
	User           userRepo.User `gorm:"references:ID"`
	FavoriteListID string        `gorm:"size:36;references:ID;notNull"`
	CreatedAt      time.Time     `gorm:"notNull"`
	UpdatedAt      time.Time     `gorm:"notNull"`
	MemberType     int32         `gorm:"notNull"`
}

type FavoriteProduct struct {
	ID             string       `gorm:"primaryKey;size:36;notNull"`
	ProductID      string       `gorm:"size:36;notNull;uniqueIndex:idx_favorite_product"`
	FavoriteListID string       `gorm:"size:36;notNull;uniqueIndex:idx_favorite_product"`
	Product        repo.Product `gorm:"references:ID"`
	CreatedAt      time.Time    `gorm:"notNull"`
	UpdatedAt      time.Time    `gorm:"notNull"`
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

func (r *Repo) CreateList(ctx context.Context, model favorite.List) error {
	if err := r.db.WithContext(ctx).Create(lo.ToPtr(modelToEntity(model))).Error; err != nil {
		return fmt.Errorf("can't create new list %s: %w", model.ID, err)
	}
	return nil
}

func (r *Repo) DeleteList(ctx context.Context, listID id.ID[favorite.List]) error {
	if err := r.db.WithContext(ctx).Delete(&FavoriteList{ID: listID.String()}).Error; err != nil { // nolint:exhaustruct
		return fmt.Errorf("can't delete favorites list %s: %w", listID, err)
	}

	return nil
}

func (r *Repo) GetByID(ctx context.Context, listID id.ID[favorite.List]) (favorite.List, error) {
	entity := FavoriteList{ID: listID.String()} // nolint:exhaustruct
	err := r.db.WithContext(ctx).
		Preload("Members").
		Preload("Products").
		Preload("Products.Product").
		Preload("Products.Product.Category").
		Preload("Products.Product.Forms").
		First(&entity).Error
	if err != nil {
		return favorite.List{}, fmt.Errorf("can't select favorites list %s: %w", listID, err)
	}

	return entityToModel(entity), nil
}

func (r *Repo) GetByUserID(ctx context.Context, userID id.ID[user.User]) ([]favorite.List, error) {
	var entities []FavoriteList
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var listIDs []string

		err := tx.WithContext(ctx).
			Model(new(FavoriteMember)).
			Where(&FavoriteMember{UserID: userID.String()}). // nolint:exhaustruct
			Pluck("favorite_list_id", &listIDs).
			Error
		if err != nil {
			return fmt.Errorf("can't get list ids of user %s: %w", userID, err)
		}

		err = tx.WithContext(ctx).Where("id in ?", listIDs).
			Preload("Members").
			Preload("Products").
			Preload("Products.Product").
			Preload("Products.Product.Category").
			Preload("Products.Product.Forms").
			Find(&entities).Error
		if err != nil {
			return fmt.Errorf("can't get lists of user %s: %w", userID, err)
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("transaction failed: %w", err)
	}

	return lo.Map(entities, func(item FavoriteList, _ int) favorite.List { return entityToModel(item) }), nil
}

func (r *Repo) GetListsByMembership(
	ctx context.Context,
	userID id.ID[user.User],
	memberType favorite.MemberType,
) (
	[]favorite.List,
	error,
) {
	var entities []FavoriteList
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var listIDs []string

		err := tx.WithContext(ctx).
			Where(&FavoriteMember{UserID: userID.String(), MemberType: int32(memberType)}). // nolint:exhaustruct
			Pluck("list_id", &listIDs).
			Error
		if err != nil {
			return fmt.Errorf("can't get list ids of user %s: %w", userID, err)
		}

		err = tx.WithContext(ctx).Where("id in ?", listIDs).Find(&entities).Error
		if err != nil {
			return fmt.Errorf("can't get lists of user %s: %w", userID, err)
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("transaction failed: %w", err)
	}

	return lo.Map(entities, func(item FavoriteList, _ int) favorite.List { return entityToModel(item) }), nil
}

func (r *Repo) GetAndUpdate(
	ctx context.Context,
	listID id.ID[favorite.List],
	f func(favorite.List) (favorite.List, error),
) (
	favorite.List,
	error,
) {
	var model favorite.List
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var err error

		entity := FavoriteList{ID: listID.String()} // nolint:exhaustruct
		err = tx.WithContext(ctx).
			Preload("Members").
			Preload("Products").
			Preload("Products.Product").
			First(&entity).Error
		if err != nil {
			return fmt.Errorf("can't select favorites list %s: %w", listID, err)
		}

		model, err = f(entityToModel(entity))
		if err != nil {
			return err
		}

		entity = modelToEntity(model)
		query := &FavoriteList{ID: listID.String()} //nolint:exhaustruct

		err = tx.WithContext(ctx).
			Model(query).
			Association("Members").
			Unscoped().
			Replace(entity.Members)
		if err != nil {
			return fmt.Errorf("can't update favorites list %s members: %w", listID, err)
		}

		err = tx.WithContext(ctx).
			Model(query).
			Association("Products").
			Unscoped().
			Replace(entity.Products)
		if err != nil {
			return fmt.Errorf("can't update favorites list %s products: %w", listID, err)
		}

		err = tx.WithContext(ctx).
			Model(query).
			Updates(&entity).
			Error
		if err != nil {
			return fmt.Errorf("can't update favorites list %s: %w", listID, err)
		}

		return nil
	})
	if err != nil {
		return model, fmt.Errorf("transaction failed: %w", err)
	}

	return model, nil
}

func entityToModel(entity FavoriteList) favorite.List {
	model := favorite.List{
		ID:        id.ID[favorite.List]{UUID: god.Believe(uuid.Parse(entity.ID))},
		CreatedAt: date.CreateDate[favorite.List]{Time: entity.CreatedAt},
		UpdatedAt: date.UpdateDate[favorite.List]{Time: entity.UpdatedAt},
		Members:   make([]favorite.Member, 0, len(entity.Members)),
		Products:  make([]favorite.Favorite, 0, len(entity.Products)),
		Type:      favorite.ListType(entity.ListType),
	}

	for _, member := range entity.Members {
		model.Members = append(model.Members, entityToMember(member))
	}
	for _, product := range entity.Products {
		model.Products = append(model.Products, entityToProduct(product))
	}

	return model
}

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
		ID:             uuid.NewString(),
		UserID:         member.UserID.String(),
		User:           userRepo.User{ID: "", Login: "", Hash: "", Role: 0},
		FavoriteListID: listID.String(),
		CreatedAt:      member.CreatedAt.Time,
		UpdatedAt:      member.UpdatedAt.Time,
		MemberType:     int32(member.Type),
	}
}

func productToEntity(listID id.ID[favorite.List], model favorite.Favorite) FavoriteProduct {
	return FavoriteProduct{
		ID:        uuid.NewString(),
		ProductID: model.Product.ID.String(),
		Product: repo.Product{
			ID:         model.Product.ID.String(),
			CreatedAt:  time.Time{},
			UpdatedAt:  time.Time{},
			Name:       "",
			CategoryID: sql.NullString{String: "", Valid: false},
			Category:   &repo.ProductCategory{ID: "", Name: ""},
			Forms:      []repo.ProductForm{},
		},
		FavoriteListID: listID.String(),
		CreatedAt:      model.CreatedAt.Time,
		UpdatedAt:      model.UpdatedAt.Time,
	}
}

func entityToMember(entity FavoriteMember) favorite.Member {
	return favorite.Member{
		UserID:    id.ID[user.User]{UUID: god.Believe(uuid.Parse(entity.UserID))},
		Type:      favorite.MemberType(entity.MemberType),
		CreatedAt: date.CreateDate[favorite.Member]{Time: entity.CreatedAt},
		UpdatedAt: date.UpdateDate[favorite.Member]{Time: entity.UpdatedAt},
	}
}

func entityToProduct(entity FavoriteProduct) favorite.Favorite {
	category := ""
	if entity.Product.Category != nil {
		category = entity.Product.Category.Name
	}

	return favorite.Favorite{
		Product: product.Product{
			Options: product.Options{
				Name:     product.Name(entity.Product.Name),
				Category: mo.EmptyableToOption(product.Category(category)),
				Forms: lo.Map(entity.Product.Forms, func(item repo.ProductForm, _ int) product.Form {
					return product.Form(item.Name)
				}),
			},
			ID:        id.ID[product.Product]{UUID: god.Believe(uuid.Parse(entity.ProductID))},
			CreatedAt: date.CreateDate[product.Product]{Time: entity.Product.CreatedAt},
			UpdatedAt: date.UpdateDate[product.Product]{Time: entity.Product.UpdatedAt},
		},
		CreatedAt: date.CreateDate[favorite.Favorite]{Time: entity.CreatedAt},
		UpdatedAt: date.UpdateDate[favorite.Favorite]{Time: entity.UpdatedAt},
	}
}
