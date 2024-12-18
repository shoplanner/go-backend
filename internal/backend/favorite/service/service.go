package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"go-backend/internal/backend/favorite"
	"go-backend/internal/backend/product"
	"go-backend/internal/backend/user"
	"go-backend/pkg/id"
)

type favoritesRepo interface {
	Get(ctx context.Context, userID uuid.UUID)
	Set(ctx context.Context, list favorite.Favorite)
	GetAndUpdate(context.Context, id.ID[user.User], func(list favorite.List) (favorite.List, error)) (
		favorite.List,
		error,
	)
}

type userService interface {
	GetUser(context.Context, uuid.UUID) (user.User, error)
	IsUserIdValid(context.Context) (bool, error)
}

type productService interface {
	IsExists() error
}

type Service struct {
	users    userService
	products productService
	repo     favoritesRepo
}

func NewService(users userService, productService productService, repo favoritesRepo) *Service {
	return &Service{
		users:    users,
		products: productService,
		repo:     repo,
	}
}

func (s *Service) AddProducts(ctx context.Context, userID id.ID[user.User], productIDs []id.ID[product.Product]) (
	favorite.List,
	error,
) {
	model, err := s.repoGetAndUpdate(ctx, userID, func(list favorite.List) (favorite.List, error) {
		for _, productID := range productIDs {
			list.Products = append(list.Products, favorite.Favorite{
				ProductID: productID,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
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
	userID id.ID[user.User],
	updateFunc func(favorite.List) (favorite.List, error),
) (favorite.List, error) {
	model, err := s.repo.GetAndUpdate(ctx, userID, updateFunc)
	if err != nil {
		return model, fmt.Errorf("can't update repo: %w", err)
	}
	return model, nil
}
