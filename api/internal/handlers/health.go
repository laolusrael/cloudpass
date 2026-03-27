package handlers

import (
	"net/http"
	"time"

	"cloudpass/internal/models"

	"github.com/labstack/echo/v4"
)

type HealthHandler struct{}

func NewHealthHandler() *HealthHandler {
	return &HealthHandler{}
}

func (h *HealthHandler) Health(c echo.Context) error {
	return c.JSON(http.StatusOK, models.HealthResponse{
		Status:    "healthy",
		Timestamp: time.Now().UTC(),
	})
}
