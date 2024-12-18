package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"go-backend/internal/backend/product"
	"go-backend/internal/backend/shopmap"
	"go-backend/internal/backend/shopmap/service"
	"go-backend/internal/backend/user"
	"go-backend/pkg/id"
)

type Handler struct {
	service *service.Service
}

func RegisterREST(r *gin.RouterGroup, service *service.Service) {
	h := Handler{service: service}

	group := r.Group("/shopmap")

	group.POST("/", h.CreateMap)
	group.GET("/user", h.GetCurrentUserList)

	idGroup := group.Group("/id")
	idGroup.GET("/:id", h.GetByID)
	idGroup.DELETE("/:id", h.DeleteMap)
	idGroup.PUT("/:id", h.UpdateMap)
	idGroup.PATCH("/:id/reorder", h.ReorderMap)
}

// @Summary	Creates new shop map
// @ID			shopmap-create
//
// @Tags		ShopMap
// @Accept		json
// @Param		config	body	shopmap.Options	true	"shop map to create"
// @Produce	json
// @Router		/shopmap [post]
func (h *Handler) CreateMap(ctx *gin.Context) {
	var shopMapCfg shopmap.Options

	if err := ctx.ShouldBindJSON(&shopMapCfg); err != nil {
		ctx.String(http.StatusBadRequest, "can't decode request")
		return
	}
	userID := id.ID[user.User]{UUID: uuid.MustParse(ctx.GetString("userId"))}
	shopMap, err := h.service.Create(ctx, userID, shopMapCfg)
	if err != nil {
		ctx.String(http.StatusInternalServerError, "can't create shop map due to internal error")
		return
	}

	ctx.JSON(http.StatusOK, shopMap)
}

// @Summary	Get existing shop map by it's ID
// @ID			shopmap-get-id
//
// @Tags		ShopMap
// @Param		id	query	string	false	"id of shop map"
// @Produce	json
// @Router		/shopmap/id/{id} [get]
func (h *Handler) GetByID(ctx *gin.Context) {
	mapID, err := uuid.Parse(ctx.Query("id"))
	if err != nil {
		ctx.String(http.StatusBadRequest, "id is not valid uuid")
		return
	}

	shopMap, err := h.service.GetByID(ctx, id.ID[shopmap.ShopMap]{UUID: mapID})
	if err != nil {
		ctx.String(http.StatusNotFound, "shop map %s not found", mapID)
		return
	}

	ctx.JSON(http.StatusOK, shopMap)
}

// @Summary	Get shop maps of current logged user
// @ID			shopmap-get-current-user
//
// @Tags		ShopMap
// @Produce	json
// @Router		/shopmap/user [get]
func (h *Handler) GetCurrentUserList(ctx *gin.Context) {
	userID := id.ID[user.User]{UUID: uuid.MustParse(ctx.GetString("userId"))}

	shopMapList, err := h.service.GetByUserID(ctx, userID)
	if err != nil {
		ctx.String(http.StatusInternalServerError, "internal error")
		return
	}

	ctx.JSON(http.StatusOK, shopMapList)
}

// @Summary	Deletes shop map
// @ID			shopmap-delete
//
// @Param		id	query	string	true	"id of shop map"
// @Tags		ShopMap
// @Produce	json
// @Router		/shopmap/id/{id} [delete]
func (h *Handler) DeleteMap(ctx *gin.Context) {
	mapID, err := uuid.Parse(ctx.Query("id"))
	if err != nil {
		ctx.String(http.StatusBadRequest, "id must be valid uuid")
		return
	}
	userID := id.ID[user.User]{UUID: uuid.MustParse(ctx.GetString("userId"))}
	shopMap, err := h.service.DeleteMap(ctx, id.ID[shopmap.ShopMap]{UUID: mapID}, userID)
	if err != nil {
		ctx.String(http.StatusForbidden, "map can be deleted only by owner")
		return
	}

	ctx.JSON(http.StatusOK, shopMap)
}

// @Summary	fully updates shop map
// @ID			shopmap-update
// @Tags		ShopMap
//
// @Param		id		query	string			true	"id of shop map"
// @Param		config	body	shopmap.Options	true	"new configuration"
// @Produce	json
// @Accept		json
// @Router		/shopmap/id/{id} [put]
func (h *Handler) UpdateMap(ctx *gin.Context) {
	var shopMapCfg shopmap.Options
	mapID, err := uuid.Parse(ctx.Query("id"))
	if err != nil {
		ctx.String(http.StatusBadRequest, "id must be valid uuid")
		return
	}

	if err := ctx.BindJSON(&shopMapCfg); err != nil {
		ctx.String(http.StatusBadRequest, "can't decode shop map")
		return
	}

	userID := id.ID[user.User]{UUID: uuid.MustParse(ctx.GetString("userId"))}
	shopMap, err := h.service.UpdateMap(ctx, id.ID[shopmap.ShopMap]{UUID: mapID}, userID, shopMapCfg)
	if err != nil {
		ctx.String(http.StatusBadRequest, "can't update shop map: %s", err.Error())
		return
	}

	ctx.JSON(http.StatusOK, shopMap)
}

type CategoryList struct {
	Categories []product.Category `json:"categories" bson:"categories"`
}

// @Summary	only reorder categories in given shop map
// @ID			shopmap-reorder
// @Tags		ShopMap
//
// @Param		id			query	string		true	"id of shop map"
// @Param		categories	body	[]string	true	"new order of categories"
// @Accept		json
// @Produce	json
// @Router		/shopmap/id/{id}/reorder [patch]
func (h *Handler) ReorderMap(ctx *gin.Context) {
	var categories []product.Category

	mapID, err := uuid.Parse(ctx.Query("id"))
	if err != nil {
		ctx.String(http.StatusBadRequest, "id is not valid")
		return
	}

	if err := ctx.BindJSON(&categories); err != nil {
		ctx.String(http.StatusBadRequest, "can't decode request")
		return
	}
	userID := id.ID[user.User]{UUID: uuid.MustParse(ctx.GetString("userId"))}
	shopMap, err := h.service.ReorderMap(ctx, id.ID[shopmap.ShopMap]{UUID: mapID}, userID, categories)
	if err != nil {
		ctx.String(http.StatusBadRequest, "error: %w", err.Error())
		return
	}

	ctx.JSON(http.StatusOK, shopMap)
}
