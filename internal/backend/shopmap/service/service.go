package service

import (
	"context"
	"errors"
	"fmt"
	"go-backend/internal/backend/product"
	"go-backend/internal/backend/shopmap"
	"go-backend/internal/backend/user"
	"go-backend/pkg/date"
	"go-backend/pkg/id"
	"go-backend/pkg/myerr"
	"go-backend/pkg/ph"
	"maps"
	"slices"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/samber/lo"
)

type repo interface {
	Create(ctx context.Context, shopMap shopmap.ShopMap) error
	GetAndUpdate(
		ctx context.Context,
		id id.ID[shopmap.ShopMap],
		updateFunc func(context.Context, shopmap.ShopMap) (shopmap.ShopMap, error),
	) (shopmap.ShopMap, error)
	Delete(context.Context, id.ID[shopmap.ShopMap]) (shopmap.ShopMap, error)
	GetByID(context.Context, id.ID[shopmap.ShopMap]) (shopmap.ShopMap, error)
	GetByUserID(context.Context, id.ID[user.User]) ([]shopmap.ShopMap, error)
}

type userService interface {
	IsExists(ctx context.Context, userID uuid.UUID) error
}

type Service struct {
	users     userService
	repo      repo
	validator *validator.Validate
}

func NewService(userService userService, repo repo) *Service {
	s := &Service{
		users:     userService,
		repo:      repo,
		validator: validator.New(),
	}
	s.initValidator()
	return s
}

func (s *Service) Create(ctx context.Context, ownerID id.ID[user.User], cfg shopmap.Options) (shopmap.ShopMap, error) {
	shopMap := shopmap.ShopMap{
		Options:   cfg,
		ID:        id.NewID[shopmap.ShopMap](),
		OwnerID:   ownerID,
		CreatedAt: date.NewCreateDate[shopmap.ShopMap](),
		UpdatedAt: date.NewUpdateDate[shopmap.ShopMap](),
	}

	if err := s.validate(ctx, shopMap); err != nil {
		return shopmap.ShopMap{}, err
	}

	return shopMap, s.repoCreate(ctx, shopMap)
}

func (s *Service) AddViewerList(ctx context.Context, mapID id.ID[shopmap.ShopMap], viewerIDs []id.ID[user.User]) (shopmap.ShopMap, error) {
	return s.repoGetAndUpdate(ctx, mapID, func(ctx context.Context, shopMap shopmap.ShopMap) (shopmap.ShopMap, error) {
		shopMap.ViewerIDList = append(shopMap.ViewerIDList, viewerIDs...)

		if err := s.validate(ctx, shopMap); err != nil {
			return shopMap, err
		}

		return shopMap, nil
	})
}

func (s *Service) RemoveViewerList(
	ctx context.Context,
	mapID id.ID[shopmap.ShopMap],
	viewerIDs []id.ID[user.User],
) (shopmap.ShopMap, error) {
	return s.repoGetAndUpdate(ctx, mapID, func(ctx context.Context, shopMap shopmap.ShopMap) (shopmap.ShopMap, error) {
		var errs []error

		toDelete := lo.SliceToMap(viewerIDs, ph.EmptyStruct)
		if _, found := toDelete[shopMap.OwnerID]; found {
			return shopMap, errors.New("can't delete owner")
		}

		for _, viewerID := range shopMap.ViewerIDList {
			if _, ok := toDelete[viewerID]; !ok {
				errs = append(errs, fmt.Errorf("viewer with id %d do not exist", viewerID))
			}
		}

		if len(errs) != 0 {
			return shopMap, errors.Join(errs...)
		}

		shopMap.ViewerIDList = slices.DeleteFunc(shopMap.ViewerIDList, func(viewerID id.ID[user.User]) bool {
			return lo.HasKey(toDelete, viewerID)
		})

		return shopMap, s.validate(ctx, shopMap)
	})
}

func (s *Service) DeleteMap(ctx context.Context, mapID id.ID[shopmap.ShopMap], userID id.ID[user.User]) (shopmap.ShopMap, error) {
	return s.repoDelete(ctx, mapID)
}

func (s *Service) UpdateMap(ctx context.Context, mapID id.ID[shopmap.ShopMap], userId id.ID[user.User], cfg shopmap.Options) (shopmap.ShopMap, error) {
	return s.repoGetAndUpdate(ctx, mapID, func(ctx context.Context, shopMap shopmap.ShopMap) (shopmap.ShopMap, error) {
		shopMap.Options = cfg
		return shopMap, s.validate(ctx, shopMap)
	})
}

func (s *Service) GetByID(ctx context.Context, mapID id.ID[shopmap.ShopMap]) (shopmap.ShopMap, error) {
	return s.repoGet(ctx, mapID)
}

func (s *Service) GetByUserID(ctx context.Context, userID id.ID[user.User]) ([]shopmap.ShopMap, error) {
	return s.repoGetByUser(ctx, userID)
}

func (s *Service) ReorderMap(ctx context.Context, mapID id.ID[shopmap.ShopMap], userID id.ID[user.User], categories []product.Category) (shopmap.ShopMap, error) {
	return s.repoGetAndUpdate(ctx, mapID, func(ctx context.Context, shopMap shopmap.ShopMap) (shopmap.ShopMap, error) {
		if maps.Equal(lo.CountValues(shopMap.CategoryList), lo.CountValues(categories)) {
			return shopMap, fmt.Errorf("%w: only order changes accepted", myerr.ErrInvalidArgument)
		}

		shopMap.CategoryList = categories

		return shopMap, nil
	})
}

func (s *Service) repoCreate(ctx context.Context, shopMap shopmap.ShopMap) error {
	err := s.repo.Create(ctx, shopMap)
	if err != nil {
		return fmt.Errorf("can't create new shop map: %w", err)
	}

	return nil
}

func (s *Service) repoDelete(ctx context.Context, mapID id.ID[shopmap.ShopMap]) (shopmap.ShopMap, error) {
	shopMap, err := s.repo.Delete(ctx, mapID)
	if err != nil {
		return shopMap, fmt.Errorf("can't delete shop map %s: %w", mapID.String(), err)
	}
	return shopMap, nil
}

func (s *Service) repoGetAndUpdate(
	ctx context.Context,
	mapID id.ID[shopmap.ShopMap],
	updateFunc func(context.Context, shopmap.ShopMap) (shopmap.ShopMap, error),
) (shopmap.ShopMap, error) {
	shopMap, err := s.repo.GetAndUpdate(ctx, mapID, func(ctx context.Context, sm shopmap.ShopMap) (shopmap.ShopMap, error) {
		sm, err := updateFunc(ctx, sm)
		if err != nil {
			return sm, err
		}
		sm.UpdatedAt.Update()
		return sm, nil
	})
	if err != nil {
		return shopMap, fmt.Errorf("shop map service: can't update shop map %s: %w", mapID, err)
	}

	return shopMap, nil
}

func (s *Service) repoGet(ctx context.Context, mapID id.ID[shopmap.ShopMap]) (shopmap.ShopMap, error) {
	shopMap, err := s.repo.GetByID(ctx, mapID)
	if err != nil {
		return shopMap, wrapErr(fmt.Errorf("can't get shop map %s: %w", mapID, err))
	}
	return shopMap, nil
}

func (s *Service) repoGetByUser(ctx context.Context, userID id.ID[user.User]) ([]shopmap.ShopMap, error) {
	shopMapList, err := s.repo.GetByUserID(ctx, userID)
	if err != nil {
		return shopMapList, wrapErr(fmt.Errorf(
			"can't get shop map list for user %s: %w",
			userID,
			err,
		))
	}
	return shopMapList, nil
}

func wrapErr(err error) error {
	if err != nil {
		return fmt.Errorf("shop map service: %w", err)
	}

	return nil
}
