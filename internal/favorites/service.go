package favorites

import (
	"context"
	"slices"
	"time"

	"github.com/google/uuid"

	"go-backend/internal/favorites/models"
	"go-backend/internal/favorites/repo"
	"go-backend/internal/product"
)

type Service struct {
	products *product.Service
	repo     *repo.Repo
}

func NewService() *Service {
	return &Service{}
}

func (s *Service) AddProducts(ctx context.Context, userID uuid.UUID, productIDS []uuid.UUID) error {
	_, err := s.repo.GetAndModify(ctx, userID, func(ctx context.Context, list models.List) (models.List, error) {
		slices.Collect()
		list.Products = append(list.Products, models.Favorite{
			ProductID: productID,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		})
	})
}

func (s *Service) AddList(userID uuid.UUID, models []models.List) error {
	panic("Not implemented")
}

func (s *Service) Delete(userID uuid.UUID, productID uuid.UUID) error {
	panic("Not implemented")
}
