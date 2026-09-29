package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

func TestRateLimit_AllowsRequestsUnderLimit(t *testing.T) {
	rl := newRateLimiter(5, time.Minute)

	assert.True(t, rl.allow("127.0.0.1"))
	assert.True(t, rl.allow("127.0.0.1"))
	assert.True(t, rl.allow("127.0.0.1"))
	assert.True(t, rl.allow("127.0.0.1"))
	assert.True(t, rl.allow("127.0.0.1"))
}

func TestRateLimit_BlocksRequestsOverLimit(t *testing.T) {
	rl := newRateLimiter(2, time.Minute)

	assert.True(t, rl.allow("127.0.0.1"))
	assert.True(t, rl.allow("127.0.0.1"))
	assert.False(t, rl.allow("127.0.0.1"))
}

func TestRateLimit_ResetsAfterWindow(t *testing.T) {
	rl := newRateLimiter(1, 50*time.Millisecond)

	assert.True(t, rl.allow("127.0.0.1"))
	assert.False(t, rl.allow("127.0.0.1"))

	time.Sleep(60 * time.Millisecond)
	assert.True(t, rl.allow("127.0.0.1"))
}

func TestRateLimit_MiddlewareBlocks(t *testing.T) {
	e := echo.New()
	middleware := RateLimit()

	handler := middleware(func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	// Should pass
	err := handler(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestRateLimit_SetsRetryAfterHeader(t *testing.T) {
	e := echo.New()
	middleware := RateLimit()

	handler := middleware(func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	var lastRec *httptest.ResponseRecorder
	for i := 0; i < defaultRateLimit+1; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		lastRec = httptest.NewRecorder()
		c := e.NewContext(req, lastRec)
		assert.NoError(t, handler(c))
	}

	assert.Equal(t, http.StatusTooManyRequests, lastRec.Code)
	assert.Equal(t, "60", lastRec.Header().Get("Retry-After"))
	assert.Contains(t, lastRec.Body.String(), "rate_limited")
}

func TestRateLimit_HealthExempt(t *testing.T) {
	e := echo.New()
	middleware := RateLimit()

	handler := middleware(func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	// Far beyond the limit: health probes must never be rejected.
	for i := 0; i < defaultRateLimit+10; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		assert.NoError(t, handler(c))
		assert.Equal(t, http.StatusOK, rec.Code)
	}

	// HEAD probes are exempt as well.
	req := httptest.NewRequest(http.MethodHead, "/api/health", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	assert.NoError(t, handler(c))
	assert.Equal(t, http.StatusOK, rec.Code)
}
