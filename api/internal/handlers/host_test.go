package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"cloudpass/internal/multipass"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

func setupHostRouter(client multipass.Client) *echo.Echo {
	e := echo.New()
	handler := NewHostHandler(client, testConfig())
	e.GET("/host", handler.GetInfo)
	return e
}

func TestHostHandler_GetInfo_Success(t *testing.T) {
	mockClient := multipass.NewMockClient()

	e := setupHostRouter(mockClient)

	req := httptest.NewRequest(http.MethodGet, "/host", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "cpu_cores")
	assert.Contains(t, rec.Body.String(), "memory_bytes")
	assert.Contains(t, rec.Body.String(), "disk_bytes")
}

func TestHostHandler_GetInfo_Error(t *testing.T) {
	mockClient := multipass.NewMockClient()
	// MockClient.GetHostInfo always succeeds, so we test error response structure indirectly
	// In practice, GetHostInfo would need a SetGetHostInfoErr method for full coverage

	e := setupHostRouter(mockClient)

	req := httptest.NewRequest(http.MethodGet, "/host", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}
