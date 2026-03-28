package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"cloudpass/internal/config"
	"cloudpass/internal/handlers"
	"cloudpass/internal/middleware"
	"cloudpass/internal/multipass"
	"cloudpass/internal/websocket"

	"github.com/labstack/echo/v4"
	echoMiddleware "github.com/labstack/echo/v4/middleware"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	configPath := os.Getenv("CLOUDPASS_CONFIG")
	if configPath == "" {
		configPath = "./config.yaml"
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		log.Warn().Err(err).Msg("failed to load config, using defaults")
		cfg = &config.Config{
			Server: config.ServerConfig{
				Host: "0.0.0.0",
				Port: 8080,
			},
		}
	}

	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	switch cfg.Logging.Level {
	case "debug":
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	case "warn":
		zerolog.SetGlobalLevel(zerolog.WarnLevel)
	case "error":
		zerolog.SetGlobalLevel(zerolog.ErrorLevel)
	default:
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}

	log.Info().Msg("starting cloudpass API server")

	mpClient := multipass.NewClient(cfg.Multipass.DefaultTimeoutSec)

	instanceHandler := handlers.NewInstanceHandler(mpClient)
	imageHandler := handlers.NewImageHandler(mpClient)
	networkHandler := handlers.NewNetworkHandler(mpClient)
	healthHandler := handlers.NewHealthHandler()
	terminalHandler := websocket.NewTerminalHandler(mpClient)

	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	e.Use(echoMiddleware.Recover())
	e.Use(middleware.Logging())

	e.Use(echoMiddleware.CORS())

	e.GET("/api/health", healthHandler.Health)

	api := e.Group("/api")
	api.Use(middleware.IPWhitelist(cfg.Security.AllowedIPs))

	api.GET("/instances", instanceHandler.List)
	api.POST("/instances", instanceHandler.Create)
	api.GET("/instances/:name", instanceHandler.Get)
	api.DELETE("/instances/:name", instanceHandler.Delete)
	api.POST("/instances/:name/start", instanceHandler.Start)
	api.POST("/instances/:name/stop", instanceHandler.Stop)
	api.POST("/instances/:name/restart", instanceHandler.Restart)
	api.POST("/instances/:name/suspend", instanceHandler.Suspend)
	api.POST("/instances/:name/resume", instanceHandler.Resume)
	api.GET("/instances/:name/terminal", terminalHandler.HandleTerminal)

	api.GET("/images", imageHandler.List)
	api.GET("/networks", networkHandler.List)
	api.POST("/networks", networkHandler.Create)
	api.DELETE("/networks/:name", networkHandler.Delete)

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	srv := &http.Server{
		Addr:         addr,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	go func() {
		log.Info().Str("addr", addr).Msg("server listening")
		if err := e.StartServer(srv); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("server failed")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info().Msg("shutting down server")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := e.Shutdown(ctx); err != nil {
		log.Fatal().Err(err).Msg("server forced to shutdown")
	}

	log.Info().Msg("server exited")
}
