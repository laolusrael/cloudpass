package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"cloudpass/internal/models"
	"cloudpass/internal/multipass"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

func setupImageRouter(client multipass.Client) *echo.Echo {
	e := echo.New()
	handler := NewImageHandler(client)
	e.GET("/images", handler.List)
	return e
}

func TestImageHandler_List_Success(t *testing.T) {
	mockClient := multipass.NewMockClient()
	mockClient.SetImages([]models.Image{
		{Alias: "22.04", Version: "22.04.3", Release: "Ubuntu 22.04 LTS", Remote: "ubuntu"},
	})

	e := setupImageRouter(mockClient)

	req := httptest.NewRequest(http.MethodGet, "/images", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "22.04")
}

func TestImageHandler_List_Error(t *testing.T) {
	mockClient := multipass.NewMockClient()
	mockClient.SetListImagesErr(assert.AnError)

	e := setupImageRouter(mockClient)

	req := httptest.NewRequest(http.MethodGet, "/images", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestImageHandler_List_Empty(t *testing.T) {
	mockClient := multipass.NewMockClient()
	mockClient.SetImages([]models.Image{})

	e := setupImageRouter(mockClient)

	req := httptest.NewRequest(http.MethodGet, "/images", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "[]")
}

func setupHealthRouter() *echo.Echo {
	e := echo.New()
	handler := NewHealthHandler()
	e.GET("/health", handler.Health)
	return e
}

func TestHealthHandler_Health(t *testing.T) {
	e := setupHealthRouter()

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "healthy")
}
