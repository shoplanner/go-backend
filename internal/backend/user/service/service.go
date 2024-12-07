package service

import (
	"shoplanner/internal/"
	"github.com/google/uuid"
)

type Service struct{}

func NewService() *Service {
	return &Service{}
}

func Create(name, password string) (user.User, error) {
}

func Login(name, password string) error {
}

func Logout(id uuid.UUID) error {
}
