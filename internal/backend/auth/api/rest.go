package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"go-backend/internal/backend/auth"
	"go-backend/internal/backend/auth/service"
)

type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	Type         string `json:"type"`
	Expires      string `json:"expires"`
}

type Handler struct {
	service *service.Service
}

func RegisterREST(r *gin.RouterGroup, authService *service.Service) {
	group := r.Group("/auth")

	h := &Handler{service: authService}

	group.POST("/login", h.Login)
	group.POST("/logout", h.Logout)
}

// @Summary	login with existing user
// @ID			auth-login
// @Tags		Auth
// @Param		opts	body auth.Credentials		true	"creds"
// @Accept		json
// @Produce	json
// @Router		/auth/login [post]
// @Success 200
func (h *Handler) Login(c *gin.Context) {
	var opts auth.Credentials

	if err := c.BindJSON(&opts); err != nil {
		c.String(http.StatusBadRequest, "can't decode request: %s", err.Error())
		return
	}

	access, refresh, err := h.service.Login(c, opts)
	if err != nil {
		c.String(http.StatusForbidden, "auth error")
		return
	}

	c.JSON(http.StatusOK, TokenResponse{
		AccessToken:  string(access.SignedString),
		RefreshToken: string(refresh.SignedString),
		Type:         "Bearer",
		Expires:      access.Expires.UTC().String(),
	})
}

//	@Summary	logout from session
//	@ID			auth-logout
//	@Tags		Auth
//
// @Router /auth/logout [post]
//
//	@Accept		json
//	@Success	200
func (h *Handler) Logout(c *gin.Context) {
}

func (h *Handler) RefreshToken(c *gin.Context) {
}
