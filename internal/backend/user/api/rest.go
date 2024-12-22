package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"

	"go-backend/internal/backend/auth/api"
	"go-backend/internal/backend/user"
	"go-backend/internal/backend/user/service"
	"go-backend/pkg/myerr"
)

type Handler struct {
	service *service.Service
}

func RegisterREST(r *gin.RouterGroup, userService *service.Service, middleware *api.JWTMiddleware) {
	group := r.Group("/user")

	h := &Handler{service: userService}

	group.GET("", middleware.Middleware(), api.NewRoleMiddleware(user.RoleAdmin), h.GetAll)
	group.POST("/register", h.Register)
}

// @Summary	creates new user
// @ID			user-register
// @Tags		User
// @Param		opts	body	user.CreateOptions	true	"data for creating new user"
// @Accept		json
// @Produce	json
// @Router		/user/register [post]
func (h *Handler) Register(c *gin.Context) {
	var opts user.CreateOptions

	if err := c.BindJSON(&opts); err != nil {
		c.String(http.StatusBadRequest, "can't decode request: %s", err.Error())
		return
	}

	model, err := h.service.Create(c, opts)

	switch {
	case errors.Is(err, myerr.ErrInvalidArgument):
		c.String(http.StatusBadRequest, err.Error())
		return
	case errors.Is(err, myerr.ErrAlreadyExists):
		c.String(http.StatusConflict, err.Error())
		return
	case err != nil:
		log.Err(err).Msg("creating user")
		c.String(http.StatusInternalServerError, "internal error")
		return
	}

	c.JSON(http.StatusOK, model)
}

// @Summary	list all users
// @ID			user-get-all
// @Tags		User
// @Produce	json
// @Success	200
// @Router		/user [get]
// @Security ApiKeyAuth
func (h *Handler) GetAll(ctx *gin.Context) {
	users, err := h.service.GetAllUsers(ctx)
	if err != nil {
		ctx.String(http.StatusInternalServerError, "internal error")
		return
	}

	ctx.JSON(http.StatusOK, users)
}
