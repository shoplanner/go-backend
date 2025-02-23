package api

import (
	"go-backend/internal/backend/auth/api"
	"go-backend/internal/backend/list"
	"go-backend/internal/backend/list/service"
	"go-backend/internal/backend/product"
	"go-backend/internal/backend/user"
	"go-backend/pkg/api/rest/rerr"
	"go-backend/pkg/id"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
)

type Handler struct {
	rerr.BaseHandler

	service *service.Service
}

func New(log zerolog.Logger, service *service.Service) *Handler {
	log = log.With().Str("component", "product list handler").Logger()
	return &Handler{
		BaseHandler: rerr.NewBaseHandler(log),
		service:     service,
	}
}

func RegisterREST(r *gin.RouterGroup, service *service.Service) {
	group := r.Group("/lists")

	h := Handler{service: service}

	r.POST("", h.CreateList)

	group.GET("/:id", h.GetByListID)
	group.DELETE("/:id", h.Delete)
	group.POST("/:id/products", h.AddProducts)
	group.DELETE("/:id/products", h.DeleteProducts)
	group.POST("/:id/members", h.AddViewerList)
	group.PUT("/:id", h.Update)
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
	if err := ctx.BindJSON(&opts); err != nil {
		ctx.String(http.StatusBadRequest, "can't decode request")
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
func (h *Handler) Get(ctx *gin.Context) {
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
// @Securty ApiAuthKey
func (h *Handler) AddViewerList(ctx *gin.Context) {
	var members []list.MemberOptions

	listID, ok := rerr.PathID[list.ProductList](ctx)
	if !ok {
		return
	}

	if ok := h.Decode(ctx, &members); ok {
		return
	}

	model, err := h.service.AppendMembers(ctx, listID, api.GetUserID(ctx), members)
	if err != nil {
		h.HandleError(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, model)
}

// @Summary delete viewers from product list
// @ID product-list-delete-members
// @Tags ProductList
// @Param body body []id.ID[user.User]
// @Param id path string true "product list id"
// @Produce json
// @Accept json
// @Router /lists/{id}/members
// @Securty ApiAuthKey
func (h *Handler) DeleteViewerList(c *gin.Context) {
	var ids []id.ID[user.User]
	listID, ok := rerr.PathID[list.ProductList](c)
	if !ok {
		return
	}

	if ok := h.Decode(c, &ids); !ok {
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
// @Param body body map[id.ID[product.Product]]list.ProductStateOptions "new products"
// @Produce json
// @Accept json
// @Router /lists/{id}/products
// @Securty ApiAuthKey
func (h *Handler) AddProducts(ctx *gin.Context) {
	var opts map[id.ID[product.Product]]list.ProductStateOptions

	listID, ok := rerr.PathID[list.ProductList](ctx)
	if !ok {
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
// @Param body body []id.ID[product.Product] "ids of deleting products"
// @Router /lists/{id}/products
// @Security ApiAuthKey
func (h *Handler) DeleteProducts(ctx *gin.Context) {
	var toDelete []id.ID[product.Product]

	listID, ok := rerr.PathID[list.ProductList](ctx)
	if !ok {
		return
	}

	if ok := h.Decode(ctx, &toDelete); !ok {
		return
	}

	model, err := h.service.DeleteProducts(ctx, listID, api.GetUserID(ctx), toDelete)
	if err != nil {
		h.HandleError(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, model)
}
