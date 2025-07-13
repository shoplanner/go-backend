package provider

import (
	"context"
	"crypto/ecdsa"
	"errors"
	"fmt"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"go-backend/internal/backend/auth"
	"go-backend/internal/backend/user"
	"go-backend/pkg/id"
)

const (
	userIDClaim    = "sub"
	roleClaim      = "rol"
	expiresClaim   = "exp"
	notBeforeClaim = "nbf"
	tokenIDClaim   = "jti"
	deviceIDClaim  = "did"
	typeClaim      = "typ"
)

type TokenType string

const (
	AccessTokenType  TokenType = "access"
	RefreshTokenType TokenType = "refresh"
)

type JWTProvider struct {
	privateKey *ecdsa.PrivateKey
}

func NewJWT(privateKey *ecdsa.PrivateKey) *JWTProvider {
	return &JWTProvider{privateKey: privateKey}
}

func (p *JWTProvider) EncodeAccessToken(_ context.Context, token auth.AccessTokenOptions) (
	auth.EncodedAccessToken,
	error,
) {
	accessToken := jwt.NewWithClaims(jwt.SigningMethodES256, jwt.MapClaims{
		userIDClaim:    token.UserID.String(),
		roleClaim:      token.Role.String(),
		expiresClaim:   token.Expires.UTC().Unix(),
		tokenIDClaim:   token.ID.String(),
		deviceIDClaim:  token.DeviceID,
		notBeforeClaim: token.IssuedAt.UTC().Unix(),
		typeClaim:      AccessTokenType,
	})

	encoded, err := accessToken.SignedString(p.privateKey)
	if err != nil {
		return "", fmt.Errorf("can't encode access token with EcDSA: %w", err)
	}

	return auth.EncodedAccessToken(encoded), nil
}

func (p *JWTProvider) EncodeRefreshToken(_ context.Context, token auth.RefreshTokenOptions) (
	auth.EncodedRefreshToken,
	error,
) {
	refreshToken := jwt.NewWithClaims(jwt.SigningMethodES256, jwt.MapClaims{
		userIDClaim:    token.UserID.String(),
		expiresClaim:   jwt.NewNumericDate(token.Expires.UTC()),
		notBeforeClaim: jwt.NewNumericDate(token.IssuedAt.UTC()),
		tokenIDClaim:   token.ID.String(),
		deviceIDClaim:  token.DeviceID,
		typeClaim:      RefreshTokenType,
	})

	encoded, err := refreshToken.SignedString(p.privateKey)
	if err != nil {
		return "", fmt.Errorf("can't encode refresh token with EcDSA: %w", err)
	}

	return auth.EncodedRefreshToken(encoded), err
}

func (p *JWTProvider) DecodeRefreshToken(_ context.Context, encoded auth.EncodedRefreshToken) (
	auth.RefreshTokenOptions,
	error,
) {
	var claims jwt.MapClaims
	var opts auth.RefreshTokenOptions

	_, err := jwt.ParseWithClaims(string(encoded), &claims, func(_ *jwt.Token) (any, error) {
		return &p.privateKey.PublicKey, nil
	})
	if errors.Is(err, jwt.ErrTokenExpired) {
		return auth.RefreshTokenOptions{}, fmt.Errorf("%w: refresh token", auth.ErrTokenExpired)
	} else if errors.Is(err, jwt.ErrTokenUsedBeforeIssued) {
		return auth.RefreshTokenOptions{}, fmt.Errorf("%w: refresh token", auth.ErrTokenNotActive)
	} else if err != nil {
		return auth.RefreshTokenOptions{}, fmt.Errorf("can't verify token: %w", err)
	}

	tokenType, passed := claims[typeClaim].(string)
	if !passed {
		return auth.RefreshTokenOptions{}, errors.New("non refresh token passed")
	}

	if tokenType != string(RefreshTokenType) {
		return auth.RefreshTokenOptions{}, errors.New("non refresh token passed")
	}

	rawTokenID, passed := claims[tokenIDClaim].(string)
	if !passed {
		return auth.RefreshTokenOptions{}, errors.New("token id isn't passed")
	}

	tokenID, err := uuid.Parse(rawTokenID)
	if err != nil {
		return opts, fmt.Errorf("can't decode token ID: %w", err)
	}
	sub, err := claims.GetSubject()
	if err != nil {
		return opts, fmt.Errorf("can't get subject claim: %w", err)
	}

	userID, err := uuid.Parse(sub)
	if err != nil {
		return opts, fmt.Errorf("can't parse user ID: %w", err)
	}
	expires, err := claims.GetExpirationTime()
	if err != nil {
		return opts, fmt.Errorf("can't get expiration time: %w", err)
	}
	issuedAt, err := claims.GetNotBefore()
	if err != nil {
		return opts, fmt.Errorf("can't get not before claim: %w", err)
	}

	rawDeviceID, passed := claims[deviceIDClaim].(string)
	if !passed {
		return opts, errors.New("device id is not passed")
	}

	return auth.RefreshTokenOptions{
		TokenID: auth.TokenID[auth.RefreshToken]{
			ID:       id.ID[auth.RefreshToken]{UUID: tokenID},
			UserID:   id.ID[user.User]{UUID: userID},
			DeviceID: auth.DeviceID(rawDeviceID),
		},
		Expires:  expires.Time,
		IssuedAt: issuedAt.Time,
	}, nil
}

func (p *JWTProvider) DecodeAccessToken(_ context.Context, encoded auth.EncodedAccessToken) (
	auth.AccessTokenOptions,
	error,
) {
	var claims jwt.MapClaims

	_, err := jwt.ParseWithClaims(string(encoded), &claims, func(_ *jwt.Token) (interface{}, error) {
		return &p.privateKey.PublicKey, nil
	})

	if errors.Is(err, jwt.ErrTokenExpired) {
		return auth.AccessTokenOptions{}, fmt.Errorf("%w: access token", auth.ErrTokenExpired)
	} else if errors.Is(err, jwt.ErrTokenUsedBeforeIssued) {
		return auth.AccessTokenOptions{}, fmt.Errorf("%w: access token", auth.ErrTokenNotActive)
	} else if err != nil {
		return auth.AccessTokenOptions{}, fmt.Errorf("JWT EcDSA decoding: %w", err)
	}
	rawTokenID, passed := claims[tokenIDClaim].(string)
	if !passed {
		return auth.AccessTokenOptions{}, errors.New("token id is not passed")
	}

	tokenType, passed := claims[typeClaim].(string)
	if !passed {
		return auth.AccessTokenOptions{}, errors.New("non access token passed")
	}

	if tokenType != string(AccessTokenType) {
		return auth.AccessTokenOptions{}, errors.New("non access token passed")
	}

	tokenID, err := uuid.Parse(rawTokenID)
	if err != nil {
		return auth.AccessTokenOptions{}, fmt.Errorf("can't decode token ID: %w", err)
	}
	sub, err := claims.GetSubject()
	if err != nil {
		return auth.AccessTokenOptions{}, fmt.Errorf("can't get subject claim: %w", err)
	}
	userID, err := uuid.Parse(sub)
	if err != nil {
		return auth.AccessTokenOptions{}, fmt.Errorf("can't parse user ID: %w", err)
	}
	expires, err := claims.GetExpirationTime()
	if err != nil {
		return auth.AccessTokenOptions{}, fmt.Errorf("can't get expiration time: %w", err)
	}
	issuedAt, err := claims.GetNotBefore()
	if err != nil {
		return auth.AccessTokenOptions{}, fmt.Errorf("can't get not before claim: %w", err)
	}
	rawRole, passed := claims[roleClaim].(string)
	if !passed {
		return auth.AccessTokenOptions{}, errors.New("role is not passed")
	}
	role, err := user.ParseRole(rawRole)
	if err != nil {
		return auth.AccessTokenOptions{}, fmt.Errorf("can't parse user role: %w", err)
	}

	rawDeviceID, passed := claims[deviceIDClaim].(string)
	if !passed {
		return auth.AccessTokenOptions{}, errors.New("device id is not passed")
	}

	return auth.AccessTokenOptions{
		Role: role,
		TokenID: auth.TokenID[auth.AccessToken]{
			ID:       id.ID[auth.AccessToken]{UUID: tokenID},
			UserID:   id.ID[user.User]{UUID: userID},
			DeviceID: auth.DeviceID(rawDeviceID),
		},
		Expires:  expires.Time,
		IssuedAt: issuedAt.Time,
	}, nil
}
