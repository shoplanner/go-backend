package service

import (
	"context"

	"github.com/google/uuid"

	"go-backend/internal/backend/user"
)

type repo interface {
	GetByName(context.Context, string) (user.User, error)
	GetByID(context.Context, string) (user.User, error)
	Create(context.Context, user.User) error
	Update(context.Context, user.User) error
	Delete(context.Context, user.User) error
}

type Service struct{}

func NewService() *Service {
	return &Service{}
}

func Create(name, password string) (user.User, error) {
}

func Login(name string, hash user.Hash) error {
}

func Logout(id uuid.UUID) error {
}
