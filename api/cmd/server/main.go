package main

import (
	"context"
	"flag"
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
	"cloudpass/internal/web"
	"cloudpass/internal/websocket"

	"github.com/labstack/echo/v4"
	echoMiddleware "github.com/labstack/echo/v4/middleware"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var (
	version     = "0.1.0"
	commit      = "dev"
	date        = "now"
	configPath  string
	resetConfig bool
	showVersion bool
)

func init() {
	flag.StringVar(&configPath, "config", "", "path to config file (default: ./config.yaml)")
	flag.BoolVar(&resetConfig, "reset-config", false, "reset config file to defaults")
	flag.BoolVar(&showVersion, "version", false, "show version info")
	flag.Parse()
}

func main() {
	if showVersion {
		mpVersion := config.GetMultipassVersion()
		fmt.Printf("cloudpass %s\n", version)
		fmt.Printf("commit: %s\n", commit)
		fmt.Printf("built: %s\n", date)
		if mpVersion != "" {
			fmt.Printf("multipass %s\n", mpVersion)
		} else {
			fmt.Println("multipass: not found")
		}
		return
	}

	if configPath == "" {
		configPath = "./config.yaml"
	}

	cfg, err := config.EnsureConfig(configPath, resetConfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize config: %v\n", err)
		os.Exit(1)
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
	api.POST("/instances/import", instanceHandler.Import)
	api.GET("/instances/:name", instanceHandler.Get)
	api.DELETE("/instances/:name", instanceHandler.Delete)
	api.POST("/instances/:name/start", instanceHandler.Start)
	api.POST("/instances/:name/stop", instanceHandler.Stop)
	api.POST("/instances/:name/restart", instanceHandler.Restart)
	api.POST("/instances/:name/suspend", instanceHandler.Suspend)
	api.POST("/instances/:name/resume", instanceHandler.Resume)
	api.POST("/instances/:name/export", instanceHandler.Export)
	api.POST("/instances/:name/snapshots", instanceHandler.CreateSnapshot)
	api.GET("/instances/:name/snapshots", instanceHandler.ListSnapshots)
	api.POST("/instances/:name/snapshots/:id/restore", instanceHandler.RestoreSnapshot)
	api.DELETE("/instances/:name/snapshots/:id", instanceHandler.DeleteSnapshot)
	api.GET("/instances/:name/terminal", terminalHandler.HandleTerminal)

	api.GET("/images", imageHandler.List)
	api.GET("/networks", networkHandler.List)
	api.POST("/networks", networkHandler.Create)
	api.DELETE("/networks/:name", networkHandler.Delete)

	e.GET("/*", web.StaticHandler())

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
