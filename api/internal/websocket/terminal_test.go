package websocket

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"cloudpass/internal/models"
	"cloudpass/internal/multipass"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

func setupTerminalRouter() *echo.Echo {
	e := echo.New()
	return e
}

func TestHandleTerminal_EmptyName(t *testing.T) {
	mockClient := multipass.NewMockClient()
	handler := NewTerminalHandler(mockClient)

	e := setupTerminalRouter()
	e.GET("/instances/:name/terminal", handler.HandleTerminal)

	req := httptest.NewRequest(http.MethodGet, "/instances//terminal", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "instance name is required")
}

func TestHandleTerminal_NonExistent(t *testing.T) {
	mockClient := multipass.NewMockClient()
	mockClient.SetGetInstanceErr(assert.AnError)

	handler := NewTerminalHandler(mockClient)

	e := setupTerminalRouter()
	e.GET("/instances/:name/terminal", handler.HandleTerminal)

	req := httptest.NewRequest(http.MethodGet, "/instances/nonexistent/terminal", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestHandleTerminal_StoppedInstance(t *testing.T) {
	mockClient := multipass.NewMockClient()
	mockClient.SetInstances([]models.Instance{{Name: "stopped", State: "Stopped", IPv4: []string{"192.168.1.100"}}})

	handler := NewTerminalHandler(mockClient)

	e := setupTerminalRouter()
	e.GET("/instances/:name/terminal", handler.HandleTerminal)

	req := httptest.NewRequest(http.MethodGet, "/instances/stopped/terminal", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "not running")
}

func TestHandleTerminal_NoIP(t *testing.T) {
	mockClient := multipass.NewMockClient()
	mockClient.SetInstances([]models.Instance{{Name: "noip", State: "Running", IPv4: []string{}}})

	handler := NewTerminalHandler(mockClient)

	e := setupTerminalRouter()
	e.GET("/instances/:name/terminal", handler.HandleTerminal)

	req := httptest.NewRequest(http.MethodGet, "/instances/noip/terminal", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "no IP address")
}
