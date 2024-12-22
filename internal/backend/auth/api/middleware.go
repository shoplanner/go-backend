package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"go-backend/internal/backend/auth"
	"go-backend/internal/backend/auth/service"
	"go-backend/internal/backend/user"
	"go-backend/pkg/myerr"
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

		rawToken, cut := strings.CutPrefix(header, "Bearer")
		if !cut {
			c.String(http.StatusBadRequest, "only 'Bearer' tokens accepted")
			return
		}

		token, err := m.auth.IsAccessTokenValid(c, auth.EncodedAccessToken(rawToken))
		if errors.Is(err, myerr.ErrForbidden) {
			c.String(http.StatusForbidden, err.Error())
			return
		} else if err != nil {
			// TODO: logging
			c.String(http.StatusForbidden, "forbidden")
			return
		}

		c.Set("userId", token.UserID)
		c.Set("userRole", token.Role)
	}
}

func NewRoleMiddleware(targetRole user.Role) func(*gin.Context) {
	return func(c *gin.Context) {
		role, casted := c.Value("userRole").(user.Role)
		if !role.IsValid() || !casted {
			c.String(http.StatusForbidden, "forbidden")
			return
		}

		if role > targetRole {
			c.String(http.StatusForbidden, "forbidden")
			return
		}
	}
}
