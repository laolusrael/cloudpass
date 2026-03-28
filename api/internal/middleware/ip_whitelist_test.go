package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

func TestIPWhitelist_EmptyAllowedList(t *testing.T) {
	e := echo.New()
	e.HTTPErrorHandler = func(err error, c echo.Context) {
		if he, ok := err.(*echo.HTTPError); ok {
			c.JSON(he.Code, he.Message)
		}
	}

	handler := IPWhitelist(nil)(func(c echo.Context) error {
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
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestIPWhitelist_AllowedCIDR(t *testing.T) {
	e := echo.New()
	e.HTTPErrorHandler = func(err error, c echo.Context) {
		if he, ok := err.(*echo.HTTPError); ok {
			c.JSON(he.Code, he.Message)
		}
	}

	handler := IPWhitelist([]string{"192.168.1.0/24"})(func(c echo.Context) error {
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
			c.JSON(he.Code, he.Message)
		}
	}

	handler := IPWhitelist([]string{"10.0.0.0/8"})(func(c echo.Context) error {
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
			c.JSON(he.Code, he.Message)
		}
	}

	handler := IPWhitelist([]string{"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"})(func(c echo.Context) error {
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
			c.JSON(he.Code, he.Message)
		}
	}

	handler := IPWhitelist([]string{"10.0.0.0/8"})(func(c echo.Context) error {
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
			c.JSON(he.Code, he.Message)
		}
	}

	handler := IPWhitelist([]string{"192.168.1.0/24"})(func(c echo.Context) error {
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
