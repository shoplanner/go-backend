//nolint:cyclop // main is stronk
package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/x509"
	"database/sql"
	"encoding/pem"
	"flag"
	"fmt"
	"net"
	"os/signal"
	"syscall"

	"github.com/gin-gonic/gin"
	"github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"
	"github.com/redis/rueidis"
	"github.com/rs/zerolog/log"

	"go-backend/internal/backend/auth"
	authAPI "go-backend/internal/backend/auth/api"
	"go-backend/internal/backend/auth/provider"
	authRepo "go-backend/internal/backend/auth/repo"
	authService "go-backend/internal/backend/auth/service"
	"go-backend/internal/backend/config"
	productAPI "go-backend/internal/backend/product/api"
	"go-backend/internal/backend/shopmap/api"
	"go-backend/internal/backend/shopmap/repo"
	"go-backend/internal/backend/shopmap/service"
	swaggerAPI "go-backend/internal/backend/swagger/api"
	userAPI "go-backend/internal/backend/user/api"
	userRepo "go-backend/internal/backend/user/repo"
	userService "go-backend/internal/backend/user/service"
	"go-backend/pkg/hashing"
)

const clientName = "shoplanner"

// @version					0.0.1
// @title						ShoPlanner
// @BasePath					/api/v1
// @securityDefinitions.apikey	ApiKeyAuth
// @in							header
// @name						Auth

//nolint:funlen,gocognit // yes, main is stronk, as it should be
func main() {
	if err := godotenv.Load(); err != nil {
		log.Info().Err(err).Msg("can't load .env file")
	}

	configPath := flag.String("config", "/etc/backend.yaml", "path to config file")

	flag.Parse()

	ctx := context.Background()
	ctx, cancel := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(gin.Logger())

	appCfg, err := config.ParseConfig(*configPath)
	if err != nil {
		log.Fatal().Err(err).Msg("can't load config")
	}
	log.Info().Any("config", appCfg).Msg("Config loaded")

	listener, err := net.Listen(appCfg.Service.Net, fmt.Sprintf("%s:%d", appCfg.Service.Host, appCfg.Service.Port))
	if err != nil {
		log.Fatal().Err(err).Msg("can't start listening")
	}

	envCfg, err := config.ParseEnv(ctx)
	if err != nil {
		log.Fatal().Err(err).Msg("can't parse environment")
	}
	log.Info().Any("env", envCfg).Msg("loaded env")

	authPrivateKey, err := decodeECDSA(envCfg.Auth.PrivateKey)
	if err != nil {
		log.Fatal().Err(err).Msg("can't parse private key for JWT tokens")
	}

	doltCfg := mysql.Config{
		User:                 envCfg.Database.User,
		Passwd:               envCfg.Database.Password,
		Net:                  envCfg.Database.Net,
		Addr:                 envCfg.Database.Host,
		AllowNativePasswords: true,
		ParseTime:            true,
	}

	redisClient, err := rueidis.NewClient(rueidis.ClientOption{
		Username:    envCfg.Redis.User,
		Password:    envCfg.Redis.Password,
		InitAddress: []string{envCfg.Redis.Addr},
		ClientName:  clientName,
	})
	if err != nil {
		log.Fatal().Err(err).Msg("creating redis client")
	}

	sqlDB, err := sql.Open("mysql", doltCfg.FormatDSN())
	if err != nil {
		log.Fatal().Err(err).Msg("can't connect to database")
	}

	if err = sqlDB.PingContext(ctx); err != nil {
		log.Fatal().Err(err).Caller().Stack().Msg("ping DB")
	} else {
		log.Info().Caller().Msg("DoltDB ping OK")
	}

	_, err = sqlDB.ExecContext(ctx, fmt.Sprintf("create database if not exists %s", envCfg.Database.Name))
	if err != nil {
		log.Fatal().Err(err).Msg("initializing database")
	}
	_, err = sqlDB.ExecContext(ctx, fmt.Sprintf(`USE %s`, envCfg.Database.Name))
	if err != nil {
		log.Fatal().Err(err).Msg("can't use database")
	}
	_, err = sqlDB.ExecContext(ctx, "set global local_infile=1")
	if err != nil {
		log.Fatal().Err(err).Msg("enabling load data in DB")
	}

	userDB, err := userRepo.NewRepo(ctx, sqlDB)
	if err != nil {
		log.Fatal().Err(err).Msg("initializing user repo")
	}

	accessRepo, err := authRepo.NewRedisRepo[auth.AccessToken](ctx, redisClient)
	if err != nil {
		log.Fatal().Err(err).Msg("initializing access tokens repo")
	}
	refreshRepo, err := authRepo.NewRedisRepo[auth.RefreshToken](ctx, redisClient)
	if err != nil {
		log.Fatal().Err(err).Msg("initializing refresh tokens repo")
	}
	shopMapRepo, err := repo.NewShopMapRepo(ctx, sqlDB)
	if err != nil {
		log.Fatal().Err(err).Msg("can't initialize shop map storage")
	}

	// business logic

	userService := userService.NewService(userDB, hashing.HashMaster{})
	authService := authService.New(
		userService,
		refreshRepo,
		accessRepo,
		provider.NewJWT(authPrivateKey),
		authService.Options{
			AccessTokenExpires:  appCfg.Auth.AccessTokenLiveTime,
			RefreshTokenExpires: appCfg.Auth.RefreshTokenLiveTime,
		},
	)
	shopMapService := service.NewService(userService, shopMapRepo)

	// API

	jwtMiddleware := authAPI.NewAuthMiddleware(authService)
	apiGroup := swaggerAPI.Init(router)
	authAPI.RegisterREST(apiGroup, authService, jwtMiddleware)

	userAPI.RegisterREST(apiGroup, userService, jwtMiddleware)

	apiGroup.Use(jwtMiddleware.Middleware())

	productAPI.RegisterREST(apiGroup, nil)
	api.RegisterREST(apiGroup, shopMapService)

	go func() {
		if err = router.RunListener(listener); err != nil && ctx.Err() == nil {
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

func decodeECDSA(pemEncoded string) (*ecdsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(pemEncoded))
	privateKey, err := x509.ParsePKCS8PrivateKey(block.Bytes)

	return privateKey.(*ecdsa.PrivateKey), err
}
