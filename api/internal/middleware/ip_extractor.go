package middleware

import (
	"net"
	"net/http"
	"strings"

	"cloudpass/internal/config"

	"github.com/labstack/echo/v4"
)

// directPeerIP returns the TCP peer address without consulting any headers.
func directPeerIP(req *http.Request) string {
	host, _, err := net.SplitHostPort(req.RemoteAddr)
	if err != nil {
		if ip := net.ParseIP(req.RemoteAddr); ip != nil {
			return req.RemoteAddr
		}
		return ""
	}
	return host
}

// cleanHeaderIP normalizes a header-supplied IP (strips brackets); it
// returns "" when the value is not a parseable IP address.
func cleanHeaderIP(value string) string {
	trimmed := strings.TrimSpace(value)
	trimmed = strings.TrimPrefix(trimmed, "[")
	trimmed = strings.TrimSuffix(trimmed, "]")
	trimmed = strings.TrimSpace(trimmed)
	if net.ParseIP(trimmed) == nil {
		return ""
	}
	return trimmed
}

// peerTrusted reports whether the direct peer is covered by the configured
// trusted-proxy entries. Invalid entries are ignored (fail-secure: an
// unparseable allowlist entry never grants trust).
func peerTrusted(directIP string, trustedProxies []string) bool {
	ip := net.ParseIP(directIP)
	if ip == nil {
		return false
	}
	for _, entry := range trustedProxies {
		ipNet, _, err := config.ParseAllowedIPEntry(entry)
		if err != nil {
			continue
		}
		if ipNet.Contains(ip) {
			return true
		}
	}
	return false
}

// IPExtractorForConfig returns an echo.IPExtractor that honors
// X-Forwarded-For (leftmost entry) and X-Real-IP only when proxy headers are
// enabled AND the direct peer is a trusted proxy. Otherwise the direct peer
// address is returned. Live config is read per request, so updates apply
// without restart.
//
// Wire it in main: e.IPExtractor = middleware.IPExtractorForConfig(cfgManager)
// address is returned. Live config is read per request, so updates apply
// without restart.
//
// Wire it in main: e.IPExtractor = middleware.IPExtractorForConfig(cfgManager)
func IPExtractorForConfig(cfgManager *config.ConfigManager) echo.IPExtractor {
	return func(req *http.Request) string {
		direct := directPeerIP(req)
		if !cfgManager.GetEnableProxyHeader() {
			return direct
		}
		if !peerTrusted(direct, cfgManager.GetTrustedProxies()) {
			return direct
		}

		if xff := req.Header.Get("X-Forwarded-For"); xff != "" {
			if ip := cleanHeaderIP(strings.Split(xff, ",")[0]); ip != "" {
				return ip
			}
		}
		if xri := req.Header.Get("X-Real-IP"); xri != "" {
			if ip := cleanHeaderIP(xri); ip != "" {
				return ip
			}
		}
		return direct
	}
}

// ClientIP extracts the client IP from an Echo context using the same
// policy as IPExtractorForConfig: proxy headers are honored only when
// useProxyHeaders is true AND the direct peer is a trusted proxy.
// Handlers use it where they need the enforced client IP (e.g. the
// client-ip diagnostic endpoint).
func ClientIP(c echo.Context, useProxyHeaders bool, trustedProxies []string) string {
	direct := directPeerIP(c.Request())
	if !useProxyHeaders {
		return direct
	}
	if !peerTrusted(direct, trustedProxies) {
		return direct
	}

	if xff := c.Request().Header.Get("X-Forwarded-For"); xff != "" {
		if ip := cleanHeaderIP(strings.Split(xff, ",")[0]); ip != "" {
			return ip
		}
	}
	if xri := c.Request().Header.Get("X-Real-IP"); xri != "" {
		if ip := cleanHeaderIP(xri); ip != "" {
			return ip
		}
	}
	return direct
}
