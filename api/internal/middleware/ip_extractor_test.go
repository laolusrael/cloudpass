package middleware

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"cloudpass/internal/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func extractorTestManager(enableProxyHeader bool, trustedProxies []string) *config.ConfigManager {
	return config.NewConfigManager(&config.Config{
		Security: config.SecurityConfig{
			EnableProxyHeader: enableProxyHeader,
			TrustedProxies:    trustedProxies,
		},
	}, "")
}

func extractorTestRequest(remoteAddr, xff, xri string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = remoteAddr
	if xff != "" {
		req.Header.Set("X-Forwarded-For", xff)
	}
	if xri != "" {
		req.Header.Set("X-Real-IP", xri)
	}
	return req
}

func TestIPExtractor_DisabledIgnoresHeaders(t *testing.T) {
	extract := IPExtractorForConfig(extractorTestManager(false, []string{"0.0.0.0/0"}))

	got := extract(extractorTestRequest("203.0.113.7:1234", "192.168.1.100", "192.168.1.101"))
	assert.Equal(t, "203.0.113.7", got)
}

func TestIPExtractor_TrustedProxyHonorsXFF(t *testing.T) {
	extract := IPExtractorForConfig(extractorTestManager(true, []string{"10.0.0.5"}))

	got := extract(extractorTestRequest("10.0.0.5:1234", "192.168.1.100, 10.0.0.1", ""))
	assert.Equal(t, "192.168.1.100", got)
}

func TestIPExtractor_UntrustedPeerIgnoresHeaders(t *testing.T) {
	extract := IPExtractorForConfig(extractorTestManager(true, []string{"10.0.0.5"}))

	got := extract(extractorTestRequest("203.0.113.7:1234", "192.168.1.100", "192.168.1.101"))
	assert.Equal(t, "203.0.113.7", got)
}

func TestIPExtractor_FallsBackToXRealIP(t *testing.T) {
	extract := IPExtractorForConfig(extractorTestManager(true, []string{"10.0.0.0/8"}))

	got := extract(extractorTestRequest("10.1.2.3:1234", "", "192.168.1.50"))
	assert.Equal(t, "192.168.1.50", got)
}

func TestIPExtractor_XFFTakesPrecedenceOverXRealIP(t *testing.T) {
	extract := IPExtractorForConfig(extractorTestManager(true, []string{"127.0.0.0/8"}))

	got := extract(extractorTestRequest("127.0.0.1:1234", "192.168.1.100", "192.168.1.101"))
	assert.Equal(t, "192.168.1.100", got)
}

func TestIPExtractor_SkipsGarbageHeaderValues(t *testing.T) {
	extract := IPExtractorForConfig(extractorTestManager(true, []string{"127.0.0.0/8"}))

	got := extract(extractorTestRequest("127.0.0.1:1234", "not-an-ip", ""))
	assert.Equal(t, "127.0.0.1", got)

	got = extract(extractorTestRequest("127.0.0.1:1234", "not-an-ip", "also-bad"))
	assert.Equal(t, "127.0.0.1", got)
}

func TestIPExtractor_EmptyTrustedListTrustsNobody(t *testing.T) {
	extract := IPExtractorForConfig(extractorTestManager(true, nil))

	got := extract(extractorTestRequest("127.0.0.1:1234", "192.168.1.100", "192.168.1.101"))
	assert.Equal(t, "127.0.0.1", got)
}

func TestIPExtractor_InvalidTrustedEntriesIgnored(t *testing.T) {
	extract := IPExtractorForConfig(extractorTestManager(true, []string{"bogus", "10.0.0.5"}))

	got := extract(extractorTestRequest("10.0.0.5:1234", "192.168.1.100", ""))
	assert.Equal(t, "192.168.1.100", got)

	got = extract(extractorTestRequest("203.0.113.7:1234", "192.168.1.100", ""))
	assert.Equal(t, "203.0.113.7", got)
}

func TestIPExtractor_ReadsLiveConfig(t *testing.T) {
	mgr := config.NewConfigManager(&config.Config{
		Security: config.SecurityConfig{
			EnableProxyHeader: true,
			TrustedProxies:    []string{"10.0.0.5"},
		},
	}, filepath.Join(t.TempDir(), "config.yaml"))

	extract := IPExtractorForConfig(mgr)
	req := extractorTestRequest("10.0.0.5:1234", "192.168.1.100", "")
	assert.Equal(t, "192.168.1.100", extract(req))

	cfg := mgr.Get()
	cfg.Security.EnableProxyHeader = false
	require.NoError(t, mgr.Update(cfg))

	assert.Equal(t, "10.0.0.5", extract(req))
}
