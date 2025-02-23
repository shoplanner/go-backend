package rerr

import (
	"errors"
	"go-backend/pkg/id"
	"go-backend/pkg/myerr"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

func ResponseMiddleware(c *gin.Context) {
	c.Next()
}

type GeneralResponse struct {
	Data  any   `json:"data"`
	Error error `json:"error,omitempty"`
}

type BaseHandler struct {
	log zerolog.Logger
}

func NewBaseHandler(log zerolog.Logger) BaseHandler {
	return BaseHandler{log: log}
}

func (h BaseHandler) HandleError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, myerr.ErrInvalidArgument):
		c.String(http.StatusBadRequest, err.Error())
		return
	case errors.Is(err, myerr.ErrAlreadyExists):
		c.String(http.StatusGone, err.Error())
		return
	case errors.Is(err, myerr.ErrForbidden):
		c.String(http.StatusForbidden, err.Error())
		return
	case errors.Is(err, myerr.ErrNotFound):
		c.String(http.StatusNotFound, err.Error())
		return
	default:
		c.String(http.StatusInternalServerError, "internal error")
		return
	}
}

func PathID[T any](ctx *gin.Context) (id.ID[T], bool) {
	rawID := ctx.Param("id")

	uuidID, err := uuid.Parse(rawID)
	if err != nil {
		ctx.String(http.StatusBadRequest, "id must be valid UUID")
		return id.ID[T]{}, false
	}

	return id.ID[T]{UUID: uuidID}, true
}

func QueryID[T any](ctx *gin.Context, name string) (id.ID[T], bool) {
	rawID := ctx.Param(name)

	uuidID, err := uuid.Parse(rawID)
	if err != nil {
		ctx.String(http.StatusBadRequest, "%s must be valid UUID", name)
		return id.ID[T]{}, false
	}

	return id.ID[T]{UUID: uuidID}, true
}

func (h BaseHandler) Decode(c *gin.Context, obj any) bool {
	if err := c.BindJSON(obj); err != nil {
		h.log.Info().Ctx(c).Err(err).Msg("decoding request failed")
		c.String(http.StatusBadRequest, "can't decode request")
		return false
	}

	return true
}
