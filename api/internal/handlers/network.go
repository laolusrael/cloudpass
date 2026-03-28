package handlers

import (
	"net/http"

	"cloudpass/internal/logger"
	"cloudpass/internal/models"
	"cloudpass/internal/multipass"

	"github.com/labstack/echo/v4"
)

type NetworkHandler struct {
	client multipass.Client
}

func NewNetworkHandler(client multipass.Client) *NetworkHandler {
	return &NetworkHandler{client: client}
}

func (h *NetworkHandler) List(c echo.Context) error {
	networks, err := h.client.ListNetworks()
	if err != nil {
		logger.API.Error().Err(err).Msg("failed to list networks")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "multipass_error",
			Message: err.Error(),
		})
	}

	logger.API.Debug().Int("count", len(networks)).Msg("listed networks")
	return c.JSON(http.StatusOK, models.NetworkList{
		Networks: networks,
	})
}

func (h *NetworkHandler) Create(c echo.Context) error {
	var req models.CreateNetworkRequest
	if err := c.Bind(&req); err != nil {
		logger.API.Warn().Err(err).Str("ip", c.RealIP()).Msg("invalid request body")
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "invalid_request",
			Message: "invalid request body",
		})
	}

	if req.Name == "" {
		logger.API.Warn().Str("ip", c.RealIP()).Msg("network name is required")
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "invalid_request",
			Message: "network name is required",
		})
	}

	err := h.client.CreateNetwork(req.Name, req.Mode, req.MAC)
	if err != nil {
		logger.API.Error().Err(err).Str("ip", c.RealIP()).Str("name", req.Name).Msg("failed to create network")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "multipass_error",
			Message: err.Error(),
		})
	}

	logger.API.Info().Str("ip", c.RealIP()).Str("name", req.Name).Msg("network created")
	return c.JSON(http.StatusCreated, models.InstanceResponse{
		Message: "Network created",
	})
}

func (h *NetworkHandler) Delete(c echo.Context) error {
	name := c.Param("name")
	if name == "" {
		logger.API.Warn().Str("ip", c.RealIP()).Msg("network name is required")
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "invalid_request",
			Message: "network name is required",
		})
	}

	err := h.client.DeleteNetwork(name)
	if err != nil {
		logger.API.Error().Err(err).Str("ip", c.RealIP()).Str("name", name).Msg("failed to delete network")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "multipass_error",
			Message: err.Error(),
		})
	}

	logger.API.Info().Str("ip", c.RealIP()).Str("name", name).Msg("network deleted")
	return c.JSON(http.StatusOK, models.InstanceResponse{
		Message: "Network deleted",
	})
}
