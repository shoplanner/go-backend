package service

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"go-backend/internal/backend/auth"
	"go-backend/internal/backend/user"
	"go-backend/pkg/id"
)

type userService interface {
	ValidatePassword(context.Context, user.Login, string) (user.User, error)
}

type refreshTokenRepo interface{}

type accessTokenRepo interface{}

type Service struct {
	users       userService
	refreshRepo refreshTokenRepo
	accessRepo  accessTokenRepo
	lock        sync.RWMutex
	options     Options
}

type Options struct {
	AccessTokenExpires  time.Duration
	RefreshTokenExpires time.Duration
	PrivateKey          *ecdsa.PrivateKey
}

func New(users userService, repo refreshTokenRepo, options Options) *Service {
	return &Service{
		users:       users,
		refreshRepo: repo,
		options:     options,
	}
}

func (s *Service) Login(ctx context.Context, opts auth.Credentials) (auth.Token, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	model, err := s.users.ValidatePassword(ctx, opts.Login, opts.Password)
	if err != nil {
		return auth.Token{}, fmt.Errorf("validate password failed: %w", err)
	}

	expires := time.Now().Add(s.options.AccessTokenExpires)

	accessTokenID := id.NewID[auth.AccessToken]()
	refreshTokenID := id.NewID[auth.RefreshToken]()

	accessToken := jwt.NewWithClaims(&jwt.SigningMethodECDSA{}, jwt.MapClaims{
		"sub":     model.ID.String(),
		"role":    model.Role.String(),
		"expires": expires.String(),
		"tid":     accessTokenID.String(),
	})
	refreshToken := jwt.NewWithClaims(&jwt.SigningMethodECDSA{}, jwt.MapClaims{
		"sub":     model.ID.String(),
		"expires": time.Now().Add(s.options.RefreshTokenExpires).String(),
		"tid":     refreshTokenID.String(),
	})

	signedRefreshToken, err := refreshToken.SignedString(s.options.PrivateKey)
	if err != nil {
		return auth.Token{}, fmt.Errorf("can't sign access token: %w", err)
	}
	signedAccessToken, err := accessToken.SignedString(s.options.PrivateKey)
	if err != nil {
		return auth.Token{}, fmt.Errorf("can't sign access token: %w", err)
	}

	//

	return auth.Token{
		AccessToken:  auth.AccessToken(signedAccessToken),
		RefreshToken: auth.RefreshToken(signedRefreshToken),
		Type:         auth.TokenType(1),
		ExpiresIn:    s.options.AccessTokenExpires,
	}, nil
}

func (s *Service) Refresh(ctx context.Context, userID id.ID[user.User]) {
}
