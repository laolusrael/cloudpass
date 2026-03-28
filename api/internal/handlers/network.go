package handlers

import (
	"net/http"

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
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "multipass_error",
			Message: err.Error(),
		})
	}

	return c.JSON(http.StatusOK, models.NetworkList{
		Networks: networks,
	})
}

func (h *NetworkHandler) Create(c echo.Context) error {
	var req models.CreateNetworkRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "invalid_request",
			Message: "invalid request body",
		})
	}

	if req.Name == "" {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "invalid_request",
			Message: "network name is required",
		})
	}

	err := h.client.CreateNetwork(req.Name, req.Mode, req.MAC)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "multipass_error",
			Message: err.Error(),
		})
	}

	return c.JSON(http.StatusCreated, models.InstanceResponse{
		Message: "Network created",
	})
}

func (h *NetworkHandler) Delete(c echo.Context) error {
	name := c.Param("name")
	if name == "" {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "invalid_request",
			Message: "network name is required",
		})
	}

	err := h.client.DeleteNetwork(name)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "multipass_error",
			Message: err.Error(),
		})
	}

	return c.JSON(http.StatusOK, models.InstanceResponse{
		Message: "Network deleted",
	})
}
