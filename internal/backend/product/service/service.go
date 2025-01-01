package service

import (
	"context"
	"fmt"
	"sync"

	"go-backend/internal/backend/product"
	"go-backend/pkg/date"
	"go-backend/pkg/id"
)

type repo interface {
	GetByListID(context.Context, []id.ID[product.Product]) ([]product.Product, error)
	GetByID(context.Context, id.ID[product.Product]) (product.Product, error)
	Create(context.Context, product.Product) error
	GetAndUpdate(
		context.Context,
		id.ID[product.Product],
		func(product.Product) (product.Product, error),
	) (product.Product, error)
}

type Service struct {
	repo repo
	lock sync.RWMutex
}

func NewService(repo repo) *Service {
	return &Service{
		repo: repo,
		lock: sync.RWMutex{},
	}
}

func (s *Service) ID(ctx context.Context, productID id.ID[product.Product]) (product.Product, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	model, err := s.repo.GetByID(ctx, productID)
	if err != nil {
		return model, wrapErr(fmt.Errorf("can't get product %s: %w", productID, err))
	}

	return model, nil
}

func (s *Service) IDList(ctx context.Context, ids []id.ID[product.Product]) ([]product.Product, error) {
	s.lock.RLock()
	defer s.lock.RUnlock()

	models, err := s.repo.GetByListID(ctx, ids)
	if err != nil {
		return nil, wrapErr(fmt.Errorf("can't get products %v: %w", ids, err))
	}

	return models, nil
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

func (s *Service) Update(ctx context.Context, productID id.ID[product.Product], options product.Options) (
	product.Product,
	error,
) {
	s.lock.Lock()
	defer s.lock.Unlock()

	model, err := s.repo.GetAndUpdate(ctx, productID, func(p product.Product) (product.Product, error) {
		p.Options = options
		return p, nil
	})
	if err != nil {
		return model, err
	}

	return model, nil
}

func wrapErr(err error) error {
	if err != nil {
		return fmt.Errorf("product service: %w", err)
	}

	return nil
}
