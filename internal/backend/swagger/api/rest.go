package api

import (
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	_ "go-backend/docs"
)

func Init(r *gin.Engine) *gin.RouterGroup {
	g := r.Group("/api/v1")
	g.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	return g
}
