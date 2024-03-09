package handler

import (
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	"go-backend/internal/business/product"

	_ "go-backend/docs"
)

func Init(r *gin.Engine, productService *product.Service) {
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	products := NewProductController(productService)

	productGroup := r.Group("product")
	products.InitRoutes(productGroup)
}
