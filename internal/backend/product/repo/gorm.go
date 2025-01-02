package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/samber/lo"
	"github.com/samber/mo"
	"gorm.io/gorm"

	"go-backend/internal/backend/product"
	"go-backend/pkg/date"
	"go-backend/pkg/god"
	"go-backend/pkg/id"
)

type Product struct {
	ID         string           `gorm:"primaryKey;size:36"`
	CreatedAt  time.Time        `gorm:"notNull"`
	UpdatedAt  time.Time        `gorm:"notNull"`
	Name       string           `gorm:"size:256;notNull"`
	CategoryID sql.NullString   `gorm:"size:36"`
	Category   *ProductCategory `gorm:"references:ID"`
	Forms      []ProductForm
}

func (c *Product) BeforeSave(_ *gorm.DB) error {
	if !c.CategoryID.Valid {
		return nil
	}
	if c.Category == nil {
		return nil
	}

	c.CategoryID = sql.NullString{String: c.Category.Name, Valid: true}
	return nil
}

type ProductCategory struct {
	ID   string `gorm:"primaryKey;size:36"`
	Name string `gorm:"primaryKey;size:255"`
}

func (c *ProductCategory) BeforeSave(tx *gorm.DB) error {
	var existed ProductCategory
	err := tx.First(&existed, "name = ?", c.Name).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.ID = uuid.NewString()
		return nil
	} else if err != nil {
		return err
	}

	c.ID = existed.ID
	return nil
}

type ProductForm struct {
	ProductID string `gorm:"size:36;references:ID"`
	ID        string `gorm:"primaryKey,size:36"`
	Name      string `gorm:"size:255"`
}

type GormRepo struct {
	db *gorm.DB
}

func NewGormRepo(ctx context.Context, db *gorm.DB) (*GormRepo, error) {
	err := db.WithContext(ctx).AutoMigrate(new(ProductCategory), new(Product), new(ProductForm))
	if err != nil {
		return nil, fmt.Errorf("can't initialize product tables: %w", err)
	}

	return &GormRepo{db: db}, nil
}

func (r *GormRepo) GetAndUpdate(
	ctx context.Context,
	productID id.ID[product.Product],
	updateFunc func(product.Product) (product.Product, error),
) (
	product.Product,
	error,
) {
	var model product.Product
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var entity Product
		err := tx.WithContext(ctx).First(&entity, productID.UUID).Error
		if err != nil {
			return wrapErr(fmt.Errorf("can't get product %s: %w", productID, err))
		}

		model, err = updateFunc(entityToModel(entity))
		if err != nil {
			return err
		}
		entity = modelToEntity(model)

		err = tx.WithContext(ctx).Session(&gorm.Session{FullSaveAssociations: true}).Model(&Product{ID: entity.ID}).
			Association("Forms").Unscoped().Replace(entity.Forms)
		if err != nil {
			return fmt.Errorf("can't update product %s associations: %w", productID, err)
		}
		err = tx.WithContext(ctx).Save(&entity).Error
		if err != nil {
			return fmt.Errorf("can't update product %s: %w", productID, err)
		}

		return nil
	})
	if err != nil {
		return model, wrapErr(fmt.Errorf("transaction failed: %w", err))
	}

	return model, nil
}

func (r *GormRepo) GetByID(ctx context.Context, productID id.ID[product.Product]) (product.Product, error) {
	var entity Product
	err := r.db.WithContext(ctx).Preload("Category").Preload("Forms").
		First(&entity, "id = ?", productID.String()).Error
	if err != nil {
		return product.Product{}, wrapErr(fmt.Errorf("can't get product %s: %w", productID, err))
	}

	return entityToModel(entity), nil
}

func (r *GormRepo) GetByListID(ctx context.Context, idList []id.ID[product.Product]) ([]product.Product, error) {
	var entities []Product

	uuids := lo.Map(idList, func(item id.ID[product.Product], _ int) uuid.UUID { return item.UUID })

	err := r.db.WithContext(ctx).Preload("Forms").Find(&entities, uuids).Error
	if err != nil {
		return nil, wrapErr(fmt.Errorf("can't select products %v: %w", idList, err))
	}

	return lo.Map(entities, func(item Product, _ int) product.Product { return entityToModel(item) }), nil
}

func (r *GormRepo) Create(ctx context.Context, model product.Product) error {
	entity := modelToEntity(model)
	log.Info().Any("model", model).Any("entity", entity).Msg("inserting new product")
	err := r.db.WithContext(ctx).Create(&entity).Error
	if err != nil {
		return wrapErr(fmt.Errorf("can't update product %s: %w", model.ID, err))
	}

	return nil
}

func (r *GormRepo) Delete(ctx context.Context, productID id.ID[product.Product]) error {
	err := r.db.WithContext(ctx).Delete(new(Product), productID.UUID[:]).Error
	if err != nil {
		return wrapErr(fmt.Errorf("can't delete product %s: %w", productID, err))
	}

	return nil
}

func entityToModel(entity Product) product.Product {
	category := mo.None[product.Category]()
	if entity.Category != nil {
		category = mo.EmptyableToOption(product.Category(entity.Category.Name))
	}

	return product.Product{
		Options: product.Options{
			Name:     product.Name(entity.Name),
			Category: category,
			Forms: lo.Map(entity.Forms, func(item ProductForm, _ int) product.Form {
				return product.Form(item.Name)
			}),
		},
		ID:        id.ID[product.Product]{UUID: god.Believe(uuid.Parse(entity.ID))},
		CreatedAt: date.CreateDate[product.Product]{Time: entity.CreatedAt},
		UpdatedAt: date.UpdateDate[product.Product]{Time: entity.UpdatedAt},
	}
}

func modelToEntity(model product.Product) Product {
	var category *ProductCategory
	if model.Category.IsPresent() {
		category = &ProductCategory{
			ID:   "", // will be set in hooks
			Name: string(model.Category.OrEmpty()),
		}
	}

	return Product{
		ID:         model.ID.String(),
		CreatedAt:  model.CreatedAt.Time,
		UpdatedAt:  model.UpdatedAt.Time,
		Name:       string(model.Name),
		CategoryID: sql.NullString{String: "", Valid: false}, // will be set in hooks
		Category:   category,
		Forms: lo.Map(model.Forms, func(item product.Form, _ int) ProductForm {
			return ProductForm{
				ProductID: model.ID.String(),
				ID:        uuid.NewString(),
				Name:      string(item),
			}
		}),
	}
}

func wrapErr(err error) error {
	if err != nil {
		return fmt.Errorf("product storage: %w", err)
	}
	return nil
}
