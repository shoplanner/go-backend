package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

func Logging(ctx *gin.Context) {
	data, _ := ctx.GetRawData()

	log.Info().
		Str("method", ctx.Request.Method).
		Str("path", ctx.Request.URL.Path).
		Str("client_ip", ctx.ClientIP()).Bytes("request", data).Msg("Request")
}
