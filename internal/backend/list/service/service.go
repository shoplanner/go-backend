package list

import (
	"context"
	"time"

	"github.com/google/uuid"

	"go-backend/internal/backend/list"
)

type repo interface {
	ID(context.Context, uuid.UUID) (list.ProductList, error)
	Create(context.Context, list.ProductList) error
	UserID(context.Context, uuid.UUID) ([]list.ProductList, error)
	Update(context.Context, list.ProductList) (list.ProductList, error)
	Delete(context.Context, uuid.UUID) error
}

type Service struct {
	repo repo
}

func NewService(repo repo) *Service {
	return &Service{repo: repo}
}

func (s *Service) Create(ctx context.Context, name string, creatorID uuid.UUID) (list.ProductList, error) {
	list := list.ProductList{
		ProductListRequest: list.ProductList{
			ID:     uuid.New(),
			Status: list.StateStatus,
		},
		OwnerID:   creatorID,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	return list, s.repo.Create(ctx, list)
}

func (s *Service) Update(ctx context.Context, list list.ProductList) (list.ProductList, error) {
	model := list.ProductList{
		ProductListRequest: list,
		UpdatedAt:          time.Now(),
	}
	return s.repo.Update(ctx, model)
}

func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

func (s *Service) ID(ctx context.Context, id uuid.UUID) (list.ProductList, error) {
	return s.repo.ID(ctx, id)
}
