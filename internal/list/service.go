package list

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type repo interface {
	ID(context.Context, uuid.UUID) (ProductListResponse, error)
	Create(context.Context, ProductListResponse) error
	UserID(context.Context, uuid.UUID) ([]ProductListResponse, error)
	Update(context.Context, ProductListResponse) (ProductListResponse, error)
}

type Service struct {
	repo repo
}

func NewService(repo repo) *Service {
	return &Service{repo: repo}
}

func (s *Service) Create(ctx context.Context, name string, creatorID uuid.UUID) (ProductListResponse, error) {
	list := ProductListResponse{
		ProductListRequest: ProductListRequest{
			ID:     uuid.New(),
			Name:   name,
			Status: StatusPlanning,
		},
		OwnerID:   creatorID,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	return list, s.repo.Create(ctx, list)
}

func (s *Service) Update(ctx context.Context, list ProductListRequest) (ProductListResponse, error) {
	model := ProductListResponse{
		ProductListRequest: list,
		UpdatedAt:          time.Now(),
	}
	return s.repo.Update(ctx, model)
}

func (s *Service) Delete(id uuid.UUID) (ProductListResponse, error) {
	panic("Not implemented")
}

func (s *Service) ID(id uuid.UUID) (ProductListResponse, error) {
	panic("Not implemented")
}
