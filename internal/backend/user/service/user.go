package service

import (
	"context"
	"fmt"
	"sync"

	"github.com/go-playground/validator/v10"
	"github.com/rs/zerolog/log"

	"go-backend/internal/backend/user"
	"go-backend/pkg/id"
	"go-backend/pkg/myerr"
)

type hashMaster interface {
	HashPassword(string) (string, error)
	Compare(string, string) bool
}

type repo interface {
	GetByLogin(context.Context, user.Login) (user.User, error)
	Create(context.Context, user.User) error
	GetAll(context.Context) ([]user.User, error)
	GetByID(context.Context, id.ID[user.User]) (user.User, error)
}

type Service struct {
	lock      sync.RWMutex
	hash      hashMaster
	userRepo  repo
	validator *validator.Validate
}

func NewService(userRepo repo, hash hashMaster) *Service {
	return &Service{
		hash:      hash,
		userRepo:  userRepo,
		validator: validator.New(),
		lock:      sync.RWMutex{},
	}
}

func (s *Service) Create(ctx context.Context, options user.CreateOptions) (user.User, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	if err := s.validator.StructCtx(ctx, options); err != nil {
		return user.User{}, fmt.Errorf("%w: %w", myerr.ErrInvalidArgument, err)
	}

	hash, err := s.hash.HashPassword(options.Password)
	if err != nil {
		return user.User{}, fmt.Errorf("can't hash user password: %w", err)
	}

	newUser := user.User{
		ID:           id.NewID[user.User](),
		Role:         user.RoleUser,
		Login:        options.Login,
		PasswordHash: user.Hash(hash),
	}

	if err = s.userRepo.Create(ctx, newUser); err != nil {
		return user.User{}, fmt.Errorf("can't save user to storage: %w", err)
	}

	return newUser, nil
}

func (s *Service) ValidatePassword(ctx context.Context, login user.Login, pass string) (user.User, error) {
	s.lock.RLock()
	defer s.lock.RUnlock()

	attemptedUser, err := s.userRepo.GetByLogin(ctx, login)
	if err != nil {
		return user.User{}, user.ErrAuthorizationFailure
	}

	if !s.hash.Compare(pass, string(attemptedUser.PasswordHash)) {
		log.Error().Str("login", string(attemptedUser.Login)).Msg("wrong password")
		return user.User{}, user.ErrAuthorizationFailure
	}

	return attemptedUser, nil
}

func (s *Service) GetAllUsers(ctx context.Context) ([]user.User, error) {
	s.lock.RLock()
	defer s.lock.RUnlock()

	users, err := s.userRepo.GetAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("can't get user list from database: %w", err)
	}

	return users, nil
}

func (s *Service) GetByID(ctx context.Context, userID id.ID[user.User]) (user.User, error) {
	s.lock.RLock()
	defer s.lock.RUnlock()

	model, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return model, fmt.Errorf("can't get user from storage: %w", err)
	}

	return model, nil
}
