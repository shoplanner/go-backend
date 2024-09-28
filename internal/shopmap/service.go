package shopmap

import (
	"context"
	"fmt"
	"time"

	"github.com/go-playground/validator/v10"
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
	validator *validator.Validate
}

func NewService() *Service {
	s := &Service{validator: validator.New()}
	s.validator.RegisterValidation("user_id_valid", s.checkUserExist)
	return s
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

func (s *Service) AddViewerList(ctx context.Context, mapID uuid.UUID, viewerIDs []uuid.UUID) (models.ShopMap, error) {
	return s.repo.GetAndUpdate(ctx, mapID, func(ctx context.Context, sm models.ShopMap) (models.ShopMap, error) {
		sm.ViewersID = append(sm.ViewersID, viewerIDs...)

		if err := s.validateMap(sm); err != nil {
			return sm, err
		}

		return sm, nil
	})
}

func (s *Service) validateMap(ctx context.Context, shopMap models.ShopMap) error {
	if err := s.validator.VarCtx(ctx, shopMap.Categories, "unique"); err != nil {
		return err
	}
	if err := s.validator.VarCtx(ctx, shopMap.ViewersID, "unique"); err != nil {
		return err
	}
}
