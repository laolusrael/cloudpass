package handlers

import (
	"net/http"

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
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "multipass_error",
			Message: err.Error(),
		})
	}

	return c.JSON(http.StatusOK, models.ImageList{
		Images: images,
	})
}
