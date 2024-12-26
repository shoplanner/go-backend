package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/samber/lo"

	"go-backend/internal/backend/auth"
	"go-backend/internal/backend/user"
	"go-backend/pkg/id"
	"go-backend/pkg/myerr"
)

type userService interface {
	ValidatePassword(context.Context, user.Login, string) (user.User, error)
	GetByID(context.Context, id.ID[user.User]) (user.User, error)
}

type tokenRepo[T any] interface {
	Set(context.Context, auth.TokenID[T], auth.TokenState) error
	GetByID(context.Context, id.ID[T]) (auth.TokenID[T], auth.TokenState, error)
	DeleteByID(context.Context, id.ID[T]) error
	RevokeByDeviceID(context.Context, id.ID[user.User], auth.DeviceID) error
	RevokeByUserID(context.Context, id.ID[user.User]) error
}

type tokenEncoder interface {
	EncodeAccessToken(context.Context, auth.AccessTokenOptions) (auth.EncodedAccessToken, error)
	EncodeRefreshToken(context.Context, auth.RefreshTokenOptions) (auth.EncodedRefreshToken, error)
	DecodeAccessToken(context.Context, auth.EncodedAccessToken) (auth.AccessTokenOptions, error)
	DecodeRefreshToken(context.Context, auth.EncodedRefreshToken) (auth.RefreshTokenOptions, error)
}

type Service struct {
	users       userService
	refreshRepo tokenRepo[auth.RefreshToken]
	accessRepo  tokenRepo[auth.AccessToken]
	lock        sync.RWMutex
	options     Options
	encoder     tokenEncoder
}

type Options struct {
	AccessTokenExpires  time.Duration
	RefreshTokenExpires time.Duration
}

func New(
	users userService,
	refreshRepo tokenRepo[auth.RefreshToken],
	accessRepo tokenRepo[auth.AccessToken],
	encoder tokenEncoder,
	options Options,
) *Service {
	return &Service{
		users:       users,
		refreshRepo: refreshRepo,
		accessRepo:  accessRepo,
		lock:        sync.RWMutex{},
		options:     options,
		encoder:     encoder,
	}
}

func (s *Service) Login(ctx context.Context, opts auth.Credentials) (auth.AccessToken, auth.RefreshToken, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	loggedUser, err := s.users.ValidatePassword(ctx, opts.Login, opts.Password)
	if err != nil {
		return auth.AccessToken{}, auth.RefreshToken{}, fmt.Errorf("%w: %w", myerr.ErrInvalidArgument, err)
	}

	return s.getNewTokens(ctx, loggedUser, opts.DeviceID)
}

func (s *Service) Refresh(ctx context.Context, encodedRefreshToken auth.EncodedRefreshToken) (
	auth.AccessToken,
	auth.RefreshToken,
	error,
) {
	s.lock.Lock()
	defer s.lock.Unlock()

	opts, err := s.encoder.DecodeRefreshToken(ctx, encodedRefreshToken)
	if err != nil {
		return auth.AccessToken{}, auth.RefreshToken{}, fmt.Errorf("can't decode refresh token: %w", err)
	}

	_, state, err := s.refreshRepo.GetByID(ctx, opts.ID)
	if err != nil {
		return auth.AccessToken{}, auth.RefreshToken{}, fmt.Errorf("can't get token info from storage: %w", err)
	}

	if state.Status == auth.TokenStatusRevoked {
		// refresh token was stolen
		// emergency close all sections
		var errs []error

		if deleteError := s.accessRepo.RevokeByUserID(ctx, opts.UserID); deleteError != nil {
			errs = append(errs, fmt.Errorf("can't revoke all refresh tokens for user id %s: %w", opts.UserID, err))
		}
		if deleteError := s.refreshRepo.RevokeByUserID(ctx, opts.UserID); deleteError != nil {
			errs = append(errs, fmt.Errorf("can't revoke all access tokens for user id %s: %w", opts.UserID, deleteError))
		}

		return auth.AccessToken{}, auth.RefreshToken{}, errors.Join(
			append(errs, fmt.Errorf("%w: token was already revoked", myerr.ErrForbidden))...,
		)
	}

	if time.Now().UTC().Compare(opts.Expires.UTC()) != -1 {
		return auth.AccessToken{}, auth.RefreshToken{}, fmt.Errorf("%w: refresh token", auth.ErrTokenExpired)
	}
	if time.Now().UTC().Compare(opts.IssuedAt.UTC()) != 1 {
		return auth.AccessToken{}, auth.RefreshToken{}, fmt.Errorf("%w: refresh token", auth.ErrTokenNotActive)
	}

	loggedUser, err := s.users.GetByID(ctx, opts.UserID)
	if err != nil {
		return auth.AccessToken{}, auth.RefreshToken{}, fmt.Errorf("can't get user %s: %w", opts.UserID, err)
	}

	if err = s.refreshRepo.RevokeByDeviceID(ctx, opts.UserID, opts.DeviceID); err != nil {
		return auth.AccessToken{}, auth.RefreshToken{}, fmt.Errorf("can't revoke: %w", err)
	}
	if err = s.accessRepo.RevokeByDeviceID(ctx, opts.UserID, opts.DeviceID); err != nil {
		return auth.AccessToken{}, auth.RefreshToken{}, fmt.Errorf("can't revoke access tokens: %w", err)
	}

	return s.getNewTokens(ctx, loggedUser, opts.DeviceID)
}

func (s *Service) IsAccessTokenValid(ctx context.Context, encodedToken auth.EncodedAccessToken) (
	auth.AccessTokenOptions,
	error,
) {
	s.lock.RLock()
	defer s.lock.RUnlock()

	opts, err := s.encoder.DecodeAccessToken(ctx, encodedToken)
	if err != nil {
		return opts, fmt.Errorf("decoding access token failed: %w", err)
	}

	log.Info().Any("opts", opts).Send()

	tokenID, state, err := s.accessRepo.GetByID(ctx, opts.ID)
	if err != nil {
		return opts, fmt.Errorf("can't get full access token from storage: %w", err)
	}
	if state.Status == auth.TokenStatusRevoked {
		return opts, fmt.Errorf("%w: token %s already revoked", myerr.ErrForbidden, tokenID.ID)
	}

	fmt.Println(time.Now().UTC().Compare(opts.Expires.UTC()))

	if time.Now().UTC().Compare(opts.Expires.UTC()) != -1 {
		return opts, fmt.Errorf("%w: access token", auth.ErrTokenExpired)
	}
	if time.Now().UTC().Compare(opts.IssuedAt.UTC()) != 1 {
		return opts, fmt.Errorf("%w: access token", auth.ErrTokenNotActive)
	}

	return opts, nil
}

func (s *Service) Logout(ctx context.Context, userID id.ID[user.User], deviceID auth.DeviceID) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	return errors.Join(
		s.accessRepo.RevokeByDeviceID(ctx, userID, deviceID),
		s.refreshRepo.RevokeByDeviceID(ctx, userID, deviceID),
	)
}

func (s *Service) getNewTokens(ctx context.Context, userModel user.User, deviceID auth.DeviceID) (
	auth.AccessToken,
	auth.RefreshToken,
	error,
) {
	var err error

	accessToken := auth.AccessToken{
		AccessTokenOptions: auth.AccessTokenOptions{
			TokenID: auth.TokenID[auth.AccessToken]{
				ID:       id.NewID[auth.AccessToken](),
				UserID:   userModel.ID,
				DeviceID: deviceID,
			},
			Role:     userModel.Role,
			Expires:  time.Now().UTC().Add(s.options.AccessTokenExpires),
			IssuedAt: time.Now().UTC(),
		},
		State: auth.TokenState{
			Status: auth.TokenStatusActive,
		},
		SignedString: "",
	}

	refreshToken := auth.RefreshToken{
		RefreshTokenOptions: auth.RefreshTokenOptions{
			TokenID: auth.TokenID[auth.RefreshToken]{
				ID:       id.NewID[auth.RefreshToken](),
				UserID:   userModel.ID,
				DeviceID: deviceID,
			},
			Expires:  time.Now().UTC().Add(s.options.RefreshTokenExpires),
			IssuedAt: time.Now().UTC(),
		},
		State: auth.TokenState{
			Status: auth.TokenStatusActive,
		},
		SignedString: "",
	}

	accessToken.SignedString, err = s.encoder.EncodeAccessToken(ctx, accessToken.AccessTokenOptions)
	if err != nil {
		return auth.AccessToken{}, auth.RefreshToken{}, fmt.Errorf("can't encode token: %w", err)
	}

	refreshToken.SignedString, err = s.encoder.EncodeRefreshToken(ctx, refreshToken.RefreshTokenOptions)
	if err != nil {
		return auth.AccessToken{}, auth.RefreshToken{}, fmt.Errorf("can't encode token: %w", err)
	}

	_, err = lo.NewTransaction[any]().Then(
		func(_ any) (any, error) { return nil, s.refreshRepo.Set(ctx, refreshToken.TokenID, refreshToken.State) },
		func(_ any) any { return s.refreshRepo.DeleteByID(ctx, refreshToken.ID) },
	).Then(
		func(_ any) (any, error) { return nil, s.accessRepo.Set(ctx, accessToken.TokenID, accessToken.State) },
		func(_ any) any { return s.accessRepo.DeleteByID(ctx, accessToken.ID) },
	).Process(nil)
	if err != nil {
		return auth.AccessToken{}, auth.RefreshToken{}, fmt.Errorf("can't save tokens to storage: %w", err)
	}

	return accessToken, refreshToken, nil
}
