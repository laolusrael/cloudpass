package middleware

import (
	"net/http"
	"sync"
	"time"

	"cloudpass/internal/models"

	"github.com/labstack/echo/v4"
)

type rateLimiter struct {
	mu       sync.RWMutex
	requests map[string][]time.Time
	limit    int
	window   time.Duration
}

func newRateLimiter(limit int, window time.Duration) *rateLimiter {
	return &rateLimiter{
		requests: make(map[string][]time.Time),
		limit:    limit,
		window:   window,
	}
}

func (rl *rateLimiter) allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-rl.window)

	// Filter out old requests
	valid := make([]time.Time, 0, len(rl.requests[ip]))
	for _, t := range rl.requests[ip] {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}

	if len(valid) >= rl.limit {
		rl.requests[ip] = valid
		return false
	}

	rl.requests[ip] = append(valid, now)
	return true
}

// RateLimit returns a middleware that limits requests per IP.
// Default: 120 requests per minute.
func RateLimit() echo.MiddlewareFunc {
	rl := newRateLimiter(120, time.Minute)

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			ip := c.RealIP()
			if !rl.allow(ip) {
				return c.JSON(http.StatusTooManyRequests, models.ErrorResponse{
					Error:   "rate_limited",
					Message: "Too many requests. Please try again later.",
				})
			}
			return next(c)
		}
	}
}
