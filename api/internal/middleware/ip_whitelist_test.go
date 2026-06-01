package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"cloudpass/internal/config"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

func TestIPWhitelist_EmptyAllowedList(t *testing.T) {
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
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler(c)
	if err != nil {
		httpError := err.(*echo.HTTPError)
		rec.Code = httpError.Code
	}
	assert.Equal(t, http.StatusForbidden, rec.Code)
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
	assert.Equal(t, http.StatusForbidden, rec.Code)

	cfg.Security.AllowedIPs = []string{"10.0.0.0/8", "192.168.1.0/24"}

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
