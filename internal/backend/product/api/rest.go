package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"go-backend/internal/backend/product"
	"go-backend/internal/backend/product/service"
	"go-backend/pkg/id"
)

type ProductHandler struct {
	service *service.Service
}

func RegisterREST(r *gin.RouterGroup, service *service.Service) {
	h := &ProductHandler{service: service}

	group := r.Group("product")

	group.GET("/id/:id", h.Get)
	group.PUT("/id/:id", h.Update)
	group.POST("", h.Create)
	group.GET("/list/:ids", h.GetList)
}

// @Summary	Creates new product
// @ID			product-create
//
// @Tags		Product
// @Accept		json
// @Param		product	body	product.Options	true	"product to create"
// @Produce	json
// @Security ApiKeyAuth
// @Router		/product [post]
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
// @Security ApiKeyAuth
// @Router		/product/id/{id} [put]
func (h *ProductHandler) Update(c *gin.Context) {
	var model product.Options
	productID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.String(http.StatusBadRequest, "ID has incorrect format")
		return
	}
	if decodeError := c.ShouldBindJSON(&model); decodeError != nil {
		c.String(http.StatusBadRequest, "Can't decode request")
		return
	}
	updated, err := h.service.Update(c, id.ID[product.Product]{UUID: productID}, model)
	if err != nil {
		log.Err(err).Msg("updating product")
		c.String(http.StatusInternalServerError, "internal error")
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
// @Router		/product/id/{id} [get]
// @Security ApiKeyAuth
func (h *ProductHandler) Get(c *gin.Context) {
	productID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.String(http.StatusBadRequest, "ID has incorrect format")
		return
	}

	model, err := h.service.ID(c, id.ID[product.Product]{UUID: productID})
	if err != nil {
		c.String(http.StatusInternalServerError, "Internal error")
		return
	}
	c.JSON(http.StatusOK, model)
}

// @Summary	Get products info
// @ID			product-get-list
//
// @Tags		Product
// @Accept		json
// @Param		ids	path	[]string	true	"product id"
// @Produce	json
// @Router		/product/list/{ids} [get]
// @Security ApiKeyAuth
func (h *ProductHandler) GetList(c *gin.Context) {
	rawIDs := strings.Split(c.Param("ids"), ",")

	parsedIDs := make([]id.ID[product.Product], 0, len(rawIDs))
	for _, rawID := range rawIDs {
		parsedID, err := uuid.Parse(rawID)
		if err != nil {
			c.String(http.StatusBadRequest, "id must be valid uuid")
			return
		}

		parsedIDs = append(parsedIDs, id.ID[product.Product]{UUID: parsedID})
	}

	models, err := h.service.IDList(c, parsedIDs)
	if err != nil {
		c.String(http.StatusInternalServerError, "internal error")
		return
	}

	c.JSON(http.StatusOK, models)
}
