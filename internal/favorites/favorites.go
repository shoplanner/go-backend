package favorites

import (
	"context"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/mongo"

	"go-backend/internal/product"
)

type Favorites struct {
	UserID   uuid.UUID
	Products []product.ProductResponse
}

type Service struct {
	col *mongo.Collection
}

func NewService(col *mongo.Collection) *Service {
	return &Service{col: col}
}

func (s *Service) Add(ctx context.Context, userID uuid.UUID, model product.ProductResponse) error {
}

func (s *Service) AddList(userID uuid.UUID, models []product.ProductResponse) error {
	panic("Not implemented")
}

func (s *Service) Delete(userID uuid.UUID, productID uuid.UUID) error {
	panic("Not implemented")
}
