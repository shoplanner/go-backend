package favorites

import (
	"github.com/google/uuid"

	"go-backend/pkg/models"
)

type Favorites struct {
	UserID   uuid.UUID
	Products []models.ProductResponse
}

type Service struct {
}

func NewService() *Service {
	panic("Not implemented")
}

func (s *Service) Add(userID uuid.UUID, model models.ProductResponse) error {
	panic("Not implemented")
}

func (s *Service) AddList(userID uuid.UUID, models []models.ProductResponse) error {
	panic("Not implemented")
}

func (s *Service) Delete(userID uuid.UUID, productID uuid.UUID) error {
	panic("Not implemented")
}
