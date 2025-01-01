package rerr

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"go-backend/pkg/myerr"
)

func HandleError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, myerr.ErrInvalidArgument):
		c.JSON(http.StatusBadRequest, err)
		return
	case errors.Is(err, myerr.ErrAlreadyExists):
	case errors.Is(err, myerr.ErrForbidden):
	case errors.Is(err, myerr.ErrNotFound):
	default:
	}
}
