package product

import (
	"context"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"go-backend/pkg/models"
)

type repo interface {
	ID(context.Context, uuid.UUID) (models.ProductResponse, error)
	IDList(context.Context, []uuid.UUID) ([]models.ProductResponse, error)
	Create(context.Context, models.ProductResponse) error
	Delete(context.Context, uuid.UUID) (models.ProductResponse, error)
	Update(context.Context, models.ProductResponse) (models.ProductResponse, error)
}

type Service struct {
	repo repo
	log  zap.SugaredLogger
}

func NewService(repo repo) *Service {
	return &Service{repo: repo}
}

func (s *Service) ID(ctx context.Context, id uuid.UUID) (models.ProductResponse, error) {
	return s.repo.ID(ctx, id)
}

func (s *Service) IDList(ctx context.Context, ids []uuid.UUID) ([]models.ProductResponse, error) {
	return s.repo.IDList(ctx, ids)
}

func (s *Service) Create(ctx context.Context, product models.ProductRequest) (models.ProductResponse, error) {
	full := models.ProductResponse{
		ProductRequest: product,
		ID:             uuid.New(),
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	return full, s.repo.Create(ctx, full)
}

func (s *Service) Update(ctx context.Context, id uuid.UUID, product models.ProductRequest) (models.ProductResponse, error) {
	full := models.ProductResponse{
		ProductRequest: product,
		ID:             id,
		UpdatedAt:      time.Now(),
	}

	return s.repo.Update(ctx, full)
}
