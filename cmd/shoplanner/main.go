package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"go-backend/internal/handler"
	businessProducts "go-backend/internal/product"
	"go-backend/pkg/middleware"
)

// @version	0.0.1
// @title		ShoPlanner
//
// @host		localhost:3000
func main() {
	uri := os.Getenv("MONGO_URI")

	ctx := context.Background()
	ctx, cancel := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	dbOptions := options.Client().ApplyURI(uri)

	dbClient, err := mongo.Connect(ctx, dbOptions)
	if err != nil {
		log.Fatal().Err(err)
	}

	products := dbClient.Database("shoplanner").Collection("products")

	productRepo := businessProducts.NewRepo(products)

	productService := businessProducts.NewService(productRepo)

	router := gin.New()
	router.Use(middleware.Logging)
	router.Use(gin.Recovery())
	gin.Logger()

	handler.Init(router, productService)

	host := os.Getenv("SHOPLANNER_HOST")
	port := os.Getenv("SHOPLANNER_PORT")

	listener, err := net.Listen("tcp", fmt.Sprintf("%s:%s", host, port))
	if err != nil {
		log.Fatal().Err(err)
	}
	log.Printf("ShoPlanner started on %s:%s...\n", host, port)
	go func() {
		if err := router.RunListener(listener); err != nil {
			log.Fatal().Err(err)
		}
	}()

	// wait for signal from OS
	<-ctx.Done()

	if err := listener.Close(); err != nil {
		log.Fatal().Err(err)
	}
}
