package api

import (
	"github.com/gin-gonic/gin"

	"go-backend/internal/backend/list/service"
)

type Handler struct {
	service *service.Service
}

func RegisterREST(r *gin.RouterGroup, service *service.Service) {
	group := r.Group("/list")

	h := Handler{service: service}

	r.POST("", h.CreateList)
	group.GET("/user", h.GetByUserID)

	idGroup := group.Group("/id")
	idGroup.GET("/:id", h.GetByListID)
	idGroup.DELETE("/:id", h.Delete)
	idGroup.POST("/:id/product", h.AddProduct)
	idGroup.DELETE("/:id/product", h.DeleteProduct)
	idGroup.POST("/:id/member", h.AddViewerList)
	idGroup.PUT("/:id", h.Update)
}

func (h *Handler) CreateList(ctx *gin.Context) {
	h.service.Create()
}

func (h *Handler) GetByListID(ctx *gin.Context) {
	h.service.GetByID()
}

func (h *Handler) GetByUserID(ctx *gin.Context) {
	h.service.GetByUserID()
}

func (h *Handler) Delete(ctx *gin.Context) {
	h.service.DeleteList()
}

func (h *Handler) Update(ctx *gin.Context) {
	h.service.Update()
}

func (h *Handler) AddViewerList(ctx *gin.Context) {
}

func (h *Handler) AddProduct(ctx *gin.Context) {
}

func (h *Handler) DeleteProduct(ctx *gin.Context) {
}
