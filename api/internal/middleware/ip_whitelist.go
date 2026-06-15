package middleware

import (
	"net"
	"net/http"
	"strings"
	"sync"

	"cloudpass/internal/config"
	"cloudpass/internal/logger"
	"cloudpass/internal/models"

	"github.com/labstack/echo/v4"
)

type parsedCIDR struct {
	ipNet  *net.IPNet
	raw    string
}

type IPWhitelistMiddleware struct {
	cfgManager        *config.ConfigManager
	parsedCIDRs       []parsedCIDR
	mu                sync.RWMutex
	configVersion     int64
	enableProxyHeader bool
}

func newIPWhitelistMiddleware(cfgManager *config.ConfigManager, enableProxyHeader bool) *IPWhitelistMiddleware {
	m := &IPWhitelistMiddleware{
		cfgManager:        cfgManager,
		enableProxyHeader: enableProxyHeader,
		configVersion:     cfgManager.GetVersion(),
	}
	m.refreshCIDRs()
	return m
}

func (m *IPWhitelistMiddleware) refreshCIDRs() {
	m.mu.Lock()
	defer m.mu.Unlock()

	allowedCIDRs := m.cfgManager.GetAllowedIPs()
	m.parsedCIDRs = make([]parsedCIDR, 0, len(allowedCIDRs))

	for _, cidr := range allowedCIDRs {
		_, ipnet, err := net.ParseCIDR(cidr)
		if err != nil {
			logger.API.Load().Warn().Str("cidr", cidr).Msg("invalid CIDR in allowed_ips config")
			continue
		}
		m.parsedCIDRs = append(m.parsedCIDRs, parsedCIDR{
			ipNet: ipnet,
			raw:   cidr,
		})
	}
}

func (m *IPWhitelistMiddleware) getClientIP(c echo.Context) string {
	if m.enableProxyHeader {
		xff := c.Request().Header.Get("X-Forwarded-For")
		if xff != "" {
			ips := strings.Split(xff, ",")
			if len(ips) > 0 {
				trimmed := strings.TrimSpace(ips[0])
				if trimmed != "" {
					return trimmed
				}
			}
		}

		xri := c.Request().Header.Get("X-Real-IP")
		if xri != "" {
			return strings.TrimSpace(xri)
		}
	}

	return c.RealIP()
}

func (m *IPWhitelistMiddleware) isAllowed(ip net.IP) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, pc := range m.parsedCIDRs {
		if pc.ipNet.Contains(ip) {
			return true
		}
	}
	return false
}

func (m *IPWhitelistMiddleware) refreshCIDRsIfNeeded() {
	currentVersion := m.cfgManager.GetVersion()
	
	m.mu.RLock()
	storedVersion := m.configVersion
	m.mu.RUnlock()

	if currentVersion != storedVersion {
		m.refreshCIDRs()
		m.mu.Lock()
		m.configVersion = currentVersion
		m.mu.Unlock()
	}
}

func (m *IPWhitelistMiddleware) handleRequest(c echo.Context, next echo.HandlerFunc) error {
	m.refreshCIDRsIfNeeded()

	m.mu.RLock()
	cidrCount := len(m.parsedCIDRs)
	m.mu.RUnlock()

	if cidrCount == 0 {
		logger.API.Load().Warn().Msg("no allowed IPs configured, denying all requests")
		return c.JSON(http.StatusForbidden, models.ErrorResponse{
			Error:   "forbidden",
			Message: "access denied: no allowed IPs configured",
		})
	}

	clientIP := m.getClientIP(c)
	if clientIP == "" {
		return c.JSON(http.StatusForbidden, models.ErrorResponse{
			Error:   "forbidden",
			Message: "access denied: no client IP",
		})
	}

	ip := net.ParseIP(clientIP)
	if ip == nil {
		logger.API.Load().Warn().Str("ip", clientIP).Msg("invalid client IP format")
		return c.JSON(http.StatusForbidden, models.ErrorResponse{
			Error:   "forbidden",
			Message: "access denied: invalid client IP",
		})
	}

	if !m.isAllowed(ip) {
		logger.API.Load().Warn().Str("ip", clientIP).Msg("access denied: IP not whitelisted")
		return c.JSON(http.StatusForbidden, models.ErrorResponse{
			Error:   "forbidden",
			Message: "access denied: IP not whitelisted",
		})
	}

	return next(c)
}

func IPWhitelist(cfgManager *config.ConfigManager) echo.MiddlewareFunc {
	enableProxyHeader := true

	mw := newIPWhitelistMiddleware(cfgManager, enableProxyHeader)

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			return mw.handleRequest(c, next)
		}
	}
}

func NewIPWhitelist(cfgManager *config.ConfigManager) echo.MiddlewareFunc {
	enableProxyHeader := true

	mw := newIPWhitelistMiddleware(cfgManager, enableProxyHeader)

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			return mw.handleRequest(c, next)
		}
	}
}
