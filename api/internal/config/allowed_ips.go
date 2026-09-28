package config

import (
	"fmt"
	"net"
	"strings"
)

// ParseAllowedIPEntry parses a single allowed_ips config entry.
//
// It accepts either CIDR notation ("192.168.1.0/24", "::1/128") or a plain
// IP address ("192.168.1.100", "::1"). Plain IPs are normalized to host
// routes ("/32" for IPv4, "/128" for IPv6) so callers can treat every entry
// uniformly as a *net.IPNet.
//
// Leading/trailing whitespace is ignored, and IPv6 zone identifiers
// ("fe80::1%eth0") are stripped before parsing; a prefix length on a zoned
// address ("fe80::1%eth0/64") is preserved.
//
// The returned string is the normalized form of the entry.
func ParseAllowedIPEntry(entry string) (*net.IPNet, string, error) {
	trimmed := strings.TrimSpace(entry)
	if trimmed == "" {
		return nil, "", fmt.Errorf("empty IP allowlist entry")
	}

	if _, ipNet, err := net.ParseCIDR(trimmed); err == nil {
		return ipNet, trimmed, nil
	}

	// Retry CIDR parsing with any IPv6 zone identifier stripped, preserving
	// the prefix length (e.g. "fe80::1%eth0/64" -> "fe80::1/64").
	if i := strings.LastIndex(trimmed, "/"); i != -1 {
		addrPart := trimmed[:i]
		if zi := strings.LastIndex(addrPart, "%"); zi != -1 {
			candidate := addrPart[:zi] + trimmed[i:]
			if _, ipNet, err := net.ParseCIDR(candidate); err == nil {
				return ipNet, candidate, nil
			}
		}
	}

	host := trimmed
	if i := strings.LastIndex(host, "%"); i != -1 {
		host = host[:i]
	}

	ip := net.ParseIP(strings.TrimSpace(host))
	if ip == nil {
		return nil, "", fmt.Errorf("invalid IP address or CIDR: %q", entry)
	}

	if ip.To4() != nil {
		_, ipNet, err := net.ParseCIDR(ip.String() + "/32")
		if err != nil {
			return nil, "", fmt.Errorf("failed to normalize IPv4 address %q: %w", entry, err)
		}
		return ipNet, ip.String() + "/32", nil
	}

	_, ipNet, err := net.ParseCIDR(ip.String() + "/128")
	if err != nil {
		return nil, "", fmt.Errorf("failed to normalize IPv6 address %q: %w", entry, err)
	}
	return ipNet, ip.String() + "/128", nil
}
