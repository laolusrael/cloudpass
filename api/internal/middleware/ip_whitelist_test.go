package middleware

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"cloudpass/internal/config"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

func TestIPWhitelist_EmptyAllowedListFallsBackToLocalNetworks(t *testing.T) {
	// Empty allowlist must not lock out local clients: the middleware falls
	// back to auto-detected local networks (loopback always included).
	e := echo.New()
	e.HTTPErrorHandler = func(err error, c echo.Context) {
		if he, ok := err.(*echo.HTTPError); ok {
			_ = c.JSON(he.Code, he.Message)
		}
	}

	cfg := &config.Config{}
	cfgManager := config.NewConfigManager(cfg, "")

	handler := IPWhitelist(cfgManager)(func(c echo.Context) error {
		return c.String(http.StatusOK, "allowed")
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler(c)
	if err != nil {
		httpError := err.(*echo.HTTPError)
		rec.Code = httpError.Code
	}
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestIPWhitelist_AllowedCIDR(t *testing.T) {
	e := echo.New()
	e.HTTPErrorHandler = func(err error, c echo.Context) {
		if he, ok := err.(*echo.HTTPError); ok {
			_ = c.JSON(he.Code, he.Message)
		}
	}

	cfg := &config.Config{
		Security: config.SecurityConfig{
			AllowedIPs: []string{"192.168.1.0/24"},
		},
	}
	cfgManager := config.NewConfigManager(cfg, "")

	handler := IPWhitelist(cfgManager)(func(c echo.Context) error {
		return c.String(http.StatusOK, "allowed")
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "192.168.1.100:12345"
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler(c)
	if err != nil {
		httpError := err.(*echo.HTTPError)
		rec.Code = httpError.Code
	}
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestIPWhitelist_BlockedIP(t *testing.T) {
	e := echo.New()
	e.HTTPErrorHandler = func(err error, c echo.Context) {
		if he, ok := err.(*echo.HTTPError); ok {
			_ = c.JSON(he.Code, he.Message)
		}
	}

	cfg := &config.Config{
		Security: config.SecurityConfig{
			AllowedIPs: []string{"10.0.0.0/8"},
		},
	}
	cfgManager := config.NewConfigManager(cfg, "")

	handler := IPWhitelist(cfgManager)(func(c echo.Context) error {
		return c.String(http.StatusOK, "allowed")
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler(c)
	if err != nil {
		httpError := err.(*echo.HTTPError)
		rec.Code = httpError.Code
	}
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestIPWhitelist_MultipleCIDRs_Allowed(t *testing.T) {
	e := echo.New()
	e.HTTPErrorHandler = func(err error, c echo.Context) {
		if he, ok := err.(*echo.HTTPError); ok {
			_ = c.JSON(he.Code, he.Message)
		}
	}

	cfg := &config.Config{
		Security: config.SecurityConfig{
			AllowedIPs: []string{"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"},
		},
	}
	cfgManager := config.NewConfigManager(cfg, "")

	handler := IPWhitelist(cfgManager)(func(c echo.Context) error {
		return c.String(http.StatusOK, "allowed")
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:8080"
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler(c)
	if err != nil {
		httpError := err.(*echo.HTTPError)
		rec.Code = httpError.Code
	}
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestIPWhitelist_MultipleCIDRs_Blocked(t *testing.T) {
	e := echo.New()
	e.HTTPErrorHandler = func(err error, c echo.Context) {
		if he, ok := err.(*echo.HTTPError); ok {
			_ = c.JSON(he.Code, he.Message)
		}
	}

	cfg := &config.Config{
		Security: config.SecurityConfig{
			AllowedIPs: []string{"10.0.0.0/8"},
		},
	}
	cfgManager := config.NewConfigManager(cfg, "")

	handler := IPWhitelist(cfgManager)(func(c echo.Context) error {
		return c.String(http.StatusOK, "allowed")
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "1.2.3.4:8080"
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler(c)
	if err != nil {
		httpError := err.(*echo.HTTPError)
		rec.Code = httpError.Code
	}
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestIPWhitelist_EmptyRemoteAddr(t *testing.T) {
	e := echo.New()
	e.HTTPErrorHandler = func(err error, c echo.Context) {
		if he, ok := err.(*echo.HTTPError); ok {
			_ = c.JSON(he.Code, he.Message)
		}
	}

	cfg := &config.Config{
		Security: config.SecurityConfig{
			AllowedIPs: []string{"192.168.1.0/24"},
		},
	}
	cfgManager := config.NewConfigManager(cfg, "")

	handler := IPWhitelist(cfgManager)(func(c echo.Context) error {
		return c.String(http.StatusOK, "allowed")
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = ""
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler(c)
	if err != nil {
		httpError := err.(*echo.HTTPError)
		rec.Code = httpError.Code
	}
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestIPWhitelist_HotReload(t *testing.T) {
	e := echo.New()
	e.HTTPErrorHandler = func(err error, c echo.Context) {
		if he, ok := err.(*echo.HTTPError); ok {
			_ = c.JSON(he.Code, he.Message)
		}
	}

	cfg := &config.Config{
		Security: config.SecurityConfig{
			AllowedIPs: []string{"192.168.1.0/24"},
		},
	}

	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.yaml")
	cfgManager := config.NewConfigManager(cfg, cfgPath)

	handler := IPWhitelist(cfgManager)(func(c echo.Context) error {
		return c.String(http.StatusOK, "allowed")
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:8080"
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler(c)
	if err != nil {
		httpError := err.(*echo.HTTPError)
		rec.Code = httpError.Code
	}
	assert.Equal(t, http.StatusForbidden, rec.Code)

	cfg.Security.AllowedIPs = []string{"10.0.0.0/8", "192.168.1.0/24"}
	if err := cfgManager.Update(cfg); err != nil {
		t.Fatalf("failed to update config: %v", err)
	}

	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.RemoteAddr = "10.0.0.1:8080"
	rec2 := httptest.NewRecorder()
	c2 := e.NewContext(req2, rec2)

	err2 := handler(c2)
	if err2 != nil {
		httpError := err2.(*echo.HTTPError)
		rec2.Code = httpError.Code
	}
	assert.Equal(t, http.StatusOK, rec2.Code)
}

func TestIPWhitelist_InvalidCIDR(t *testing.T) {
	e := echo.New()
	e.HTTPErrorHandler = func(err error, c echo.Context) {
		if he, ok := err.(*echo.HTTPError); ok {
			_ = c.JSON(he.Code, he.Message)
		}
	}

	cfg := &config.Config{
		Security: config.SecurityConfig{
			AllowedIPs: []string{"invalid-cidr", "192.168.1.0/24"},
		},
	}
	cfgManager := config.NewConfigManager(cfg, "")

	handler := IPWhitelist(cfgManager)(func(c echo.Context) error {
		return c.String(http.StatusOK, "allowed")
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "192.168.1.100:12345"
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler(c)
	if err != nil {
		httpError := err.(*echo.HTTPError)
		rec.Code = httpError.Code
	}
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestIPWhitelist_SingleIPAllowed(t *testing.T) {
	e := echo.New()

	cfg := &config.Config{
		Security: config.SecurityConfig{
			AllowedIPs: []string{"192.168.1.100"},
		},
	}
	cfgManager := config.NewConfigManager(cfg, "")

	handler := IPWhitelist(cfgManager)(func(c echo.Context) error {
		return c.String(http.StatusOK, "allowed")
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "192.168.1.100:12345"
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	assert.NoError(t, handler(c))
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestIPWhitelist_SingleIPNeighborBlocked(t *testing.T) {
	e := echo.New()

	cfg := &config.Config{
		Security: config.SecurityConfig{
			AllowedIPs: []string{"192.168.1.100"},
		},
	}
	cfgManager := config.NewConfigManager(cfg, "")

	handler := IPWhitelist(cfgManager)(func(c echo.Context) error {
		return c.String(http.StatusOK, "allowed")
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "192.168.1.101:12345"
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	assert.NoError(t, handler(c))
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestIPWhitelist_SingleIPv6Allowed(t *testing.T) {
	e := echo.New()

	cfg := &config.Config{
		Security: config.SecurityConfig{
			AllowedIPs: []string{"::1"},
		},
	}
	cfgManager := config.NewConfigManager(cfg, "")

	handler := IPWhitelist(cfgManager)(func(c echo.Context) error {
		return c.String(http.StatusOK, "allowed")
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "[::1]:12345"
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	assert.NoError(t, handler(c))
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestIPWhitelist_MixedCIDRAndSingleIP(t *testing.T) {
	e := echo.New()

	cfg := &config.Config{
		Security: config.SecurityConfig{
			AllowedIPs: []string{"10.0.0.0/8", "192.168.1.100"},
		},
	}
	cfgManager := config.NewConfigManager(cfg, "")

	handler := IPWhitelist(cfgManager)(func(c echo.Context) error {
		return c.String(http.StatusOK, "allowed")
	})

	for _, tc := range []struct {
		remote string
		want   int
	}{
		{"10.5.6.7:1234", http.StatusOK},
		{"192.168.1.100:1234", http.StatusOK},
		{"192.168.1.101:1234", http.StatusForbidden},
	} {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = tc.remote
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		assert.NoError(t, handler(c))
		assert.Equal(t, tc.want, rec.Code, "remote %s", tc.remote)
	}
}

func TestIPWhitelist_AllInvalidEntriesDenied(t *testing.T) {
	e := echo.New()

	cfg := &config.Config{
		Security: config.SecurityConfig{
			AllowedIPs: []string{"not-an-ip", "999.999.0.0/16"},
		},
	}
	cfgManager := config.NewConfigManager(cfg, "")

	handler := IPWhitelist(cfgManager)(func(c echo.Context) error {
		return c.String(http.StatusOK, "allowed")
	})

	// Explicitly configured but entirely invalid: fail closed, even for
	// loopback. A typo must never silently widen access.
	for _, remote := range []string{"127.0.0.1:12345", "192.168.1.100:12345"} {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = remote
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		assert.NoError(t, handler(c))
		assert.Equal(t, http.StatusForbidden, rec.Code, "remote %s", remote)
	}
}

func TestIPWhitelist_IPv6ZoneIDClient(t *testing.T) {
	e := echo.New()

	cfg := &config.Config{
		Security: config.SecurityConfig{
			AllowedIPs: []string{"fe80::/10"},
		},
	}
	cfgManager := config.NewConfigManager(cfg, "")

	handler := IPWhitelist(cfgManager)(func(c echo.Context) error {
		return c.String(http.StatusOK, "allowed")
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "[fe80::1%eth0]:12345"
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	assert.NoError(t, handler(c))
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestIPWhitelist_HotReloadSingleIP(t *testing.T) {
	e := echo.New()

	cfg := &config.Config{
		Security: config.SecurityConfig{
			AllowedIPs: []string{"192.168.1.0/24"},
		},
	}

	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.yaml")
	cfgManager := config.NewConfigManager(cfg, cfgPath)

	handler := IPWhitelist(cfgManager)(func(c echo.Context) error {
		return c.String(http.StatusOK, "allowed")
	})

	allowed := func(remote string) int {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = remote
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		if err := handler(c); err != nil {
			if httpError, ok := err.(*echo.HTTPError); ok {
				rec.Code = httpError.Code
			}
		}
		return rec.Code
	}

	assert.Equal(t, http.StatusOK, allowed("192.168.1.100:12345"))
	assert.Equal(t, http.StatusForbidden, allowed("10.9.9.9:12345"))

	cfg.Security.AllowedIPs = []string{"10.9.9.9"}
	if err := cfgManager.Update(cfg); err != nil {
		t.Fatalf("failed to update config: %v", err)
	}

	assert.Equal(t, http.StatusForbidden, allowed("192.168.1.100:12345"))
	assert.Equal(t, http.StatusOK, allowed("10.9.9.9:12345"))
	assert.Equal(t, http.StatusForbidden, allowed("10.9.9.10:12345"))
}
