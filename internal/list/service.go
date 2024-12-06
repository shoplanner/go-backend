package list

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/samber/lo"

	"go-backend/internal/list/models"
)

type repo interface {
	ID(context.Context, uuid.UUID) (models.ProductList, error)
	Create(context.Context, models.ProductList) error
	UserID(context.Context, uuid.UUID) ([]models.ProductList, error)
	Update(context.Context, models.ProductList) (models.ProductList, error)
	Delete(context.Context, uuid.UUID) error
}

type Service struct {
	repo repo
}

func NewService(repo repo) *Service {
	return &Service{repo: repo}
}

func (s *Service) Create(ctx context.Context, name string, creatorID uuid.UUID) (models.ProductList, error) {
	list := models.ProductList{
		ProductListRequest: models.ProductList{
			ID:     uuid.New(),
			Status: models.StateStatus,
		},
		OwnerID:   creatorID,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	lo.SomeBy()
	return list, s.repo.Create(ctx, list)
}

func (s *Service) Update(ctx context.Context, list models.ProductListRequest) (models.ProductListResponse, error) {
	model := models.ProductList{
		ProductListRequest: list,
		UpdatedAt:          time.Now(),
	}
	return s.repo.Update(ctx, model)
}

func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

func (s *Service) ID(ctx context.Context, id uuid.UUID) (models.ProductListResponse, error) {
	return s.repo.ID(ctx, id)
}
