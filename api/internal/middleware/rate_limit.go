package middleware

import (
	"net/http"
	"sync"
	"time"

	"cloudpass/internal/models"

	"github.com/labstack/echo/v4"
)

const (
	defaultRateLimit       = 120
	defaultRateLimitWindow = time.Minute
	defaultMaxEntries      = 10000
	defaultCleanupInterval = 5 * time.Minute
)

type rateLimiter struct {
	mu          sync.RWMutex
	requests    map[string][]time.Time
	limit       int
	window      time.Duration
	maxEntries  int
	stopCleanup chan struct{}
}

func newRateLimiter(limit int, window time.Duration) *rateLimiter {
	rl := &rateLimiter{
		requests:    make(map[string][]time.Time),
		limit:       limit,
		window:      window,
		maxEntries:  defaultMaxEntries,
		stopCleanup: make(chan struct{}),
	}
	go rl.cleanupLoop()
	return rl
}

func (rl *rateLimiter) cleanupLoop() {
	ticker := time.NewTicker(defaultCleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-rl.stopCleanup:
			return
		case <-ticker.C:
			rl.cleanup()
		}
	}
}

func (rl *rateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-rl.window)

	for ip, times := range rl.requests {
		valid := make([]time.Time, 0, len(times))
		for _, t := range times {
			if t.After(cutoff) {
				valid = append(valid, t)
			}
		}
		if len(valid) == 0 {
			delete(rl.requests, ip)
		} else {
			rl.requests[ip] = valid
		}
	}
}

func (rl *rateLimiter) Stop() {
	close(rl.stopCleanup)
}

func (rl *rateLimiter) allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-rl.window)

	if len(rl.requests) >= rl.maxEntries && rl.requests[ip] == nil {
		return false
	}

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

func RateLimit() echo.MiddlewareFunc {
	rl := newRateLimiter(defaultRateLimit, defaultRateLimitWindow)

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
