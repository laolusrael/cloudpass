package websocket

import (
	"encoding/json"
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
	handler := NewTerminalHandler(mockClient, "")

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

	handler := NewTerminalHandler(mockClient, "")

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

	handler := NewTerminalHandler(mockClient, "")

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

	handler := NewTerminalHandler(mockClient, "")

	e := setupTerminalRouter()
	e.GET("/instances/:name/terminal", handler.HandleTerminal)

	req := httptest.NewRequest(http.MethodGet, "/instances/noip/terminal", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "no IP address")
}

func TestTerminalMessage_JSONSerialization(t *testing.T) {
	msg := TerminalMessage{
		Type: "resize",
		Cols: 80,
		Rows: 24,
	}

	data, err := json.Marshal(msg)
	assert.NoError(t, err)

	var decoded TerminalMessage
	err = json.Unmarshal(data, &decoded)
	assert.NoError(t, err)
	assert.Equal(t, "resize", decoded.Type)
	assert.Equal(t, 80, decoded.Cols)
	assert.Equal(t, 24, decoded.Rows)
}

func TestTerminalMessage_ErrorFormat(t *testing.T) {
	errorMsg := map[string]string{
		"type": "error",
		"data": "failed to connect via SSH: connection refused",
	}

	data, err := json.Marshal(errorMsg)
	assert.NoError(t, err)

	var decoded map[string]string
	err = json.Unmarshal(data, &decoded)
	assert.NoError(t, err)
	assert.Equal(t, "error", decoded["type"])
	assert.Contains(t, decoded["data"], "failed to connect via SSH")
}
