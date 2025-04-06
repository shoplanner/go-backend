package user

import (
	"context"
	"errors"

	"go-backend/pkg/id"
)

//go:generate python $GOENUM

// ENUM(admin=1, user)
type Role int32

type Login string

type Hash string

type User struct {
	ID           id.ID[User] `json:"id"`
	Role         Role        `json:"role"`
	Login        Login       `json:"login"`
	PasswordHash Hash        `json:"-"`
}

type CreateOptions struct {
	Login    Login  `json:"login" `
	Password string `validate:"required,max=72" json:"password"`
}

var ErrAuthorizationFailure = errors.New("authorization error")

type Subscriber interface {
	HandleUserCreated(context.Context, User) error
}
