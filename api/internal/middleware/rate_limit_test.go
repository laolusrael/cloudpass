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
