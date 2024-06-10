package handler

import (
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	_ "go-backend/docs"

	"go-backend/internal/product"
)

func Init(r *gin.Engine, productService *product.Service) {
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	products := product.NewProductController(productService)

	productGroup := r.Group("product")
	products.InitRoutes(productGroup)
}
