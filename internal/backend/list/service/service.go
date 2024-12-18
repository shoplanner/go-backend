package list

import (
	"context"

	"github.com/google/uuid"

	"go-backend/internal/backend/list"
	"go-backend/internal/backend/user"
	"go-backend/pkg/date"
	"go-backend/pkg/id"
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

func (s *Service) Create(ctx context.Context, name string, creatorID id.ID[user.User]) (list.ProductList, error) {
	list := list.ProductList{
		Options: list.Options{
			States: []list.ProductState{},
			Status: list.ListStatusPlanning,
		},
		OwnerID:   creatorID,
		CreatedAt: date.NewCreateDate[list.ProductList](),
		UpdatedAt: date.NewUpdateDate[list.ProductList](),
	}

	return list, s.repo.Create(ctx, list)
}

func (s *Service) Update(ctx context.Context, model list.Options) (list.ProductList, error) {
	return s.repo.Update(ctx, list.ProductList{Options: model})
}

func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

func (s *Service) ID(ctx context.Context, id uuid.UUID) (list.ProductList, error) {
	return s.repo.ID(ctx, id)
}
