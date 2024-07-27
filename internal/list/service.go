package list

import (
	"context"
	"time"

	"github.com/google/uuid"

	"go-backend/internal/list/models"
)

type repo interface {
	ID(context.Context, uuid.UUID) (models.ProductListResponse, error)
	Create(context.Context, models.ProductListResponse) error
	UserID(context.Context, uuid.UUID) ([]models.ProductListResponse, error)
	Update(context.Context, models.ProductListResponse) (models.ProductListResponse, error)
	Delete(context.Context, uuid.UUID) error
}

type Service struct {
	repo repo
}

func NewService(repo repo) *Service {
	return &Service{repo: repo}
}

func (s *Service) Create(ctx context.Context, name string, creatorID uuid.UUID) (models.ProductListResponse, error) {
	list := models.ProductListResponse{
		ProductListRequest: models.ProductListRequest{
			ID:     uuid.New(),
			Name:   name,
			Status: models.StatusPlanning,
		},
		OwnerID:   creatorID,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	return list, s.repo.Create(ctx, list)
}

func (s *Service) Update(ctx context.Context, list models.ProductListRequest) (models.ProductListResponse, error) {
	model := models.ProductListResponse{
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
