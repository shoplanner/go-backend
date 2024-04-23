package shopmap

import (
	"time"

	"github.com/google/uuid"
)

type ShopMap struct {
	ID         uuid.UUID
	CreatedAt  time.Time
	UpdatedAt  time.Time
	Categories []string
}

type repo interface {
}

type Service struct {
}

func NewService() *Service {
	return nil
}
