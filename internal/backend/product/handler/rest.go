package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"go-backend/internal/backend/product"
	"go-backend/internal/backend/product/service"
)

type ProductHandler struct {
	service *service.Service
}

func NewProductController(service *service.Service) *ProductHandler {
	return &ProductHandler{service: service}
}

func (h *ProductHandler) InitRoutes(r *gin.Engine) {
	group := r.Group("product")

	group.GET("/:id", h.Get)
	group.PUT("/:id", h.Update)
	group.POST("/", h.Create)
}

// @Summary	Creates new product
// @ID			product-create
//
// @Tags		Product
// @Accept		json
// @Param		product	body	product.Options	true	"product to create"
// @Produce	json
// @Router		/product [post]
// @Success	200	{object}	product.Product
func (h *ProductHandler) Create(c *gin.Context) {
	var model product.Options
	if err := c.ShouldBindJSON(&model); err != nil {
		c.String(http.StatusBadRequest, "Can't decode request")
		return
	}
	created, err := h.service.Create(c, model)
	if err != nil {
		c.String(http.StatusInternalServerError, "Internal error")
		return
	}

	c.JSON(http.StatusOK, created)
}

// @Summary	Update existing new product
// @ID			product-update
//
// @Tags		Product
// @Param		id		path	string			true	"product id"
// @Param		product	body	product.Options	true	"product to update"
// @Produce	json
// @Router		/product/{id} [put]
// @Success	200	{object}	product.Product
func (h *ProductHandler) Update(c *gin.Context) {
	var model product.Options
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.String(http.StatusBadRequest, "ID has incorrect format")
		return
	}
	if decodeError := c.ShouldBindJSON(&model); decodeError != nil {
		c.String(http.StatusBadRequest, "Can't decode request")
		return
	}
	updated, err := h.service.Update(c, id, model)
	if err != nil {
		c.String(http.StatusInternalServerError, "Internal error")
		return
	}

	c.JSON(http.StatusOK, updated)
}

// @Summary	Get product info
// @ID			product-get
//
// @Tags		Product
// @Accept		json
// @Param		id	path	string	true	"product id"
// @Produce	json
// @Router		/product/{id} [get]
// @Success	200	{object}	product.Product
func (h *ProductHandler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.String(http.StatusBadRequest, "ID has incorrect format")
		return
	}

	model, err := h.service.ID(c, id)
	if err != nil {
		c.String(http.StatusInternalServerError, "Internal error")
		return
	}
	c.JSON(http.StatusOK, model)
}
