package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"go-backend/internal/backend/auth"
	"go-backend/internal/backend/auth/service"
	"go-backend/internal/backend/user"
	"go-backend/pkg/api/rest/rerr"
	"go-backend/pkg/id"
	"go-backend/pkg/myerr"
)

const (
	userRoleKey = "userRole"
	userIDKey   = "userId"
	deviceIDKey = "deviceId"

	authHeader = "Auth"
)

type JWTMiddleware struct {
	rerr.BaseHandler

	auth *service.Service
	log  zerolog.Logger
}

func NewAuthMiddleware(log zerolog.Logger, auth *service.Service) *JWTMiddleware {
	return &JWTMiddleware{
		BaseHandler: rerr.NewBaseHandler(log),
		auth:        auth,
		log:         log.With().Str("component", "auth middleware").Logger(),
	}
}

func (m *JWTMiddleware) Middleware() func(*gin.Context) {
	return func(c *gin.Context) {
		header := c.GetHeader(authHeader)

		rawToken, cut := strings.CutPrefix(header, "Bearer ")
		if !cut {
			m.HandleError(c, fmt.Errorf("%w: only 'Bearer tokens accepted", myerr.ErrInvalidArgument))
			c.Abort()
			return
		}

		token, err := m.auth.IsAccessTokenValid(c, auth.EncodedAccessToken(rawToken))
		switch {
		case errors.Is(err, auth.ErrTokenExpired):
			c.String(http.StatusGone, "access token expired")
			c.Abort()
			m.log.Error().
				Stringer("user_id", token.UserID).
				Str("method", c.Request.Method).
				Str("uri", c.Request.RequestURI).
				Msg("access token expired")

			return

		case errors.Is(err, myerr.ErrForbidden):
			c.String(http.StatusForbidden, err.Error())
			m.log.Error().
				Stringer("user_id", token.UserID).
				Str("method", c.Request.Method).
				Str("uri", c.Request.RequestURI).
				Err(err).
				Msg("access forbidden")

			c.Abort()
			return

		case err != nil:
			log.Err(err).Str("user_id", token.ID.String()).Msg("login failed")
			c.String(http.StatusForbidden, "forbidden")
			c.Abort()
			return
		}

		c.Set(userIDKey, token.UserID)
		c.Set(userRoleKey, token.Role)
		c.Set(deviceIDKey, token.DeviceID)
	}
}

func NewRoleMiddleware(targetRole user.Role) func(*gin.Context) {
	return func(c *gin.Context) {
		role, casted := c.Value(userRoleKey).(user.Role)
		if !role.IsValid() || !casted {
			c.String(http.StatusForbidden, "forbidden")
			c.Abort()
			return
		}

		if role > targetRole {
			c.String(http.StatusForbidden, "forbidden")
			c.Abort()
			return
		}

		c.Next()
	}
}

func GetUserID(c context.Context) id.ID[user.User] {
	userID, _ := c.Value(userIDKey).(id.ID[user.User])
	return userID
}

func GetDeviceID(c context.Context) auth.DeviceID {
	deviceID, _ := c.Value(deviceIDKey).(auth.DeviceID)
	return deviceID
}
