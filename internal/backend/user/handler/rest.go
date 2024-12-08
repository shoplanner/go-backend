package handler

import (
	"github.com/gin-gonic/gin"

	"go-backend/internal/backend/user/service"
)

type AuthMiddleware struct {
	user *service.Service
}

type Handler struct {
	user *service.Service
}

func NewREST(r *gin.Engine, userService *service.Service) {
	group := r.Group("/user")

	group.POST("/register")
	group.POST("/login")
	group.POST("/logout")
}

func (h *Handler) Register(c *gin.Context) {
}

func (h *Handler) Login(c *gin.Context) {
}

func (h *Handler) Logout(c *gin.Context) {
}
