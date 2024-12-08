package service

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"go-backend/internal/backend/product"
	"go-backend/internal/backend/shopmap"
	"go-backend/pkg/id"
)

type repo interface {
	Create(ctx context.Context, shopMap shopmap.ShopMap) error
	GetAndUpdate(
		ctx context.Context,
		id uuid.UUID,
		updateFunc func(context.Context, shopmap.ShopMap) (shopmap.ShopMap, error),
	) (shopmap.ShopMap, error)
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
	s.initValidator()
	return s
}

func (s *Service) Create(ctx context.Context, ownerID uuid.UUID, categories []product.Category) (shopmap.ShopMap, error) {
	newShopMap := shopmap.ShopMap{
		ID:         id.NewID[shopmap.ShopMap](),
		OwnerID:    id.ID[user.User](id.NewID[u]()),
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

		if err := s.validator.StructCtx(ctx, sm); err != nil {
			return sm, err
		}

		return sm, nil
	})
}

func (s *Service) RemoveViewerList(ctx context.Context, mapID uuid.UUID, viewerIDs []uuid.UUID) (models.ShopMap, error) {
	return s.repo.GetAndUpdate(ctx, mapID, func(ctx context.Context, shopMap models.ShopMap) (models.ShopMap, error) {
		var errs []error
		toDelete := make(map[uuid.UUID]struct{}, len(viewerIDs))
		for _, viewer := range viewerIDs {
			toDelete[viewer] = struct{}{}
		}

		for _, viewerID := range shopMap.ViewersID {
			if _, ok := toDelete[viewerID]; !ok {
				errs = append(errs, fmt.Errorf("viewer with id %d do not exist", viewerID))
			}
		}

		if len(errs) != 0 {
			return shopMap, errors.Join(errs...)
		}

		shopMap.ViewersID = slices.DeleteFunc(shopMap.ViewersID, func(id uuid.UUID) bool {
			_, deleted := toDelete[id]
			return deleted
		})

		return shopMap, nil
	})
}
