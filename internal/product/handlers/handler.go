package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"go-backend/internal/product"
	"go-backend/internal/product/models"
)

type ProductHandler struct {
	service *product.Service
}

func NewProductController(service *product.Service) *ProductHandler {
	return &ProductHandler{service: service}
}

func (h *ProductHandler) InitRoutes(group *gin.RouterGroup) {
	group.GET("/:id", h.Get)
	group.PUT("/:id", h.Update)
	group.POST("/", h.Create)
}

// @Summary	Creates new product
// @ID			product-create
//
// @Tags		Product
// @Accept		json
// @Param		product	body	models.Request	true	"product to create"
// @Produce	json
// @Router		/product [post]
func (h *ProductHandler) Create(c *gin.Context) {
	var model models.Request
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
// @Param		id		path	string					true	"product id"
// @Param		product	body	models.Request	true	"product to update"
// @Produce	json
// @Router		/product/{id} [put]
func (h *ProductHandler) Update(c *gin.Context) {
	var model models.Request
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.String(http.StatusBadRequest, "ID has incorrect format")
		return
	}
	if err := c.ShouldBindJSON(&model); err != nil {
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

// @Summary	Creates new product
// @ID			product-get
//
// @Tags		Product
// @Accept		json
// @Param		id	path	string	true	"product id"
// @Produce	json
// @Router		/product/{id} [get]
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
