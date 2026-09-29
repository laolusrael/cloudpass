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
	ipNet *net.IPNet
	raw   string
}

type IPWhitelistMiddleware struct {
	cfgManager        *config.ConfigManager
	parsedCIDRs       []parsedCIDR
	fallbackCIDRs     []parsedCIDR
	invalidEntries    []string
	mu                sync.RWMutex
	configVersion     int64
	enableProxyHeader bool
}

func newIPWhitelistMiddleware(cfgManager *config.ConfigManager, enableProxyHeader bool) *IPWhitelistMiddleware {
	m := &IPWhitelistMiddleware{
		cfgManager:        cfgManager,
		enableProxyHeader: enableProxyHeader,
	}
	m.refreshCIDRs()
	return m
}

func (m *IPWhitelistMiddleware) refreshCIDRs() {
	m.mu.Lock()
	defer m.mu.Unlock()

	allowedCIDRs := m.cfgManager.GetAllowedIPs()
	m.parsedCIDRs = make([]parsedCIDR, 0, len(allowedCIDRs))
	m.fallbackCIDRs = nil
	m.invalidEntries = nil
	m.configVersion = m.cfgManager.GetVersion()
	m.enableProxyHeader = m.cfgManager.GetEnableProxyHeader()

	var invalid []string
	for _, entry := range allowedCIDRs {
		ipNet, normalized, err := config.ParseAllowedIPEntry(entry)
		if err != nil {
			invalid = append(invalid, entry)
			continue
		}
		if ones, _ := ipNet.Mask.Size(); ones == 0 {
			logger.API.Load().Warn().Str("entry", normalized).Msg("allowed_ips entry permits all addresses")
		}
		m.parsedCIDRs = append(m.parsedCIDRs, parsedCIDR{
			ipNet: ipNet,
			raw:   normalized,
		})
	}

	if len(invalid) > 0 {
		logger.API.Load().Warn().Strs("entries", invalid).Msg("ignoring invalid allowed_ips entries")
	}

	if len(m.parsedCIDRs) > 0 {
		return
	}

	// Fail-closed when entries were explicitly configured but none parsed:
	// a typo must never silently widen access, so no fallback applies here.
	// (The deny message stays generic; details go to the server log only.)
	if len(allowedCIDRs) > 0 {
		m.invalidEntries = invalid
		logger.API.Load().Error().Strs("invalid_entries", invalid).Msg("no valid allowed IPs configured, denying all requests")
		return
	}

	// Empty allowlist means "auto-detect": fall back to local networks so a
	// fresh or reset config does not lock out legitimate local clients
	// (e.g. VM creation from localhost).
	for _, cidr := range config.DetectLocalNetworks() {
		ipNet, normalized, err := config.ParseAllowedIPEntry(cidr)
		if err != nil {
			continue
		}
		m.fallbackCIDRs = append(m.fallbackCIDRs, parsedCIDR{
			ipNet: ipNet,
			raw:   normalized,
		})
	}
	if len(m.fallbackCIDRs) == 0 {
		return
	}

	logger.API.Load().Warn().Msg("no allowed IPs configured, falling back to auto-detected local networks")
}

func (m *IPWhitelistMiddleware) getClientIP(c echo.Context) string {
	// Shared trust policy (see ClientIP): with headers disabled this uses
	// the direct peer address — c.RealIP() must NOT be used here because
	// Echo's fallback trusts X-Forwarded-For when no IPExtractor is set.
	return ClientIP(c, m.enableProxyHeader, m.cfgManager.GetTrustedProxies())
}

func (m *IPWhitelistMiddleware) isAllowed(ip net.IP) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, pc := range m.parsedCIDRs {
		if pc.ipNet.Contains(ip) {
			return true
		}
	}
	for _, pc := range m.fallbackCIDRs {
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
	}
}

func (m *IPWhitelistMiddleware) handleRequest(c echo.Context, next echo.HandlerFunc) error {
	m.refreshCIDRsIfNeeded()

	m.mu.RLock()
	cidrCount := len(m.parsedCIDRs) + len(m.fallbackCIDRs)
	invalidEntries := append([]string(nil), m.invalidEntries...)
	m.mu.RUnlock()

	if cidrCount == 0 {
		logger.API.Load().Warn().Strs("invalid_entries", invalidEntries).Msg("no usable allowed IPs and auto-detect yielded nothing, denying all requests")
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

	// Strip any IPv6 zone identifier (e.g. "fe80::1%eth0") before parsing.
	if i := strings.LastIndex(clientIP, "%"); i != -1 {
		clientIP = clientIP[:i]
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
	mw := newIPWhitelistMiddleware(cfgManager, cfgManager.GetEnableProxyHeader())

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			return mw.handleRequest(c, next)
		}
	}
}

func NewIPWhitelist(cfgManager *config.ConfigManager) echo.MiddlewareFunc {
	mw := newIPWhitelistMiddleware(cfgManager, cfgManager.GetEnableProxyHeader())

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			return mw.handleRequest(c, next)
		}
	}
}
