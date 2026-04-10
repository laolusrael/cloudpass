package handlers

import (
	"cloudpass/internal/config"
	"cloudpass/internal/logger"
	"cloudpass/internal/models"
	"cloudpass/internal/multipass"
	"net/http"

	"github.com/labstack/echo/v4"
)

type HostHandler struct {
	client multipass.Client
	cfg    *config.Config
}

func NewHostHandler(client multipass.Client, cfg *config.Config) *HostHandler {
	return &HostHandler{client: client, cfg: cfg}
}

func (h *HostHandler) GetInfo(c echo.Context) error {
	hostInfo, err := h.client.GetHostInfo()
	if err != nil {
		logger.API.Error().Err(err).Msg("failed to get host info")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "host_error",
			Message: "failed to get host information",
		})
	}

	logger.API.Debug().Msg("host info retrieved")
	return c.JSON(http.StatusOK, hostInfo)
}
