//nolint:cyclop // main is stronk
package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/x509"
	"database/sql"
	"encoding/pem"
	"errors"
	"flag"
	"fmt"
	"log"
	"log/syslog"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	_ "modernc.org/sqlite"

	authAPI "go-backend/internal/backend/auth/api"
	"go-backend/internal/backend/auth/provider"
	authService "go-backend/internal/backend/auth/service"
	"go-backend/internal/backend/config"
	favoritesAPI "go-backend/internal/backend/favorite/api"
	favoritesRepo "go-backend/internal/backend/favorite/repo"
	favoritesService "go-backend/internal/backend/favorite/service"
	listAPI "go-backend/internal/backend/list/api"
	listRepo "go-backend/internal/backend/list/repo"
	listService "go-backend/internal/backend/list/service"
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
	"go-backend/pkg/hashing"
)

// @version					0.0.1
// @title						ShoPlanner
// @BasePath					/api/v1
// @securityDefinitions.apikey	ApiKeyAuth
// @in							header
// @name						Auth

//nolint:funlen,gocognit // yes, main is stronk, as it should be
func main() {
	configPath := flag.String("config", "/etc/backend.yaml", "path to config file")

	flag.Parse()

	ctx := context.Background()

	envCfg, err := config.ParseEnv(ctx)
	if err != nil {
		log.Fatalf("can't parse environment: %v", err)
	}

	parentLogger, err := makeLogger(envCfg.Logging.Writer)
	if err != nil {
		log.Fatalf("can't initialize logger: %v", err)
	}
	zerolog.SetGlobalLevel(zerolog.InfoLevel)

	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(gin.Logger())

	appCfg, err := config.ParseConfig(*configPath)
	if err != nil {
		parentLogger.Fatal().Err(err).Msg("can't load config")
	}
	parentLogger.Info().Any("config", appCfg).Msg("loaded")

	listener, err := net.Listen(appCfg.Service.Net, fmt.Sprintf("%s:%d", appCfg.Service.Host, appCfg.Service.Port))
	if err != nil {
		parentLogger.Fatal().Err(err).Msg("can't start listening")
	}

	parentLogger.Info().Any("env", envCfg).Msg("loaded env")

	privateKey, err := os.ReadFile(envCfg.Auth.PrivateKey)
	if err != nil {
		parentLogger.Err(err).Str("path", envCfg.Auth.PrivateKey).Msg("failed to read private key file")
	}

	authPrivateKey, err := decodeECDSA(string(privateKey))
	if err != nil {
		parentLogger.Fatal().Err(err).Msg("can't parse private key for JWT tokens")
	}

	ctx, cancel := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	db, err := sql.Open("sqlite", envCfg.Database.Path)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	userDB, err := userRepo.NewRepo(ctx, db)
	if err != nil {
		parentLogger.Fatal().Err(err).Msg("initializing user repo")
	}
	shopMapRepo, err := shopMapRepo.NewShopMapRepo(ctx, db)
	if err != nil {
		parentLogger.Fatal().Err(err).Msg("can't initialize shop map storage")
	}
	productRepo, err := productRepo.NewRepo(ctx, db)
	if err != nil {
		parentLogger.Fatal().Err(err).Msg("can't initialize product storage")
	}
	favoritesRepo, err := favoritesRepo.NewRepo(ctx, db)
	if err != nil {
		parentLogger.Fatal().Err(err).Msg("initializing favorites repo")
	}
	listRepo, err := listRepo.NewRepo(ctx, db)
	if err != nil {
		parentLogger.Fatal().Err(err).Msg("initalizing list repo")
	}

	// business logic
	userService := userService.NewService(userDB, hashing.HashMaster{})
	authService := authService.New(
		parentLogger,
		userService,
		provider.NewJWT(authPrivateKey),
		authService.Options{
			AccessTokenExpires:  appCfg.Auth.AccessTokenLiveTime,
			RefreshTokenExpires: appCfg.Auth.RefreshTokenLiveTime,
		},
	)
	shopMapService := shopMapService.NewService(parentLogger, userService, shopMapRepo)
	productService := productService.NewService(productRepo)
	favoriteService := favoritesService.NewService(favoritesRepo, userService)
	listService := listService.NewService(listRepo, parentLogger)

	// API

	jwtMiddleware := authAPI.NewAuthMiddleware(parentLogger, authService)

	apiGroup := swaggerAPI.Init(router)

	// overrides role model
	authAPI.RegisterREST(apiGroup, authService, jwtMiddleware)
	userAPI.RegisterREST(apiGroup, userService, jwtMiddleware)

	apiGroup.Use(jwtMiddleware.Middleware())

	productAPI.RegisterREST(apiGroup, productService)
	shopMapAPI.RegisterREST(apiGroup, shopMapService, parentLogger)
	favoritesAPI.RegisterREST(apiGroup, favoriteService, parentLogger.With().Logger())
	listAPI.RegisterREST(apiGroup, listService, parentLogger)

	listAPI.RegisterWebSocket(apiGroup, listService, parentLogger)

	go func() {
		if err = router.RunListener(listener); err != nil && ctx.Err() == nil {
			parentLogger.Fatal().Err(err).Msg("listener returns error")
		}
	}()

	// wait for signal from OS
	<-ctx.Done()

	parentLogger.Info().Msg("received interrupt signal")

	if err = listener.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
		parentLogger.Error().Err(err).Msg("closing listener cause error")
	}

	parentLogger.Info().Msg("server stopped")
}

func makeLogger(writerType string) (zerolog.Logger, error) {
	if strings.EqualFold(writerType, "stdout") {
		return zerolog.New(os.Stdout).With().Timestamp().Caller().Logger(), nil
	}

	writer, err := syslog.New(syslog.LOG_DEBUG, os.Args[0])
	if err != nil {
		return zerolog.Logger{}, err
	}

	return zerolog.New(zerolog.SyslogLevelWriter(writer)).With().Timestamp().Caller().Logger(), nil
}

var ErrUnexpectedPrivateKeyType = errors.New("provided private key is not ECDSA")

func decodeECDSA(pemEncoded string) (*ecdsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(pemEncoded))
	if block == nil || len(block.Bytes) == 0 {
		return nil, errors.New("private key is invalid")
	}

	privateKey, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	parsedKey, parsed := privateKey.(*ecdsa.PrivateKey)
	if !parsed {
		return nil, ErrUnexpectedPrivateKeyType
	}

	return parsedKey, nil
}
