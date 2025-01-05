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
		c.JSON(http.StatusBadRequest, err)
		return
	case errors.Is(err, myerr.ErrAlreadyExists):
		c.JSON(http.StatusGone, err.Error())
	case errors.Is(err, myerr.ErrForbidden):
	case errors.Is(err, myerr.ErrNotFound):
	default:
	}
}
