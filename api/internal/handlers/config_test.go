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

func TestConfigUpdate_ProxySettings(t *testing.T) {
	e := echo.New()
	mgr := testConfigManager(t, &config.Config{
		Security: config.SecurityConfig{EnableProxyHeader: true},
	})
	handler := NewConfigHandler(mgr)

	body := `{"security":{"enable_proxy_header":false,"trusted_proxies":["10.0.0.5","192.168.0.0/16"]}}`
	req := httptest.NewRequest(http.MethodPost, "/api/config", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	require.NoError(t, handler.Update(c))
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.False(t, mgr.Get().Security.EnableProxyHeader)
	assert.Equal(t, []string{"10.0.0.5", "192.168.0.0/16"}, mgr.GetTrustedProxies())
}

func TestConfigUpdate_RejectsInvalidTrustedProxy(t *testing.T) {
	e := echo.New()
	mgr := testConfigManager(t, &config.Config{})
	handler := NewConfigHandler(mgr)

	body := `{"security":{"trusted_proxies":["bogus"]}}`
	req := httptest.NewRequest(http.MethodPost, "/api/config", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	require.NoError(t, handler.Update(c))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "invalid IP or CIDR")
}

func TestConfigGet_ReturnsProxySettings(t *testing.T) {
	e := echo.New()
	mgr := testConfigManager(t, &config.Config{
		Security: config.SecurityConfig{
			EnableProxyHeader: false,
			TrustedProxies:    []string{"10.0.0.5"},
		},
	})
	handler := NewConfigHandler(mgr)

	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	require.NoError(t, handler.Get(c))
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"enable_proxy_header":false`)
	assert.Contains(t, rec.Body.String(), "10.0.0.5")
}

func TestConfigGet_ReturnsUploadSettings(t *testing.T) {
	e := echo.New()
	mgr := testConfigManager(t, &config.Config{
		Upload: config.UploadConfig{
			MaxFileSizeMB: 100,
			DefaultPath:   "/home/ubuntu/uploads",
		},
		Multipass: config.MultipassConfig{
			SSHKeyPath: "/home/user/.cloudpass/multipass_id_rsa",
		},
	})
	handler := NewConfigHandler(mgr)

	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	require.NoError(t, handler.Get(c))
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"max_file_size_mb":100`)
	assert.Contains(t, rec.Body.String(), "/home/ubuntu/uploads")
	assert.Contains(t, rec.Body.String(), "multipass_id_rsa")
}

func TestConfigUpdate_UploadSettings(t *testing.T) {
	e := echo.New()
	mgr := testConfigManager(t, &config.Config{})
	handler := NewConfigHandler(mgr)

	body := `{"upload":{"max_file_size_mb":50,"default_path":"/custom/uploads"}}`
	req := httptest.NewRequest(http.MethodPost, "/api/config", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	require.NoError(t, handler.Update(c))
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, 50, mgr.GetUploadConfig().MaxFileSizeMB)
	assert.Equal(t, "/custom/uploads", mgr.GetUploadConfig().DefaultPath)
}

func TestConfigUpdate_RejectsInvalidUploadSettings(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{"zero size", `{"upload":{"max_file_size_mb":0}}`, "max_file_size_mb must be between 1 and 1024"},
		{"negative size", `{"upload":{"max_file_size_mb":-5}}`, "max_file_size_mb must be between 1 and 1024"},
		{"oversize", `{"upload":{"max_file_size_mb":2048}}`, "max_file_size_mb must be between 1 and 1024"},
		{"relative path", `{"upload":{"default_path":"relative/uploads"}}`, "must be an absolute path"},
		{"traversal path", `{"upload":{"default_path":"/home/../etc"}}`, "must not contain '..'"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := echo.New()
			mgr := testConfigManager(t, &config.Config{})
			handler := NewConfigHandler(mgr)

			req := httptest.NewRequest(http.MethodPost, "/api/config", strings.NewReader(tt.body))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			require.NoError(t, handler.Update(c))
			assert.Equal(t, http.StatusBadRequest, rec.Code)
			assert.Contains(t, rec.Body.String(), tt.want)
		})
	}
}

func TestConfigUpdate_SSHKeyPath(t *testing.T) {
	e := echo.New()
	mgr := testConfigManager(t, &config.Config{})
	handler := NewConfigHandler(mgr)

	body := `{"multipass":{"ssh_key_path":"/home/user/.cloudpass/multipass_id_rsa"}}`
	req := httptest.NewRequest(http.MethodPost, "/api/config", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	require.NoError(t, handler.Update(c))
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "/home/user/.cloudpass/multipass_id_rsa", mgr.GetSSHKeyPath())
}

func TestConfigClientIP_HonorsProxyHeadersWhenEnabled(t *testing.T) {
	e := echo.New()
	mgr := testConfigManager(t, &config.Config{
		Security: config.SecurityConfig{
			EnableProxyHeader: true,
			TrustedProxies:    []string{"10.0.0.0/8"},
		},
	})
	handler := NewConfigHandler(mgr)

	req := httptest.NewRequest(http.MethodGet, "/api/config/client-ip", nil)
	req.RemoteAddr = "10.0.0.5:12345"
	req.Header.Set("X-Forwarded-For", "192.168.1.100, 10.0.0.1")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	require.NoError(t, handler.ClientIP(c))
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"ip":"192.168.1.100"`)
}

func TestConfigClientIP_IgnoresProxyHeadersWhenDisabled(t *testing.T) {
	e := echo.New()
	mgr := testConfigManager(t, &config.Config{
		Security: config.SecurityConfig{EnableProxyHeader: false},
	})
	handler := NewConfigHandler(mgr)

	req := httptest.NewRequest(http.MethodGet, "/api/config/client-ip", nil)
	req.RemoteAddr = "10.0.0.5:12345"
	req.Header.Set("X-Forwarded-For", "192.168.1.100")
	req.Header.Set("X-Real-IP", "192.168.1.101")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	require.NoError(t, handler.ClientIP(c))
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"ip":"10.0.0.5"`)
}

func TestConfigClientIP_IgnoresHeadersFromUntrustedPeer(t *testing.T) {
	e := echo.New()
	mgr := testConfigManager(t, &config.Config{
		Security: config.SecurityConfig{
			EnableProxyHeader: true,
			TrustedProxies:    []string{"10.0.0.5"},
		},
	})
	handler := NewConfigHandler(mgr)

	req := httptest.NewRequest(http.MethodGet, "/api/config/client-ip", nil)
	req.RemoteAddr = "203.0.113.7:12345"
	req.Header.Set("X-Forwarded-For", "192.168.1.100")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	require.NoError(t, handler.ClientIP(c))
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"ip":"203.0.113.7"`)
}
