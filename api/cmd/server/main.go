package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"cloudpass/internal/config"
	"cloudpass/internal/handlers"
	"cloudpass/internal/logger"
	"cloudpass/internal/middleware"
	"cloudpass/internal/multipass"
	"cloudpass/internal/web"
	"cloudpass/internal/websocket"

	"github.com/labstack/echo/v4"
	echoMiddleware "github.com/labstack/echo/v4/middleware"
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
		homeDir, err := os.UserHomeDir()
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to get home directory: %v\n", err)
			os.Exit(1)
		}
		configPath = filepath.Join(homeDir, ".cloudpass", "config.yaml")
	}

	cfg, err := config.EnsureConfig(configPath, resetConfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize config: %v\n", err)
		os.Exit(1)
	}

	if err := logger.Init(cfg.Logging); err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize logger: %v\n", err)
		os.Exit(1)
	}

	log.Info().Msg("starting cloudpass API server")

	mpClient := multipass.NewClient(cfg.Multipass.DefaultTimeoutSec)

	jobStorage, err := handlers.NewJobStorage(filepath.Join(filepath.Dir(configPath), "data"))
	if err != nil {
		log.Fatal().Err(err).Msg("failed to initialize job storage")
	}

	eventHub := handlers.NewEventHub()

	instanceHandler := handlers.NewInstanceHandler(mpClient)
	imageHandler := handlers.NewImageHandler(mpClient)
	networkHandler := handlers.NewNetworkHandler(mpClient)
	healthHandler := handlers.NewHealthHandler()
	configHandler := handlers.NewConfigHandler(configPath)
	terminalHandler := websocket.NewTerminalHandler(mpClient)
	jobHandler := handlers.NewJobHandler(mpClient, cfg.Multipass.DefaultTimeoutSec, jobStorage, eventHub)

	jobStorage.Cleanup(24 * time.Hour)

	go func() {
		for {
			jobStorage.Cleanup(24 * time.Hour)
			time.Sleep(time.Hour)
		}
	}()

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
	api.POST("/instances/async", jobHandler.CreateInstanceAsync)
	api.POST("/instances/import", instanceHandler.Import)
	api.GET("/instances/:name", instanceHandler.Get)
	api.GET("/instances/:name/state", instanceHandler.GetState)
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
	api.POST("/instances/:name/mounts", instanceHandler.Mount)
	api.DELETE("/instances/:name/mounts", instanceHandler.Unmount)
	api.GET("/instances/:name/terminal", terminalHandler.HandleTerminal)

	api.GET("/images", imageHandler.List)
	api.GET("/networks", networkHandler.List)
	api.POST("/networks", networkHandler.Create)
	api.DELETE("/networks/:name", networkHandler.Delete)
	api.GET("/jobs", jobHandler.List)
	api.GET("/jobs/:id", jobHandler.Get)
	api.GET("/jobs/stream", jobHandler.Stream)
	api.GET("/config", configHandler.Get)
	api.POST("/config", configHandler.Update)

	e.GET("/*", web.StaticHandlerWithFallback())

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	srv := &http.Server{
		Addr:         addr,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	printBanner(cfg.Server.Host, cfg.Server.Port)

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

func printBanner(host string, port int) {
	fmt.Println("")
	fmt.Println("CloudPass is running!")
	fmt.Println("")

	urls := []string{}

	if host == "0.0.0.0" || host == "127.0.0.1" || host == "localhost" {
		urls = append(urls, fmt.Sprintf("  Web UI:  http://localhost:%d", port))
		urls = append(urls, fmt.Sprintf("  API:     http://localhost:%d/api", port))

		if localIP := getLocalIP(); localIP != "" {
			urls = append(urls, fmt.Sprintf("  Web UI:  http://%s:%d", localIP, port))
			urls = append(urls, fmt.Sprintf("  API:      http://%s:%d/api", localIP, port))
		}
	} else {
		urls = append(urls, fmt.Sprintf("  Web UI:  http://%s:%d", host, port))
		urls = append(urls, fmt.Sprintf("  API:     http://%s:%d/api", host, port))
	}

	for _, u := range urls {
		fmt.Println(u)
	}

	fmt.Println("")
	fmt.Println("Press Ctrl+C to stop")
	fmt.Println("")
}

func getLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}

	for _, addr := range addrs {
		if ipNet, ok := addr.(*net.IPNet); ok && ipNet.IP.To4() != nil {
			if !ipNet.IP.IsLoopback() {
				return ipNet.IP.String()
			}
		}
	}

	return ""
}
