package provider

import (
	"context"
	"crypto/ecdsa"
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
)

type JWTProvider struct {
	privateKey *ecdsa.PrivateKey
}

func NewJWT(privateKey *ecdsa.PrivateKey) *JWTProvider {
	return &JWTProvider{privateKey: privateKey}
}

func (p *JWTProvider) EncodeAccessToken(_ context.Context, token auth.AccessTokenOptions) (auth.EncodedAccessToken, error) {
	accessToken := jwt.NewWithClaims(&jwt.SigningMethodECDSA{}, jwt.MapClaims{
		userIDClaim:    token.UserID.String(),
		roleClaim:      token.Role.String(),
		expiresClaim:   token.Expires.UTC().Unix(),
		tokenIDClaim:   token.ID.String(),
		deviceIDClaim:  token.DeviceID,
		notBeforeClaim: token.IssuedAt.UTC().Unix(),
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
	refreshToken := jwt.NewWithClaims(&jwt.SigningMethodECDSA{}, jwt.MapClaims{
		userIDClaim:    token.UserID.String(),
		expiresClaim:   jwt.NewNumericDate(token.Expires.UTC()),
		notBeforeClaim: jwt.NewNumericDate(token.IssuedAt.UTC()),
		tokenIDClaim:   token.ID.String(),
		deviceIDClaim:  token.DeviceID,
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

	_, err := jwt.ParseWithClaims(string(encoded), &claims, func(t *jwt.Token) (interface{}, error) {
		return p.privateKey.PublicKey, nil
	})
	if err != nil {
		return auth.RefreshTokenOptions{}, fmt.Errorf("can't verify token: %w", err)
	}

	tokenID, err := uuid.Parse(claims[tokenIDClaim].(string))
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

	return auth.RefreshTokenOptions{
		TokenID: auth.TokenID[auth.RefreshToken]{
			ID:       id.ID[auth.RefreshToken]{UUID: tokenID},
			UserID:   id.ID[user.User]{UUID: userID},
			DeviceID: claims[deviceIDClaim].(auth.DeviceID),
		},
		Expires:  expires.Time,
		IssuedAt: issuedAt.Time,
	}, nil
}

func (p *JWTProvider) DecodeAccessToken(_ context.Context, encoded auth.EncodedAccessToken) (auth.AccessTokenOptions, error) {
	var claims jwt.MapClaims

	_, err := jwt.ParseWithClaims(string(encoded), &claims, func(t *jwt.Token) (interface{}, error) {
		return p.privateKey.PublicKey, nil
	})
	if err != nil {
		return auth.AccessTokenOptions{}, err
	}
	tokenID, err := uuid.Parse(claims[tokenIDClaim].(string))
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
	role, err := user.ParseRole(claims[roleClaim].(string))
	if err != nil {
		return auth.AccessTokenOptions{}, fmt.Errorf("can't parse user role: %w", err)
	}

	return auth.AccessTokenOptions{
		Role: role,
		TokenID: auth.TokenID[auth.AccessToken]{
			ID:       id.ID[auth.AccessToken]{UUID: tokenID},
			UserID:   id.ID[user.User]{UUID: userID},
			DeviceID: claims[deviceIDClaim].(auth.DeviceID),
		},
		Expires:  expires.Time,
		IssuedAt: issuedAt.Time,
	}, nil
}
