package service

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/samber/lo"
	"go.uber.org/zap"

	"go-backend/internal/backend/product"
	"go-backend/pkg/id"
)

type repo interface {
	GetByID(context.Context, id.ID[product.Product])
}

type Service struct {
	repo repo
	log  zap.SugaredLogger
	lock sync.RWMutex
}

func NewService(repo repo) *Service {
	return &Service{repo: repo, log: *zap.NewNop().Sugar().Named("")}
}

func (s *Service) ID(ctx context.Context, id uuid.UUID) (product.Product, error) {
	var product models.ProductInfo
	var err error
	lo.Synchronize(&s.lock).Do(func() {
		product, err = s.repo.ID(ctx, id)
	})

	return product, err
}

func (s *Service) IDList(ctx context.Context, ids []uuid.UUID) ([]models.Response, error) {
	s.lock.RLock()
	defer s.lock.RUnlock()

	return s.repo.IDList(ctx, ids)
}

func (s *Service) Create(ctx context.Context, product models.Request) (models.Response, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	full := models.Response{
		Request:   product,
		ID:        uuid.New(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	return full, s.repo.Create(ctx, full)
}

func (s *Service) Update(ctx context.Context, id uuid.UUID, product models.Request) (models.Response, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	model, err := s.repo.ID(ctx, id)
	if err != nil {
		return model, err
	}

	model.Request = product
	model.UpdatedAt = time.Now()

	return s.repo.Update(ctx, model)
}
