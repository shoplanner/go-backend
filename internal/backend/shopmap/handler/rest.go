package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"go-backend/internal/backend/shopmap"
	"go-backend/internal/backend/shopmap/service"
)

type Handler struct {
	service *service.Service
}

func New(r *gin.Engine, service *service.Service) *Handler {
	h := Handler{service: service}

	group := r.Group("/shopmap")

	group.POST("/", h.CreateMap)
	group.GET("/user", h.GetCurrentUserList)

	idGroup := group.Group("/id")
	idGroup.GET("/:id", h.GetByID)
	idGroup.DELETE("/:id", h.DeleteMap)
	idGroup.PUT("/:id", h.UpdateMap)
	idGroup.PATCH("/:id/reorder", h.ReorderMap)

	return &h
}

// @Summary Creates new shop map
// @ID shopmap-create
//
// @Tags ShopMap
// @Accept json
// @Param config body shopmap.ShopMapConfig true "shop map to create"
// @Product json
// @Router /shopmap [post]
func (h *Handler) CreateMap(ctx *gin.Context) {
	var shopMapCfg shopmap.ShopMapConfig

	if err := ctx.ShouldBindJSON(&shopMapCfg); err != nil {
		ctx.String(http.StatusBadRequest, "can't decode request")
		return
	}
	shopMap, err := h.service.Create(ctx, nil, shopMapCfg)
	if err != nil {
		ctx.String(http.StatusInternalServerError, "can't create shop map due to internal error")
		return
	}

	ctx.JSON(http.StatusOK, shopMap)
}

func (h *Handler) GetByID(ctx *gin.Context) {
}

func (h *Handler) GetCurrentUserList(ctx *gin.Context) {
}

func (h *Handler) DeleteMap(ctx *gin.Context) {
}

func (h *Handler) UpdateMap(ctx *gin.Context) {
}

func (h *Handler) ReorderMap(ctx *gin.Context) {
}
