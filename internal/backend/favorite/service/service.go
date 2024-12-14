package service

import (
	"context"
	"time"

	"github.com/google/uuid"

	"go-backend/internal/backend/favorite"
	"go-backend/internal/backend/favorite/repo"
	"go-backend/internal/backend/product"
	"go-backend/internal/backend/user"
)

type productService interface{}

type favoritesRepo interface {
	Get(ctx context.Context, userID uuid.UUID)
	Set(ctx context.Context, list favorite.Favorite)
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
	repo     *repo.Repo
}

func NewService() *Service {
	return &Service{}
}

func (s *Service) AddProducts(ctx context.Context, userID uuid.UUID, productIDS []uuid.UUID) error {
	_, err := s.repo.GetAndModify(ctx, userID, func(ctx context.Context, list favorite.List) (favorite.List, error) {
		list.Products = append(list.Products, favorite.Favorite{
			ProductID: productID,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		})
	})
	if err != nil {
		return err
	}
}

func (s *Service) AddList(userID uuid.UUID, models []models.List) error {
}

func (s *Service) Delete(userID uuid.UUID, productID uuid.UUID) error {
	panic("Not implemented")
}
