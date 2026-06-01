package middleware

import (
	"net"
	"net/http"

	"cloudpass/internal/config"
	"cloudpass/internal/logger"
	"cloudpass/internal/models"

	"github.com/labstack/echo/v4"
)

func IPWhitelist(cfgManager *config.ConfigManager) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			allowedCIDRs := cfgManager.GetAllowedIPs()

			cidrs := make([]*net.IPNet, 0, len(allowedCIDRs))
			for _, cidr := range allowedCIDRs {
				_, ipnet, err := net.ParseCIDR(cidr)
				if err != nil {
					logger.API.Load().Warn().Str("cidr", cidr).Msg("invalid CIDR in allowed_ips config")
					continue
				}
				cidrs = append(cidrs, ipnet)
			}

			clientIP := c.RealIP()
			if clientIP == "" {
				return c.JSON(http.StatusForbidden, models.ErrorResponse{
					Error:   "forbidden",
					Message: "access denied: no client IP",
				})
			}

			ip := net.ParseIP(clientIP)
			if ip == nil {
				return c.JSON(http.StatusForbidden, models.ErrorResponse{
					Error:   "forbidden",
					Message: "access denied: invalid client IP",
				})
			}

			allowed := false
			for _, cidr := range cidrs {
				if cidr.Contains(ip) {
					allowed = true
					break
				}
			}

			if !allowed {
				logger.API.Load().Warn().Str("ip", clientIP).Msg("access denied: IP not whitelisted")
				return c.JSON(http.StatusForbidden, models.ErrorResponse{
					Error:   "forbidden",
					Message: "access denied: IP not whitelisted",
				})
			}

			return next(c)
		}
	}
}
