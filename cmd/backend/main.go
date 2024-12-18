package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"net"
	"os/signal"
	"syscall"

	"github.com/gin-gonic/gin"
	"github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"

	"go-backend/internal/backend/config"
	"go-backend/internal/backend/swagger"
	api1 "go-backend/internal/backend/user/api"
	"go-backend/internal/backend/user/repo"
	userService "go-backend/internal/backend/user/service"
	"go-backend/pkg/hashing"
)

// @version	0.0.1
// @title		ShoPlanner
// @BasePath /api/v1
func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal().Err(err).Msg("can't load .env file")
	}

	configPath := flag.String("config", "/etc/backend.yaml", "path to config file")
	flag.Parse()

	ctx := context.Background()
	ctx, cancel := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	router := gin.New()
	// router.Use(middleware.Logging)
	router.Use(gin.Recovery())
	router.Use(gin.Logger())

	appCfg, err := config.ParseConfig(*configPath)
	if err != nil {
		log.Fatal().Err(err).Msg("can't load config")
	}
	log.Info().Any("config", appCfg).Msg("Config loaded")

	apiGroup := swagger.Init(router)

	listener, err := net.Listen(appCfg.Service.Net, fmt.Sprintf("%s:%d", appCfg.Service.Host, appCfg.Service.Port))
	if err != nil {
		log.Fatal().Err(err).Msg("can't start listening")
	}

	envCfg, err := config.ParseEnv(ctx)
	if err != nil {
		log.Fatal().Err(err).Msg("can't parse environment")
	}
	log.Debug().Any("env", envCfg).Msg("loaded env")

	doltCfg := mysql.Config{
		User:                 envCfg.Database.User,
		Passwd:               envCfg.Database.Password,
		Net:                  envCfg.Database.Net,
		Addr:                 envCfg.Database.Host,
		DBName:               envCfg.Database.Name,
		AllowNativePasswords: true,
	}

	redisClient := redis.NewClient()

	db, err := sql.Open("mysql", doltCfg.FormatDSN())
	if err != nil {
		log.Fatal().Err(err).Msg("can't connect to database")
	}

	_, err = db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS users (
    id varchar(36) PRIMARY KEY,
    role int NOT NULL,
    login text NOT NULL,
    hash text NOT NULL,
    CONSTRAINT U_Login UNIQUE (id,login)
)`)
	if err != nil {
		log.Error().Err(err).Msg("creating table")
	}

	userDB := repo.NewRepo(db)
	userService := userService.NewService(userDB, hashing.HashMaster{})
	api1.RegisterREST(apiGroup, userService)

	go func() {
		if err = router.RunListener(listener); err != nil {
			log.Fatal().Err(err).Msg("listener returns error")
		}
	}()

	// wait for signal from OS
	<-ctx.Done()

	log.Info().Msg("received interrupt signal")

	if err = listener.Close(); err != nil {
		log.Error().Err(err).Msg("closing listener cause error")
	}
}
