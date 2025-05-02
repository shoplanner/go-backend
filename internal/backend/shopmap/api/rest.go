package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"go-backend/internal/backend/auth/api"
	"go-backend/internal/backend/product"
	"go-backend/internal/backend/shopmap"
	"go-backend/internal/backend/shopmap/service"
	"go-backend/pkg/id"
	"go-backend/pkg/myerr"
)

type Handler struct {
	service *service.Service
}

func RegisterREST(r *gin.RouterGroup, service *service.Service) {
	h := Handler{service: service}

	group := r.Group("/shopmap")

	group.POST("", h.CreateMap)
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
// @Security ApiKeyAuth
func (h *Handler) CreateMap(ctx *gin.Context) {
	var shopMapCfg shopmap.Options

	if err := ctx.ShouldBindJSON(&shopMapCfg); err != nil {
		ctx.String(http.StatusBadRequest, "can't decode request")
		return
	}
	shopMap, err := h.service.Create(ctx, api.GetUserID(ctx), shopMapCfg)
	if err != nil {
		log.Err(err).Msg("creating shop map")
		ctx.String(http.StatusInternalServerError, "internal error")
		return
	}

	ctx.JSON(http.StatusOK, shopMap)
}

// @Summary	Get existing shop map by it's ID
// @ID			shopmap-get-id
//
// @Tags		ShopMap
// @Param		id	path	string	false	"id of shop map"
// @Produce	json
// @Router		/shopmap/id/{id} [get]
// @Security ApiKeyAuth
func (h *Handler) GetByID(ctx *gin.Context) {
	mapID, err := uuid.Parse(ctx.Param("id"))
	if err != nil {
		ctx.String(http.StatusBadRequest, "id is not valid uuid")
		return
	}

	shopMap, err := h.service.GetByID(ctx, id.ID[shopmap.ShopMap]{UUID: mapID})
	if errors.Is(err, myerr.ErrNotFound) {
		ctx.String(http.StatusNotFound, "shop map %s not found", mapID)
		return
	} else if err != nil {
		log.Err(err).Msg("getting shop map")
		ctx.String(http.StatusInternalServerError, "internal error")

		return
	}

	ctx.JSON(http.StatusOK, shopMap)
}

// @Summary	Get shop maps of current logged user
// @ID			shopmap-get-current-user
//
// @Tags		ShopMap
// @Produce	json
// @Security ApiKeyAuth
// @Router		/shopmap/user [get]
func (h *Handler) GetCurrentUserList(ctx *gin.Context) {
	shopMapList, err := h.service.GetByUserID(ctx, api.GetUserID(ctx))
	if err != nil {
		log.Err(err).Msg("get user's shop maps")
		ctx.String(http.StatusInternalServerError, "internal error")

		return
	}

	ctx.JSON(http.StatusOK, shopMapList)
}

// @Summary	Deletes shop map
// @ID			shopmap-delete
//
// @Param		id	path	string	true	"id of shop map"
// @Tags		ShopMap
// @Produce	json
// @Security ApiKeyAuth
// @Router		/shopmap/id/{id} [delete]
func (h *Handler) DeleteMap(ctx *gin.Context) {
	mapID, err := uuid.Parse(ctx.Param("id"))
	if err != nil {
		ctx.String(http.StatusBadRequest, "id must be valid uuid")
		return
	}
	shopMap, err := h.service.DeleteMap(ctx, id.ID[shopmap.ShopMap]{UUID: mapID}, api.GetUserID(ctx))

	if errors.Is(err, myerr.ErrForbidden) {
		ctx.String(http.StatusForbidden, err.Error())

		return
	} else if err != nil {
		log.Err(err).Msg("deleting shop map")
		ctx.String(http.StatusInternalServerError, "internal error")

		return
	}

	ctx.JSON(http.StatusOK, shopMap)
}

// @Summary	fully updates shop map
// @ID			shopmap-update
// @Tags		ShopMap
//
// @Param		id		path	string			true	"id of shop map"
// @Param		config	body	shopmap.Options	true	"new configuration"
// @Produce	json
// @Accept		json
// @Router		/shopmap/id/{id} [put]
// @Security ApiKeyAuth
func (h *Handler) UpdateMap(ctx *gin.Context) {
	var shopMapCfg shopmap.Options

	mapID, err := uuid.Parse(ctx.Param("id"))
	if err != nil {
		ctx.String(http.StatusBadRequest, "id must be valid uuid")
		return
	}

	if err = ctx.BindJSON(&shopMapCfg); err != nil {
		ctx.String(http.StatusBadRequest, "can't decode shop map")
		return
	}

	shopMap, err := h.service.UpdateMap(ctx, id.ID[shopmap.ShopMap]{UUID: mapID}, api.GetUserID(ctx), shopMapCfg)
	if errors.Is(err, myerr.ErrInvalidArgument) {
		ctx.String(http.StatusBadRequest, "can't update shop map: %s", err.Error())
		return
	}

	ctx.JSON(http.StatusOK, shopMap)
}

type CategoryList struct {
	Categories []product.Category `json:"categories"`
}

// @Summary	only reorder categories in given shop map
// @ID			shopmap-reorder
// @Tags		ShopMap
//
// @Param		id			path	string		true	"id of shop map"
// @Param		categories	body	[]string	true	"new order of categories"
// @Accept		json
// @Produce	json
// @Router		/shopmap/id/{id}/reorder [patch]
// @Security ApiKeyAuth
func (h *Handler) ReorderMap(c *gin.Context) {
	var categories []product.Category

	mapID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.String(http.StatusBadRequest, "id is not valid")
		return
	}

	if err = c.BindJSON(&categories); err != nil {
		c.String(http.StatusBadRequest, "can't decode request")
		return
	}
	shopMap, err := h.service.ReorderMap(c, id.ID[shopmap.ShopMap]{UUID: mapID}, api.GetUserID(c), categories)
	if err != nil {
		c.String(http.StatusBadRequest, "error: %s", err.Error())
		return
	}

	c.JSON(http.StatusOK, shopMap)
}
