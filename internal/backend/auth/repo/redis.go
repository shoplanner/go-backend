package repo

import (
	"go-backend/internal/backend/auth"
	"go-backend/internal/backend/user"
	"go-backend/pkg/id"
)

type Repo struct {
	m map[id.ID[user.User]]id.ID[auth.RefreshToken]
}

func NewRepo() *Repo {
	return &Repo{m: make(map[id.ID[user.User]]id.ID[auth.RefreshToken])}
}

func (r *Repo) Insert(userID id.ID[user.User], id.ID[auth.RefreshToken]) {
}
