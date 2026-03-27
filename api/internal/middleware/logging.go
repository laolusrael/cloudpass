package middleware

import (
	"time"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

func Logging() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()

			err := next(c)

			latency := time.Since(start)

			log.Info().
				Str("method", c.Request().Method).
				Str("path", c.Request().URL.Path).
				Str("ip", c.RealIP()).
				Int("status", c.Response().Status).
				Dur("latency", latency).
				Msg("request")

			return err
		}
	}
}
