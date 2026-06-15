package handlers

import (
	"net"
	"net/http"
	"strings"

	"cloudpass/internal/config"
	"cloudpass/internal/logger"
	"cloudpass/internal/models"

	"github.com/labstack/echo/v4"
)

type ConfigHandler struct {
	cfgManager *config.ConfigManager
}

func NewConfigHandler(cfgManager *config.ConfigManager) *ConfigHandler {
	return &ConfigHandler{cfgManager: cfgManager}
}

func validateCIDR(cidr string) error {
	_, _, err := net.ParseCIDR(cidr)
	return err
}

func validatePort(port int) error {
	if port < 1 || port > 65535 {
		return http.ErrNoCookie
	}
	return nil
}

func (h *ConfigHandler) Get(c echo.Context) error {
	cfg := h.cfgManager.Get()

	return c.JSON(http.StatusOK, models.ConfigResponse{
		Server: models.ServerConfigResponse{
			Host: cfg.Server.Host,
			Port: cfg.Server.Port,
		},
		Security: models.SecurityConfigResponse{
			WebsocketTimeoutMin: cfg.Security.WebsocketTimeoutMin,
		},
		Multipass: models.MultipassConfigResponse{
			SocketPath:        cfg.Multipass.SocketPath,
			DefaultTimeoutSec: cfg.Multipass.DefaultTimeoutSec,
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
		logger.API.Load().Warn().Err(err).Msg("invalid config request body")
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "invalid_request",
			Message: "invalid request body",
		})
	}

	cfg := h.cfgManager.Get()

	modified := false
	needsRestart := false

	if req.Server != nil {
		if req.Server.Host != "" {
			cfg.Server.Host = req.Server.Host
			modified = true
			needsRestart = true
		}
		if req.Server.Port > 0 {
			if err := validatePort(req.Server.Port); err != nil {
				return c.JSON(http.StatusBadRequest, models.ErrorResponse{
					Error:   "invalid_request",
					Message: "port must be between 1 and 65535",
				})
			}
			cfg.Server.Port = req.Server.Port
			modified = true
			needsRestart = true
		}
	}

	if req.Security != nil {
		if req.Security.AllowedIPs != nil {
			for _, ip := range req.Security.AllowedIPs {
				if err := validateCIDR(ip); err != nil {
					return c.JSON(http.StatusBadRequest, models.ErrorResponse{
						Error:   "invalid_request",
						Message: "invalid CIDR format: " + ip,
					})
				}
			}
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
		validLevels := map[string]bool{"trace": true, "debug": true, "info": true, "warn": true, "error": true, "fatal": true, "panic": true}
		validFormats := map[string]bool{"console": true, "json": true}
		validOutputs := map[string]bool{"stdout": true, "stderr": true, "file": true, "syslog": true}

		if req.Logging.Level != "" {
			if !validLevels[req.Logging.Level] {
				return c.JSON(http.StatusBadRequest, models.ErrorResponse{
					Error:   "invalid_request",
					Message: "invalid log level: " + req.Logging.Level,
				})
			}
			cfg.Logging.Level = req.Logging.Level
			modified = true
		}
		if req.Logging.Format != "" {
			if !validFormats[req.Logging.Format] {
				return c.JSON(http.StatusBadRequest, models.ErrorResponse{
					Error:   "invalid_request",
					Message: "invalid log format: " + req.Logging.Format,
				})
			}
			cfg.Logging.Format = req.Logging.Format
			modified = true
		}
		if req.Logging.Output != "" {
			if !validOutputs[req.Logging.Output] {
				return c.JSON(http.StatusBadRequest, models.ErrorResponse{
					Error:   "invalid_request",
					Message: "invalid log output: " + req.Logging.Output,
				})
			}
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

	if err := h.cfgManager.Update(cfg); err != nil {
		logger.API.Load().Error().Err(err).Msg("failed to save config")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "internal_error",
			Message: "failed to save configuration",
		})
	}

	var msg strings.Builder
	msg.WriteString("Configuration updated.")

	if req.Logging.Level != "" || req.Logging.Format != "" || req.Logging.Output != "" {
		if err := logger.Reinit(cfg.Logging); err != nil {
			logger.API.Load().Warn().Err(err).Msg("failed to reinit logger, changes will apply on restart")
			msg.WriteString(" Logging changes will apply on restart.")
		} else {
			msg.WriteString(" Logging changes applied immediately.")
		}
	}

	if needsRestart {
		logger.API.Load().Info().Msg("configuration updated (restart required for server settings)")
		msg.WriteString(" Server restart required for host/port changes to take effect.")
	} else {
		logger.API.Load().Info().Msg("configuration updated successfully")
	}

	return c.JSON(http.StatusOK, models.InstanceResponse{
		Message: strings.TrimSpace(msg.String()),
	})
}
