package handlers

import (
	"net/http"

	"cloudpass/internal/logger"
	"cloudpass/internal/models"
	"cloudpass/internal/multipass"

	"github.com/labstack/echo/v4"
)

type ImageHandler struct {
	client multipass.Client
}

func NewImageHandler(client multipass.Client) *ImageHandler {
	return &ImageHandler{client: client}
}

func (h *ImageHandler) List(c echo.Context) error {
	images, err := h.client.ListImages()
	if err != nil {
		logger.API.Error().Err(err).Msg("failed to list images")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "multipass_error",
			Message: err.Error(),
		})
	}

	logger.API.Debug().Int("count", len(images)).Msg("listed images")
	return c.JSON(http.StatusOK, models.ImageList{
		Images: images,
	})
}
