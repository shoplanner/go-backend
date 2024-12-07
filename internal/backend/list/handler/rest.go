package handler

import "github.com/gin-gonic/gin"

type Handler struct{}

func NewHandler(r *gin.Engine) *gin.RouterGroup {
	group := r.Group("list")

	h := Handler{}

	r.POST("/", h.Create)
	group.GET("/user", h.GetByUserID)

	idGroup := group.Group("/id")
	idGroup.GET("/:id", h.GetByListID)
	idGroup.DELETE("/:id", h.Delete)
	idGroup.POST("/:id/product", h.AddProduct)
	idGroup.DELETE("/:id/product", h.DeleteProduct)
	idGroup.POST("/:id/member", h.AddViewer)
	idGroup.PUT("/:id", h.Update)
	return group
}

func (h *Handler) Create(ctx *gin.Context) {
}

func (h *Handler) GetByListID(ctx *gin.Context) {
}

func (h *Handler) GetByUserID(ctx *gin.Context) {
}

func (h *Handler) Delete(ctx *gin.Context) {
}

func (h *Handler) Update(ctx *gin.Context) {
}

func (h *Handler) AddViewer(ctx *gin.Context) {
}

func (h *Handler) AddProduct(ctx *gin.Context) {
}

func (h *Handler) DeleteProduct(ctx *gin.Context) {
}
