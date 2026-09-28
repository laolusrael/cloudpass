package handlers

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"cloudpass/internal/config"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testConfigManager(t *testing.T, cfg *config.Config) *config.ConfigManager {
	t.Helper()
	return config.NewConfigManager(cfg, filepath.Join(t.TempDir(), "config.yaml"))
}

func TestConfigUpdate_AcceptsSingleIP(t *testing.T) {
	e := echo.New()
	mgr := testConfigManager(t, &config.Config{})
	handler := NewConfigHandler(mgr)

	body := `{"security":{"allowed_ips":["192.168.1.100","10.0.0.0/8"]}}`
	req := httptest.NewRequest(http.MethodPost, "/api/config", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	require.NoError(t, handler.Update(c))
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, []string{"192.168.1.100", "10.0.0.0/8"}, mgr.GetAllowedIPs())
}

func TestConfigUpdate_RejectsInvalidIP(t *testing.T) {
	e := echo.New()
	mgr := testConfigManager(t, &config.Config{})
	handler := NewConfigHandler(mgr)

	body := `{"security":{"allowed_ips":["not-an-ip"]}}`
	req := httptest.NewRequest(http.MethodPost, "/api/config", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	require.NoError(t, handler.Update(c))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "invalid IP or CIDR")
}

func TestConfigGet_ReturnsAllowedIPs(t *testing.T) {
	e := echo.New()
	mgr := testConfigManager(t, &config.Config{
		Security: config.SecurityConfig{
			AllowedIPs:          []string{"192.168.1.100", "10.0.0.0/8"},
			WebsocketTimeoutMin: 30,
		},
	})
	handler := NewConfigHandler(mgr)

	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	require.NoError(t, handler.Get(c))
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "192.168.1.100")
	assert.Contains(t, rec.Body.String(), "10.0.0.0/8")
}
