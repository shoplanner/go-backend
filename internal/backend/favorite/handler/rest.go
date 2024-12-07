package handler

import "github.com/gin-gonic/gin"

type Handler struct{}

func NewREST(r *gin.Engine) {
	group := r.Group("/favorite/user")

	h := Handler{}

	group.GET("/", h.GetFavoriteList)
	group.DELETE("/", h.DeleteProductList)
	group.POST("/", h.AppendProductList)
}

func (h *Handler) GetFavoriteList(ctx *gin.Context) {
}

func (h *Handler) AppendProductList(ctx *gin.Context) {
}

func (h *Handler) DeleteProductList(ctx *gin.Context) {
}
