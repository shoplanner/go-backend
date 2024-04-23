package list

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type repo interface {
	ID(context.Context, uuid.UUID) (ProductListResponse, error)
	Create(context.Context, ProductListResponse) (ProductListResponse, error)
	UserID(context.Context, uuid.UUID) ([]ProductListResponse, error)
	Update(context.Context, ProductListResponse) (ProductListResponse, error)
}

type Service struct {
	repo repo
}

func NewService(repo repo) *Service {
	return &Service{repo: repo}
}

func (s *Service) Create(name string, creatorID uuid.UUID) (ProductListResponse, error) {
	list := ProductListResponse{
		ProductListRequest: ProductListRequest{
			ID:     uuid.New(),
			Name:   name,
			Status: ListStatusPlanning,
		},
		OwnerID:   creatorID,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

}

func (s *Service) Update(list ProductListResponse) (ProductListResponse, error) {
	panic("Not implemented")
}

func (s *Service) Delete(id uuid.UUID) (ProductListResponse, error) {
	panic("Not implemented")

}

func (s *Service) ID(id uuid.UUID) (ProductListResponse, error) {
	panic("Not implemented")
}
