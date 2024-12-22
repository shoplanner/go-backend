package service

import (
	"context"
	"sync"

	"github.com/samber/lo"
	"go.uber.org/zap"

	"go-backend/internal/backend/product"
	"go-backend/pkg/date"
	"go-backend/pkg/id"
)

type repo interface {
	GetByIDList(context.Context, []id.ID[product.Product]) ([]product.Product, error)
	Create(context.Context, product.Product) error
	Update(context.Context, product.Product) error
}

type Service struct {
	repo repo
	log  zap.SugaredLogger
	lock sync.RWMutex
}

func NewService(repo repo) *Service {
	return &Service{repo: repo, log: *zap.NewNop().Sugar().Named("")}
}

func (s *Service) ID(ctx context.Context, productID id.ID[product.Product]) (product.Product, error) {
	var model []product.Product
	var err error
	lo.Synchronize(&s.lock).Do(func() {
		model, err = s.repo.GetByIDList(ctx, []id.ID[product.Product]{productID})
	})

	return model[0], err
}

func (s *Service) IDList(ctx context.Context, ids []id.ID[product.Product]) ([]product.Product, error) {
	s.lock.RLock()
	defer s.lock.RUnlock()

	return s.repo.GetByIDList(ctx, ids)
}

func (s *Service) Create(ctx context.Context, options product.Options) (product.Product, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	full := product.Product{
		Options:   options,
		ID:        id.NewID[product.Product](),
		CreatedAt: date.NewCreateDate[product.Product](),
		UpdatedAt: date.NewUpdateDate[product.Product](),
	}

	return full, s.repo.Create(ctx, full)
}

func (s *Service) Update(ctx context.Context, productID id.ID[product.Product], options product.Options) (product.Product, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	model, err := s.repo.GetByIDList(ctx, []id.ID[product.Product]{productID})
	if err != nil {
		return model[0], err
	}

	return model[0], s.repo.Update(ctx, model[0])
}
