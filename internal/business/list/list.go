package list

import (
	"context"

	"github.com/google/uuid"

	"go-backend/pkg/models"
)

type repo interface {
	ID(context.Context, uuid.UUID) (models.ProductListResponse, error)
	Create(context.Context, models.ProductListRequest) (models.ProductListResponse, error)
	UserID(context.Context, uuid.UUID) ([]models.ProductListResponse, error)
	Update(context.Context, models.ProductListRequest) (models.ProductListResponse, error)
}

type Service struct {
	repo repo
}

func NewService(repo repo) *Service {
	return &Service{repo: repo}
}

func (s *Service) Create(name string) (models.ProductListResponse, error) {

}

func (s *Service) Update(list models.ProductListResponse) (models.ProductListResponse, error) {
	panic("Not implemented")
}

func (s *Service) Delete(id uuid.UUID) (models.ProductListResponse, error) {
	panic("Not implemented")

}

func (s *Service) ID(id uuid.UUID) (models.ProductListResponse, error) {
	panic("Not implemented")
}
