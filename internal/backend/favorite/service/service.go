package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/samber/mo"

	"go-backend/internal/backend/favorite"
	"go-backend/internal/backend/product"
	"go-backend/internal/backend/user"
	"go-backend/pkg/date"
	"go-backend/pkg/id"
)

type favoritesRepo interface {
	Get(ctx context.Context, userID uuid.UUID) (favorite.List, error)
	AddFavorite(context.Context, favorite.Favorite) (favorite.List, error)
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

		if list

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

func (s *Service) Delete(userID uuid.UUID, productID uuid.UUID) error {
	panic("Not implemented")
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
