package middleware

import (
	"net"
	"net/http"

	"github.com/labstack/echo/v4"
)

func IPWhitelist(allowedCIDRs []string) echo.MiddlewareFunc {
	if len(allowedCIDRs) == 0 {
		return func(next echo.HandlerFunc) echo.HandlerFunc {
			return next
		}
	}

	cidrs := make([]*net.IPNet, 0, len(allowedCIDRs))
	for _, cidr := range allowedCIDRs {
		_, ipnet, err := net.ParseCIDR(cidr)
		if err != nil {
			continue
		}
		cidrs = append(cidrs, ipnet)
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			clientIP := c.RealIP()
			if clientIP == "" {
				return echo.NewHTTPError(http.StatusForbidden, "access denied: no client IP")
			}

			ip := net.ParseIP(clientIP)
			if ip == nil {
				return echo.NewHTTPError(http.StatusForbidden, "access denied: invalid client IP")
			}

			allowed := false
			for _, cidr := range cidrs {
				if cidr.Contains(ip) {
					allowed = true
					break
				}
			}

			if !allowed {
				return echo.NewHTTPError(http.StatusForbidden, "access denied: IP not whitelisted")
			}

			return next(c)
		}
	}
}
