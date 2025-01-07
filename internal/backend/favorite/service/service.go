package service

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/samber/lo"
	"github.com/samber/mo"

	"go-backend/internal/backend/favorite"
	"go-backend/internal/backend/product"
	"go-backend/internal/backend/user"
	"go-backend/pkg/date"
	"go-backend/pkg/id"
	"go-backend/pkg/myerr"
	"go-backend/pkg/ph"
)

type favoritesRepo interface {
	GetByID(context.Context, id.ID[favorite.List]) (favorite.List, error)
	GetByUserID(context.Context, id.ID[user.User]) ([]favorite.List, error)
	GetAndUpdate(context.Context, id.ID[user.User], func(list favorite.List) (favorite.List, error)) (
		favorite.List,
		error,
	)
}

type Service struct {
	repo favoritesRepo
}

func NewService(repo favoritesRepo) *Service {
	return &Service{repo: repo}
}

func (s *Service) AddProducts(
	ctx context.Context,
	listID id.ID[favorite.List],
	userID id.ID[user.User],
	productIDs []id.ID[product.Product],
) (
	favorite.List,
	error,
) {
	model, err := s.repoGetAndUpdate(ctx, listID, userID, func(list favorite.List) (favorite.List, error) {
		for _, productID := range productIDs {
			list.Products = append(list.Products, favorite.Favorite{
				ListID: listID,
				Product: product.Product{
					Options:   product.Options{Name: "", Category: mo.None[product.Category](), Forms: []product.Form{}},
					ID:        productID,
					CreatedAt: date.CreateDate[product.Product]{Time: time.Time{}},
					UpdatedAt: date.UpdateDate[product.Product]{Time: time.Time{}},
				},
				CreatedAt: date.NewCreateDate[favorite.Favorite](),
				UpdatedAt: date.NewUpdateDate[favorite.Favorite](),
			})
		}
		return list, nil
	})
	if err != nil {
		return model, err
	}

	return model, nil
}

func (s *Service) DeleteProducts(ctx context.Context,
	listID id.ID[favorite.List],
	userID id.ID[user.User],
	productIDs []id.ID[product.Product],
) (
	favorite.List,
	error,
) {
	model, err := s.repoGetAndUpdate(ctx, listID, userID, func(list favorite.List) (favorite.List, error) {
		productExists := make(map[string]int, len(list.Products))
		for i, product := range list.Products {
			productExists[product.Product.ID.String()] = i
		}

		for _, productID := range productIDs {
			if _, exists := productExists[productID.String()]; !exists {
				return list, favorite.ErrProductNotFound(listID, productID)
			}
			delete(productExists, productID.String())
		}

		list.Products = make([]favorite.Favorite, len(productExists))

		return list, nil
	})
	if err != nil {
		return model, err
	}

	return model, nil
}

func (s *Service) GetListByID(
	ctx context.Context,
	listID id.ID[favorite.List],
	userID id.ID[user.User],
) (
	favorite.List,
	error,
) {
	model, err := s.repo.GetByID(ctx, listID)
	if err != nil {
		return favorite.List{}, fmt.Errorf("can't get list %s: %w", listID, err)
	}

	if err := model.AllowedToView(userID); err != nil {
		return favorite.List{}, err
	}

	return model, nil
}

func (s *Service) GetListsByUserID(ctx context.Context, userID id.ID[user.User]) ([]favorite.List, error) {
	models, err := s.repo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("can't get favorites lists of user %s: %w", userID, err)
	}

	return models, nil
}

func (s *Service) repoGetAndUpdate(
	ctx context.Context,
	listID id.ID[favorite.List],
	userID id.ID[user.User],
	updateFunc func(favorite.List) (favorite.List, error),
) (favorite.List, error) {
	model, err := s.repo.GetAndUpdate(ctx, userID, updateFunc)
	if err != nil {
		return model, fmt.Errorf("can't update repo: %w", err)
	}
	return model, nil
}
