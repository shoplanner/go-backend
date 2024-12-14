package repo

import (
	"context"
	"go-backend/internal/backend/user"
	"go-backend/pkg/id"
)

type Repo struct{}

func (r *Repo) Get(ctx context.Context, userID id.ID[user.User]) (user.User, error) {
}

func (r *Repo) Create(ctx context.Context, model user.User) error {
}
