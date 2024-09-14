package user

import (
	"crypto"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/mongo"
)

type User struct {
	id   uuid.UUID
	name string
	hash crypto.Hash
}

type Service struct {
	
}

func NewService() *Service {
}

func Create(name string, password string) (User, error) {
}

func Login(name, password string) error {
}

func Logout(id uuid.UUID) error {
}
