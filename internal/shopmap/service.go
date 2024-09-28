package shopmap

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	productModel "go-backend/internal/product/models"
	"go-backend/internal/shopmap/models"
)

type repo interface {
	Create(ctx context.Context, shopMap models.ShopMap) error
	GetAndUpdate(
		ctx context.Context,
		id uuid.UUID,
		updateFunc func(context.Context, models.ShopMap) (models.ShopMap, error),
	) (models.ShopMap, error)
}

type userService interface {
	IsExists(ctx context.Context, userID uuid.UUID) error
}

type Service struct {
	users     userService
	repo      repo
	log       *zerolog.Logger
	validator *validate.Validator
}

func NewService() *Service {
	return &Service{}
}

func (s *Service) Create(ctx context.Context, ownerID uuid.UUID, categories []productModel.Category) (models.ShopMap, error) {
	newShopMap := models.ShopMap{
		ID:         uuid.New(),
		OwnerID:    ownerID,
		Categories: categories,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	if err := s.users.IsExists(ctx, ownerID); err != nil {
		return models.ShopMap{}, fmt.Errorf("user service returns an error: %w", err)
	}

	if err := s.repo.Create(ctx, newShopMap); err != nil {
		return models.ShopMap{}, err
	}

	return newShopMap, nil
}

func (s *Service) AddViewer(ctx context.Context, mapID uuid.UUID, viewerID uuid.UUID) (models.ShopMap, error) {
	var shopMap models.ShopMap

	err := s.repo.GetAndUpdate(ctx, mapID, func(ctx context.Context, sm models.ShopMap) (models.ShopMap, error) {
		sm.ViewersID = append(sm.ViewersID, viewerID)
		return sm, nil
	})

	return sm, err
}
