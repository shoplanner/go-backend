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
	return nil
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

func (s *Service) Update() {
}
