package service

import (
	"context"
	"sync"

	"github.com/google/uuid"

	"go-backend/internal/backend/user"
	"go-backend/pkg/id"
)

type repo interface {
	GetByID(context.Context, string) (user.User, error)
	Create(context.Context, user.User) error
}

type Service struct {
	lock sync.RWMutex
}

func NewService() *Service {
	return &Service{}
}

func Create(name, password string) (user.User, error) {
	newUser := user.User{
		ID:           id.NewID[user.User](),
		Role:         user.RoleUser,
		Login:        name,
		PasswordHash: []byte{},
	}
}

func Login(name string, hash user.Hash) error {
}

func Logout(id uuid.UUID) error {
}
