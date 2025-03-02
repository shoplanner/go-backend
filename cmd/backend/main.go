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
	stdLog "log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/gin-gonic/gin"
	"github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"
	"github.com/redis/rueidis"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	gormMySQL "gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"go-backend/internal/backend/auth"
	authAPI "go-backend/internal/backend/auth/api"
	"go-backend/internal/backend/auth/provider"
	authRepo "go-backend/internal/backend/auth/repo"
	authService "go-backend/internal/backend/auth/service"
	"go-backend/internal/backend/config"
	favoritesAPI "go-backend/internal/backend/favorite/api"
	favoritesRepo "go-backend/internal/backend/favorite/repo"
	favoritesService "go-backend/internal/backend/favorite/service"
	productAPI "go-backend/internal/backend/product/api"
	productRepo "go-backend/internal/backend/product/repo"
	productService "go-backend/internal/backend/product/service"
	shopMapAPI "go-backend/internal/backend/shopmap/api"
	shopMapRepo "go-backend/internal/backend/shopmap/repo"
	shopMapService "go-backend/internal/backend/shopmap/service"
	swaggerAPI "go-backend/internal/backend/swagger/api"
	userAPI "go-backend/internal/backend/user/api"
	userRepo "go-backend/internal/backend/user/repo"
	userService "go-backend/internal/backend/user/service"
	"go-backend/pkg/bd"
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
	parentLogger := zerolog.New(os.Stdout).With().Timestamp().Caller().Logger()

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

	gormLog := logger.New(stdLog.New(os.Stdout, "\n", stdLog.LstdFlags), logger.Config{
		Colorful:                  true,
		IgnoreRecordNotFoundError: false,
		ParameterizedQueries:      false,
		LogLevel:                  logger.Info,
	})

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

	// nolint:exhaustruct
	doltCfg := mysql.Config{
		User:                 envCfg.Database.User,
		Passwd:               envCfg.Database.Password,
		Net:                  envCfg.Database.Net,
		Addr:                 envCfg.Database.Host,
		AllowNativePasswords: true,
		ParseTime:            true,
	}

	// nolint:exhaustruct
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
	sqlAdapter := bd.NewDB(sqlDB, parentLogger.With().Logger())

	doltCfg.DBName = envCfg.Database.Name
	gormDB, err := gorm.Open(
		gormMySQL.Open(doltCfg.FormatDSN()),
		// nolint:exhaustruct
		&gorm.Config{Logger: gormLog},
	)
	if err != nil {
		log.Fatal().Err(err).Msg("gorm: connecting to database")
	}

	if err = sqlDB.PingContext(ctx); err != nil {
		log.Fatal().Err(err).Caller().Stack().Msg("ping DB")
	} else {
		log.Info().Caller().Msg("DoltDB ping OK")
	}

	_, err = sqlAdapter.ExecContext(ctx, fmt.Sprintf("create database if not exists %s", envCfg.Database.Name))
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

	userDB, err := userRepo.NewRepo(ctx, sqlAdapter, gormDB)
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
	shopMapRepo, err := shopMapRepo.NewShopMapRepo(ctx, sqlDB)
	if err != nil {
		log.Fatal().Err(err).Msg("can't initialize shop map storage")
	}
	productRepo, err := productRepo.NewGormRepo(ctx, gormDB)
	if err != nil {
		log.Fatal().Err(err).Msg("can't initialize product storage")
	}
	favoritesRepo, err := favoritesRepo.NewRepo(ctx, gormDB)
	if err != nil {
		parentLogger.Fatal().Err(err).Msg("initializing favorites repo")
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
	shopMapService := shopMapService.NewService(userService, shopMapRepo)
	productService := productService.NewService(productRepo)
	favoriteService := favoritesService.NewService(favoritesRepo, userService)

	// API

	jwtMiddleware := authAPI.NewAuthMiddleware(authService)

	apiGroup := swaggerAPI.Init(router)

	// overrides role model
	authAPI.RegisterREST(apiGroup, authService, jwtMiddleware)
	userAPI.RegisterREST(apiGroup, userService, jwtMiddleware)

	apiGroup.Use(jwtMiddleware.Middleware())

	productAPI.RegisterREST(apiGroup, productService)
	shopMapAPI.RegisterREST(apiGroup, shopMapService)
	favoritesAPI.RegisterREST(apiGroup, favoriteService, parentLogger.With().Logger())

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
