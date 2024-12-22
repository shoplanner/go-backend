package api

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"

	"go-backend/internal/backend/auth"
	"go-backend/internal/backend/auth/service"
	"go-backend/internal/backend/user"
	"go-backend/pkg/id"
	"go-backend/pkg/myerr"
)

const (
	userRoleKey = "userRole"
	userIDKey   = "userId"
	deviceIDKey = "deviceId"
)

type JWTMiddleware struct {
	auth *service.Service
}

func NewAuthMiddleware(auth *service.Service) *JWTMiddleware {
	return &JWTMiddleware{
		auth: auth,
	}
}

func (m *JWTMiddleware) Middleware() func(*gin.Context) {
	return func(c *gin.Context) {
		header := c.GetHeader("Auth")

		rawToken, cut := strings.CutPrefix(header, "Bearer ")
		if !cut {
			c.String(http.StatusBadRequest, "only 'Bearer' tokens accepted")
			c.Abort()
			return
		}

		token, err := m.auth.IsAccessTokenValid(c, auth.EncodedAccessToken(rawToken))
		if errors.Is(err, auth.ErrTokenExpired) {
			c.String(http.StatusGone, "access token expired")
			c.Abort()
			return
		} else if errors.Is(err, myerr.ErrForbidden) {
			c.String(http.StatusForbidden, err.Error())
			c.Abort()
			return
		} else if err != nil {
			log.Err(err).Str("userId", token.ID.String()).Msg("login failed")
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
