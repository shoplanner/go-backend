package handler

import "github.com/gin-gonic/gin"

type Handler struct{}

func New(r *gin.Engine) *Handler {
	h := Handler{}

	group := r.Group("/map")

	group.POST("/", h.CreateMap)
	group.GET("/user", h.GetCurrentUserList)

	idGroup := group.Group("/id")
	idGroup.GET("/:id", h.GetByID)
	idGroup.DELETE("/:id", h.DeleteMap)
	idGroup.PUT("/:id", h.UpdateMap)
	idGroup.PATCH("/:id/reorder", h.ReorderMap)

	return &h
}

func (h *Handler) CreateMap(ctx *gin.Context) {
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
