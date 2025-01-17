package rerr

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"go-backend/pkg/myerr"
)

func ResponseMiddleware(c *gin.Context) {
	c.Next()
}

type GeneralResponse struct {
	Data  any   `json:"data"`
	Error error `json:"error,omitempty"`
}

func HandleError(c *gin.Context, err error) {
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
