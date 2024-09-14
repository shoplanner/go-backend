package favorites

import (
	"context"
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

func (s *Service) AddProduct(ctx context.Context, userID uuid.UUID, productID uuid.UUID) error {
	found, err := s.products.IsExist(ctx, productID)
	if err != nil {
		return err
	}
	if !found {
	}

	_, err = s.repo.GetAndModify(ctx, userID, func(ctx context.Context, list models.List) (models.List, error) {
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
