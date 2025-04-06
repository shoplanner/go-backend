package auth

import (
	"fmt"
	"time"

	"go-backend/internal/backend/user"
	"go-backend/pkg/id"
	"go-backend/pkg/myerr"
)

//go:generate python $GOENUM

// ENUM(active=1,revoked)
type TokenStatus int32

type Credentials struct {
	Login    user.Login `json:"login"`
	Password string     `json:"password"`
	DeviceID DeviceID   `json:"device_id"`
}

type RefreshToken struct {
	RefreshTokenOptions

	SignedString EncodedRefreshToken
	State        TokenState
}

type RefreshTokenOptions struct {
	TokenID[RefreshToken]

	Expires  time.Time
	IssuedAt time.Time
}

type AccessTokenOptions struct {
	TokenID[AccessToken]

	Role     user.Role
	Expires  time.Time
	IssuedAt time.Time
}

type TokenID[T any] struct {
	ID       id.ID[T]         `json:"id"`
	UserID   id.ID[user.User] `json:"user_id"`
	DeviceID DeviceID         `json:"device_id"`
}

type AccessToken struct {
	AccessTokenOptions

	SignedString EncodedAccessToken
	State        TokenState
}

type TokenState struct {
	Status TokenStatus `json:"status"`
}

type (
	EncodedAccessToken  string
	EncodedRefreshToken string
)

type DeviceID string

var (
	ErrTokenExpired   = fmt.Errorf("%w: expired", myerr.ErrForbidden)
	ErrTokenNotActive = fmt.Errorf("%w: not active yet", myerr.ErrForbidden)
)
