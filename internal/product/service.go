package product

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

type repo interface {
	ID(context.Context, uuid.UUID) (Response, error)
	IDList(context.Context, []uuid.UUID) ([]Response, error)
	Create(context.Context, Response) error
	Delete(context.Context, uuid.UUID) (Response, error)
	Update(context.Context, Response) (Response, error)
}

type Service struct {
	repo repo
	log  zap.SugaredLogger
	lock sync.RWMutex
}

func NewService(repo repo) *Service {
	return &Service{repo: repo}
}

func (s *Service) ID(ctx context.Context, id uuid.UUID) (Response, error) {
	s.lock.RLock()
	defer s.lock.RUnlock()

	return s.repo.ID(ctx, id)
}

func (s *Service) IDList(ctx context.Context, ids []uuid.UUID) ([]Response, error) {
	s.lock.RLock()
	defer s.lock.RUnlock()

	return s.repo.IDList(ctx, ids)
}

func (s *Service) Create(ctx context.Context, product Request) (Response, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	full := Response{
		Request:   product,
		ID:        uuid.New(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	return full, s.repo.Create(ctx, full)
}

func (s *Service) Update(ctx context.Context, id uuid.UUID, product Request) (Response, error) {
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
