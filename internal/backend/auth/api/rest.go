package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"

	"go-backend/internal/backend/auth"
	"go-backend/internal/backend/auth/service"
	"go-backend/pkg/myerr"
)

type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	Type         string `json:"type"`
	Expires      string `json:"expires"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type Handler struct {
	service *service.Service
}

func RegisterREST(r *gin.RouterGroup, authService *service.Service, middle *JWTMiddleware) {
	group := r.Group("/auth")

	h := &Handler{service: authService}

	group.POST("/login", h.Login)
	group.POST("/logout", middle.Middleware(), h.Logout)
	group.POST("/refresh", h.RefreshToken)
}

// @Summary	login with existing user
// @ID			auth-login
// @Tags		Auth
// @Param		opts	body	auth.Credentials	true	"creds"
// @Accept		json
// @Produce	json
// @Router		/auth/login [post]
// @Success	200	{object}	TokenResponse
func (h *Handler) Login(c *gin.Context) {
	var opts auth.Credentials

	if err := c.BindJSON(&opts); err != nil {
		c.String(http.StatusBadRequest, "can't decode request: %s", err.Error())
		return
	}

	access, refresh, err := h.service.Login(c, opts)
	if errors.Is(err, auth.ErrTokenExpired) {
		c.String(http.StatusGone, "access token expired")
		return
	} else if err != nil {
		log.Err(err).Str("login", string(opts.Login)).Msg("auth failed")
		c.String(http.StatusForbidden, "auth error")
		return
	}

	c.JSON(http.StatusOK, tokensToResponse(access, refresh))
}

// @Summary	logout from session
// @ID			auth-logout
// @Tags		Auth
// @Router		/auth/logout [post]
// @Accept		json
// @Success	200
// @Security	ApiAuthKey
func (h *Handler) Logout(c *gin.Context) {
	err := h.service.Logout(c, GetUserID(c), GetDeviceID(c))
	if err != nil {
		c.String(http.StatusInternalServerError, "internal error")
	}

	c.Status(http.StatusOK)
}

// @Summary	refresh access token
// @ID			auth-refresh
// @Tags		Auth
// @Param		token	body	RefreshRequest	true	"refresh token"
// @Router		/auth/refresh [post]
// @Accept		json
// @Success	200	{object}	TokenResponse
func (h *Handler) RefreshToken(c *gin.Context) {
	var req RefreshRequest

	if err := c.BindJSON(&req); err != nil {
		c.String(http.StatusBadRequest, "can't decode request")
		return
	}

	access, refresh, err := h.service.Refresh(c, auth.EncodedRefreshToken(req.RefreshToken))
	if errors.Is(err, auth.ErrTokenExpired) {
		c.String(http.StatusGone, "refresh token expired")
		return
	} else if errors.Is(err, myerr.ErrForbidden) {
		log.Err(err).Str("userId", GetUserID(c).String()).Msg("refreshing token failed")
		c.String(http.StatusForbidden, err.Error())
		return
	} else if err != nil {
		log.Err(err).Str("userId", GetUserID(c).String()).Msg("refreshing token failed")
		c.String(http.StatusForbidden, "forbidden")
		return
	}

	c.JSON(http.StatusOK, tokensToResponse(access, refresh))
}

func tokensToResponse(access auth.AccessToken, refresh auth.RefreshToken) TokenResponse {
	return TokenResponse{
		AccessToken:  string(access.SignedString),
		RefreshToken: string(refresh.SignedString),
		Type:         "Bearer",
		Expires:      access.Expires.UTC().String(),
	}
}
