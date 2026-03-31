package handlers

import (
	"net/http"
	"sync"

	"cloudpass/internal/config"
	"cloudpass/internal/logger"
	"cloudpass/internal/models"

	"github.com/labstack/echo/v4"
)

type ConfigHandler struct {
	configPath string
	mu         sync.RWMutex
}

func NewConfigHandler(configPath string) *ConfigHandler {
	return &ConfigHandler{configPath: configPath}
}

func (h *ConfigHandler) Get(c echo.Context) error {
	cfg, err := config.Load(h.configPath)
	if err != nil {
		logger.API.Error().Err(err).Msg("failed to load config")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "internal_error",
			Message: "failed to load configuration",
		})
	}

	return c.JSON(http.StatusOK, models.ConfigResponse{
		Server: models.ServerConfigResponse{
			Host: cfg.Server.Host,
			Port: cfg.Server.Port,
		},
		Security: models.SecurityConfigResponse{
			AllowedIPs:          cfg.Security.AllowedIPs,
			WebsocketTimeoutMin: cfg.Security.WebsocketTimeoutMin,
		},
		Multipass: models.MultipassConfigResponse{
			SocketPath:        cfg.Multipass.SocketPath,
			DefaultTimeoutSec: cfg.Multipass.DefaultTimeoutSec,
			SSHKeyPath:        cfg.Multipass.SSHKeyPath,
		},
		Logging: models.LoggingConfigResponse{
			Level:  cfg.Logging.Level,
			Format: cfg.Logging.Format,
			Output: cfg.Logging.Output,
		},
	})
}

func (h *ConfigHandler) Update(c echo.Context) error {
	var req models.ConfigUpdateRequest
	if err := c.Bind(&req); err != nil {
		logger.API.Warn().Err(err).Msg("invalid config request body")
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "invalid_request",
			Message: "invalid request body",
		})
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	cfg, err := config.Load(h.configPath)
	if err != nil {
		logger.API.Error().Err(err).Msg("failed to load config for update")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "internal_error",
			Message: "failed to load configuration",
		})
	}

	modified := false

	if req.Server != nil {
		if req.Server.Host != "" {
			cfg.Server.Host = req.Server.Host
			modified = true
		}
		if req.Server.Port > 0 {
			cfg.Server.Port = req.Server.Port
			modified = true
		}
	}

	if req.Security != nil {
		if req.Security.AllowedIPs != nil {
			cfg.Security.AllowedIPs = req.Security.AllowedIPs
			modified = true
		}
		if req.Security.WebsocketTimeoutMin > 0 {
			cfg.Security.WebsocketTimeoutMin = req.Security.WebsocketTimeoutMin
			modified = true
		}
	}

	if req.Multipass != nil {
		if req.Multipass.SocketPath != "" {
			cfg.Multipass.SocketPath = req.Multipass.SocketPath
			modified = true
		}
		if req.Multipass.DefaultTimeoutSec > 0 {
			cfg.Multipass.DefaultTimeoutSec = req.Multipass.DefaultTimeoutSec
			modified = true
		}
		if req.Multipass.SSHKeyPath != "" {
			cfg.Multipass.SSHKeyPath = req.Multipass.SSHKeyPath
			modified = true
		}
	}

	if req.Logging != nil {
		if req.Logging.Level != "" {
			cfg.Logging.Level = req.Logging.Level
			modified = true
		}
		if req.Logging.Format != "" {
			cfg.Logging.Format = req.Logging.Format
			modified = true
		}
		if req.Logging.Output != "" {
			cfg.Logging.Output = req.Logging.Output
			modified = true
		}
	}

	if !modified {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "invalid_request",
			Message: "no valid changes provided",
		})
	}

	if err := config.Save(cfg, h.configPath); err != nil {
		logger.API.Error().Err(err).Msg("failed to save config")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "internal_error",
			Message: "failed to save configuration",
		})
	}

	logger.API.Info().Msg("configuration updated")
	return c.JSON(http.StatusOK, models.InstanceResponse{
		Message: "Configuration updated. Restart required for changes to take effect.",
	})
}
