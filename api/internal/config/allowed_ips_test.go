package config

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func parseTestIP(t *testing.T, s string) net.IP {
	t.Helper()
	ip := net.ParseIP(s)
	require.NotNil(t, ip, "test IP %q must parse", s)
	return ip
}

func TestParseAllowedIPEntry(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		wantNormalized string
		wantContains   string
		wantErr        bool
	}{
		{"cidr passthrough", "192.168.1.0/24", "192.168.1.0/24", "192.168.1.100", false},
		{"single ipv4", "192.168.1.100", "192.168.1.100/32", "192.168.1.100", false},
		{"single ipv4 loopback", "127.0.0.1", "127.0.0.1/32", "127.0.0.1", false},
		{"single ipv6 loopback", "::1", "::1/128", "::1", false},
		{"ipv6 cidr", "fd00::/8", "fd00::/8", "fd00::1", false},
		{"whitespace trimmed", "  10.0.0.5  ", "10.0.0.5/32", "10.0.0.5", false},
		{"ipv6 zone stripped", "fe80::1%eth0", "fe80::1/128", "fe80::1", false},
		{"zoned cidr keeps mask", "fe80::1%eth0/64", "fe80::1/64", "fe80::abcd", false},
		{"empty", "", "", "", true},
		{"whitespace only", "   ", "", "", true},
		{"not an ip", "not-an-ip", "", "", true},
		{"invalid cidr", "192.168.1.0/33", "", "", true},
		{"hostname", "example.com", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ipNet, normalized, err := ParseAllowedIPEntry(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantNormalized, normalized)
			require.NotNil(t, ipNet)
			assert.True(t, ipNet.Contains(parseTestIP(t, tt.wantContains)), "expected %s to contain %s", normalized, tt.wantContains)
		})
	}
}

func TestParseAllowedIPEntry_SingleIPDoesNotContainNeighbor(t *testing.T) {
	ipNet, normalized, err := ParseAllowedIPEntry("192.168.1.100")
	require.NoError(t, err)
	assert.Equal(t, "192.168.1.100/32", normalized)
	assert.False(t, ipNet.Contains(parseTestIP(t, "192.168.1.101")))
}
