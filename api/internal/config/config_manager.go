package config

import (
	"fmt"
	"net"
	"sort"
	"sync"
)

// DetectLocalNetworks scans all network interfaces and returns CIDR ranges
// for RFC1918 private networks (10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16)
// plus loopback ranges (127.0.0.0/8, ::1/128) and IPv6 ULA (fd00::/8).
func DetectLocalNetworks() []string {
	privateCIDRs := []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
	}

	privateNets := make([]*net.IPNet, 0, len(privateCIDRs))
	for _, cidr := range privateCIDRs {
		_, ipnet, err := net.ParseCIDR(cidr)
		if err != nil {
			continue
		}
		privateNets = append(privateNets, ipnet)
	}

	_, ipv6ULA, _ := net.ParseCIDR("fd00::/8")
	_, ipv6Loopback, _ := net.ParseCIDR("::1/128")

	addrs, err := net.InterfaceAddrs()
	if err != nil {
		fmt.Printf("Warning: failed to enumerate network interfaces: %v\n", err)
		return []string{"127.0.0.0/8"}
	}

	seen := make(map[string]bool)
	seen["127.0.0.0/8"] = true
	result := []string{"127.0.0.0/8"}

	for _, addr := range addrs {
		ipNet, ok := addr.(*net.IPNet)
		if !ok {
			continue
		}

		ip := ipNet.IP

		if ip4 := ip.To4(); ip4 != nil {
			for _, privateNet := range privateNets {
				if privateNet.Contains(ip) {
					cidr := ipNet.String()
					if !seen[cidr] {
						seen[cidr] = true
						result = append(result, cidr)
						fmt.Printf("  Detected local network: %s\n", cidr)
					}
					break
				}
			}
		} else if ip.To16() != nil {
			if ipv6ULA.Contains(ip) || ipv6Loopback.Contains(ip) {
				cidr := ipNet.String()
				if !seen[cidr] {
					seen[cidr] = true
					result = append(result, cidr)
					fmt.Printf("  Detected local network (IPv6): %s\n", cidr)
				}
			}
		}
	}

	sort.Strings(result)

	if len(result) == 1 {
		fmt.Println("  No local private networks detected, only loopback will be allowed")
	}

	return result
}

// ConfigManager provides thread-safe access to the application configuration
// and supports hot-reloading of certain settings.
type ConfigManager struct {
	mu         sync.RWMutex
	cfg        *Config
	configPath string
}

// NewConfigManager creates a new ConfigManager from an existing config.
func NewConfigManager(cfg *Config, configPath string) *ConfigManager {
	return &ConfigManager{
		cfg:        cfg,
		configPath: configPath,
	}
}

// Get returns a copy of the current configuration.
func (m *ConfigManager) Get() *Config {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cfgCopy := *m.cfg
	return &cfgCopy
}

// Update replaces the configuration and saves it to disk.
func (m *ConfigManager) Update(cfg *Config) error {
	m.mu.Lock()
	m.cfg = cfg
	m.mu.Unlock()

	return m.Save()
}

// Save writes the current configuration to disk.
func (m *ConfigManager) Save() error {
	m.mu.RLock()
	cfg := m.cfg
	m.mu.RUnlock()

	return Save(cfg, m.configPath)
}

// GetAllowedIPs returns the current allowed IP CIDRs.
func (m *ConfigManager) GetAllowedIPs() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ips := make([]string, len(m.cfg.Security.AllowedIPs))
	copy(ips, m.cfg.Security.AllowedIPs)
	return ips
}

// GetWebsocketTimeout returns the websocket idle timeout in minutes.
func (m *ConfigManager) GetWebsocketTimeout() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.cfg.Security.WebsocketTimeoutMin
}

// GetMultipassTimeout returns the multipass default timeout in seconds.
func (m *ConfigManager) GetMultipassTimeout() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.cfg.Multipass.DefaultTimeoutSec
}

// GetSSHKeyPath returns the SSH key path for terminal access.
func (m *ConfigManager) GetSSHKeyPath() string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.cfg.Multipass.SSHKeyPath
}

// GetSocketPath returns the multipass socket path.
func (m *ConfigManager) GetSocketPath() string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.cfg.Multipass.SocketPath
}

// GetLoggingConfig returns the current logging configuration.
func (m *ConfigManager) GetLoggingConfig() LoggingConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.cfg.Logging
}

// GetUploadConfig returns the current upload configuration.
func (m *ConfigManager) GetUploadConfig() UploadConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.cfg.Upload
}
