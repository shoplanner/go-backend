package auth

import (
	"time"

	"go-backend/internal/backend/user"
)

//go:generate go-enum --marshal --names --values

type Credentials struct {
	Login    user.Login `json:"login"`
	Password string     `json:"password"`
}

type AccessToken string

type RefreshToken string

// ENUM(Bearer=1)
type TokenType int

type Token struct {
	AccessToken  AccessToken   `json:"access_token"`
	RefreshToken RefreshToken  `json:"refresh_token"`
	Type         TokenType     `json:"type"`
	ExpiresIn    time.Duration `json:"expires_in"`
}
