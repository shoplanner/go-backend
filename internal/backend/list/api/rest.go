package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"

	"go-backend/internal/backend/auth/api"
	"go-backend/internal/backend/list"
	"go-backend/internal/backend/list/service"
	"go-backend/internal/backend/product"
	"go-backend/internal/backend/user"
	"go-backend/pkg/api/rest/rerr"
	"go-backend/pkg/id"
)

type ProductStateOptions struct {
	Count       *int32 `json:"count"`
	FormIdx     *int32 `json:"form_idx"`
	Status      string `json:"status"`
	Replacement *struct {
		Count     *int32 `json:"count"`
		FormIdx   *int32 `json:"form_idx"`
		ProductID *int32 `json:"product_id"`
	} `json:"replacement"`
}

type Handler struct {
	rerr.BaseHandler

	service *service.Service
}

func RegisterREST(r *gin.RouterGroup, service *service.Service, log zerolog.Logger) {
	group := r.Group("/lists")
	log = log.With().Str("component", "product list handler").Logger()

	h := &Handler{
		BaseHandler: rerr.NewBaseHandler(log),
		service:     service,
	}

	group.POST("", h.CreateList)
	group.GET("", h.GetByUserID)
	group.GET("/:id", h.GetByListID)
	group.DELETE("/:id", h.Delete)
	group.POST("/:id/products", h.AddProducts)
	group.DELETE("/:id/products", h.DeleteProducts)
	group.POST("/:id/members", h.AddViewerList)
	group.DELETE("/:id/members", h.DeleteViewerList)
	group.PUT("/:id", h.Update)
	group.PATCH("/:id/reorder", h.ReorderState)
	group.PATCH("/:id/products/:product_id", h.UpdateProductState)
}

// @Summary creates new product list
// @ID product-list-create
// @Tags ProductList
// @Param opts body list.ListOptions true "options of new product list"
// @Produce json
// @Accept json
// @Router /lists [post]
// @Security	ApiKeyAuth
func (h *Handler) CreateList(ctx *gin.Context) {
	var opts list.ListOptions

	if ok := h.Decode(ctx, &opts); !ok {
		return
	}

	model, err := h.service.Create(ctx, api.GetUserID(ctx), opts)
	if err != nil {
		h.HandleError(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, model)
}

// @Summary get product list by list id
// @ID product-list-get-by-id
// @Tags ProductList
// @Param id path string true "id of product list"
// @Produce json
// @Router /lists/{id} [get]
// @Security	ApiKeyAuth
func (h *Handler) GetByListID(ctx *gin.Context) {
	listID, ok := rerr.PathID[list.ProductList](ctx)
	if !ok {
		return
	}

	model, err := h.service.GetByID(ctx, listID, api.GetUserID(ctx))
	if err != nil {
		h.HandleError(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, model)
}

// @Summary get product lists by user id
// @ID product-list-get
// @Tags ProductList
// @Produce json
// @Router /lists [get]
// @Security	ApiKeyAuth
func (h *Handler) GetByUserID(ctx *gin.Context) {
	models, err := h.service.GetByUserID(ctx, api.GetUserID(ctx))
	if err != nil {
		h.HandleError(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, models)
}

// @Summary delete product list by id
// @ID product-list-delete-by-id
// @Tags ProductList
// @Param id path string true "id of product list"
// @Router /lists/{id} [delete]
// @Security ApiKeyAuth
func (h *Handler) Delete(ctx *gin.Context) {
	userID, found := rerr.PathID[list.ProductList](ctx)
	if !found {
		return
	}

	err := h.service.DeleteList(ctx, api.GetUserID(ctx), userID)
	if err != nil {
		h.HandleError(ctx, err)
		return
	}

	ctx.Status(http.StatusOK)
}

// @Summary update product list by id
// @ID product-list-update-by-id
// @Tags ProductList
// @Param body body list.ListOptions true "opts to update"
// @Param id path string true "id of product list"
// @Produce json
// @Accept json
// @Router /lists/{id} [put]
// @Security ApiKeyAuth
func (h *Handler) Update(ctx *gin.Context) {
	var opts list.ListOptions
	listID, ok := rerr.PathID[list.ProductList](ctx)
	if !ok {
		return
	}

	if err := ctx.BindJSON(&opts); err != nil {
		return
	}

	model, err := h.service.Update(ctx, listID, api.GetUserID(ctx), opts)
	if err != nil {
		h.HandleError(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, model)
}

// @Summary add viewers to product list
// @ID product-list-add-members
// @Tags ProductList
// @Param body body []list.MemberOptions true "users to add"
// @Param id path string true "id of product list"
// @Produce json
// @Router /lists/{id}/members [post]
// @Security ApiKeyAuth
func (h *Handler) AddViewerList(ctx *gin.Context) {
	var members []list.MemberOptions

	listID, ok := rerr.PathID[list.ProductList](ctx)
	if !ok {
		return
	}

	if ok = h.Decode(ctx, &members); !ok {
		return
	}

	model, err := h.service.AppendMembers(ctx, listID, api.GetUserID(ctx), members)
	if err != nil {
		h.HandleError(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, model)
}

// @Summary delete members from product list
// @ID product-list-delete-members
// @Tags ProductList
// @Param body body []string true "id of members to delete"
// @Param id path string true "product list id"
// @Produce json
// @Accept json
// @Router /lists/{id}/members [delete]
// @Security ApiKeyAuth
func (h *Handler) DeleteViewerList(c *gin.Context) {
	var ids []id.ID[user.User]
	listID, ok := rerr.PathID[list.ProductList](c)
	if !ok {
		return
	}

	if ok = h.Decode(c, &ids); !ok {
		return
	}

	model, err := h.service.DeleteMembers(c, listID, api.GetUserID(c), ids)
	if err != nil {
		h.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, model)
}

// @Summary add new products to product list
// @ID product-list-add-products
// @Tags ProductList
// @Param id path string true "product list id"
// @Param body body map[string]ProductStateOptions true "new products"
// @Produce json
// @Accept json
// @Router /lists/{id}/products [post]
// @Security	ApiKeyAuth
func (h *Handler) AddProducts(ctx *gin.Context) {
	var opts map[id.ID[product.Product]]list.ProductStateOptions

	listID, ok := rerr.PathID[list.ProductList](ctx)
	if !ok {
		return
	}

	if ok = h.Decode(ctx, &opts); !ok {
		return
	}

	model, err := h.service.AppendProducts(ctx, listID, api.GetUserID(ctx), opts)
	if err != nil {
		h.HandleError(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, model)
}

// @Summary delete products from product list
// @ID product-list-delete-products
// @Tags ProductList
// @Param id path string true "product list id"
// @Param body body []string true "ids of deleting products"
// @Router /lists/{id}/products [delete]
// @Security ApiKeyAuth
func (h *Handler) DeleteProducts(ctx *gin.Context) {
	var toDelete []id.ID[product.Product]

	listID, ok := rerr.PathID[list.ProductList](ctx)
	if !ok {
		return
	}

	if ok = h.Decode(ctx, &toDelete); !ok {
		return
	}

	model, err := h.service.DeleteProducts(ctx, listID, api.GetUserID(ctx), toDelete)
	if err != nil {
		h.HandleError(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, model)
}

// @Summary change order of products in product list
// @ID product-list-reorder-states
// @Tags ProductList
// @Param id path string true "product list id"
// @Param body body []string true "ids of products in new order"
// @Router /lists/{id}/reorder [patch]
// @Security ApiKeyAuth
func (h *Handler) ReorderState(ctx *gin.Context) {
	var ids []id.ID[product.Product]

	listID, ok := rerr.PathID[list.ProductList](ctx)
	if !ok {
		return
	}

	if ok = h.Decode(ctx, &ids); !ok {
		return
	}

	err := h.service.ReoderStates(ctx, api.GetUserID(ctx), listID, ids)
	if err != nil {
		h.HandleError(ctx, err)
		return
	}

	ctx.Status(http.StatusOK)
}

// @Summary update single product state in given product list
// @ID product-list-update-state
// @Tags ProductList
// @Param id path string true "product list id"
// @Param product_id path string true "product state product id"
// @Param body body ProductStateOptions true "product state options"
// @Router /lists/{id}/products/{product_id} [patch]
// @Security ApiKeyAuth
func (h *Handler) UpdateProductState(ctx *gin.Context) {
	var opts list.ProductStateOptions

	listID, ok := rerr.PathID[list.ProductList](ctx)
	if !ok {
		return
	}
	productID, ok := rerr.Path[product.Product](ctx, "product_id")
	if !ok {
		return
	}

	if ok = h.Decode(ctx, &opts); !ok {
		return
	}

	state, err := h.service.UpdateProductState(ctx, listID, api.GetUserID(ctx), productID, opts)
	if err != nil {
		h.HandleError(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, state)
}
