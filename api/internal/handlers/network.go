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
