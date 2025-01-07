package api

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/samber/lo"

	"go-backend/internal/backend/auth/api"
	"go-backend/internal/backend/favorite/service"
	"go-backend/internal/backend/product"
	"go-backend/pkg/id"
)

type ProductList struct {
	ProductIDs []string `json:"product_ids"`
}

type Handler struct {
	service *service.Service
}

func RegisterREST(r *gin.RouterGroup, service *service.Service) {
	group := r.Group("/favorite")

	h := Handler{service: service}

	group.GET("/id/:id", h.GetFavoriteListByID)
	group.GET("/user", h.GetUserLists)
	group.DELETE("/id/:id/product", h.DeleteProductList)
	group.POST("/id/:id/product", h.AppendProductList)
}

// @Summary	get list of favorite products by id
// @ID			get-favorite-list-id
// @Tags		Favorites
//
// @Param		id	query	string	true	"id of favorites list"
// @Produce	json
// @Router		/favorite/id/{id} [get]
// @Security	ApiKeyAuth
func (h *Handler) GetFavoriteListByID(c *gin.Context) {
	userID := api.GetUserID(c)
}

// @Summary	add new products to list of favorties
// @ID			add-favorite-products
// @Tags		Favorites
//
// @Param		id			query	string				true	"id of favorites list"
// @Param		products	body	AddProductsRequest	true	"ids of new products"
// @Produce	json
// @Router		/favorite/id/{id}/product [post]
// @Security	ApiKeyAuth
func (h *Handler) AppendProductList(ctx *gin.Context) {
	var productList []string
}

// @Summary	remove some products from favorite list
// @ID			remove-favorite-products
// @Tags		Favorites
//
// @Param		id			query	string				true	"id of favorites list"
// @Param		products	body	AddProductsRequest	true	"ids of new products"
// @Produce	json
// @Router		/favorite/id/{id}/product [delete]
// @Security	ApiKeyAuth
func (h *Handler) DeleteProductList(ctx *gin.Context) {
	var rawProductList []string

	if err := ctx.BindJSON(&rawProductList); err != nil {
		ctx.String(http.StatusBadRequest, "can't decode request")
		return
	}

	rawListID := ctx.Param("id")
	listUUID, err := uuid.Parse(rawListID)
	if err != nil {
		ctx.Str
	}

	idList := make([]id.ID[product.Product], 0, len(rawProductList))
	for _, rawID := range rawProductList {
		parsedUUID, err := uuid.Parse(rawID)
		if err != nil {
			ctx.String(http.StatusBadRequest, "id must be valid UUID")
			return
		}

		idList = append(idList, id.ID[product.Product]{UUID: parsedUUID})
	}

	model, err := h.service.AddProducts(ctx, , userID id.ID[user.User], productIDs []id.ID[product.Product])
}

// @Summary	get all favorites lists, related to logged user
// @ID			get-user-favorites-lists
// @Tags		Favorites
//
// @Produce	json
// @Router		/favorite/user [get]
// @Security	ApiKeyAuth
func (h Handler) GetUserLists(c *gin.Context) {
	userID := api.GetUserID(c)
}
