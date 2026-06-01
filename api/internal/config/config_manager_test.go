package config

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDetectLocalNetworks(t *testing.T) {
	networks := DetectLocalNetworks()

	assert.NotEmpty(t, networks)
	assert.Contains(t, networks, "127.0.0.0/8")

	for _, cidr := range networks {
		_, _, err := net.ParseCIDR(cidr)
		assert.NoError(t, err, "invalid CIDR: %s", cidr)
	}
}

func TestConfigManager_Get(t *testing.T) {
	cfg := &Config{
		Server: ServerConfig{Host: "127.0.0.1", Port: 9000},
		Security: SecurityConfig{
			AllowedIPs:          []string{"10.0.0.0/8"},
			WebsocketTimeoutMin: 60,
		},
	}
	mgr := NewConfigManager(cfg, "")

	got := mgr.Get()
	assert.Equal(t, "127.0.0.1", got.Server.Host)
	assert.Equal(t, 9000, got.Server.Port)
	assert.Equal(t, []string{"10.0.0.0/8"}, got.Security.AllowedIPs)

	got.Server.Host = "modified"
	assert.Equal(t, "127.0.0.1", mgr.Get().Server.Host, "Get should return independent copy")
}

func TestConfigManager_GetAllowedIPs(t *testing.T) {
	cfg := &Config{
		Security: SecurityConfig{
			AllowedIPs: []string{"192.168.1.0/24", "10.0.0.0/8"},
		},
	}
	mgr := NewConfigManager(cfg, "")

	ips := mgr.GetAllowedIPs()
	assert.Equal(t, []string{"192.168.1.0/24", "10.0.0.0/8"}, ips)

	ips[0] = "modified"
	assert.Equal(t, "192.168.1.0/24", mgr.GetAllowedIPs()[0], "GetAllowedIPs should return independent copy")
}

func TestConfigManager_Update(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	cfg := &Config{
		Server: ServerConfig{Host: "0.0.0.0", Port: 8080},
		Security: SecurityConfig{
			AllowedIPs:          []string{"127.0.0.0/8"},
			WebsocketTimeoutMin: 30,
		},
		Multipass: MultipassConfig{
			DefaultTimeoutSec: 300,
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "console",
			Output: "stdout",
		},
	}
	mgr := NewConfigManager(cfg, configPath)

	cfg.Security.AllowedIPs = []string{"192.168.1.0/24"}
	require.NoError(t, mgr.Update(cfg))

	assert.Equal(t, []string{"192.168.1.0/24"}, mgr.GetAllowedIPs())

	data, err := os.ReadFile(configPath)
	require.NoError(t, err)
	assert.Contains(t, string(data), "192.168.1.0/24")
}

func TestConfigManager_ConcurrentAccess(t *testing.T) {
	cfg := &Config{
		Security: SecurityConfig{
			AllowedIPs: []string{"10.0.0.0/8"},
		},
	}
	mgr := NewConfigManager(cfg, "")

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(2)
		go func(i int) {
			defer wg.Done()
			ips := mgr.GetAllowedIPs()
			assert.NotEmpty(t, ips)
		}(i)
		go func(i int) {
			defer wg.Done()
			newCfg := mgr.Get()
			newCfg.Security.AllowedIPs = []string{fmt.Sprintf("10.%d.0.0/8", i)}
			_ = mgr.Update(newCfg)
		}(i)
	}
	wg.Wait()
}

func TestConfigManager_Getters(t *testing.T) {
	cfg := &Config{
		Security: SecurityConfig{
			AllowedIPs:          []string{"10.0.0.0/8"},
			WebsocketTimeoutMin: 45,
		},
		Multipass: MultipassConfig{
			DefaultTimeoutSec: 600,
			SocketPath:        "/custom/socket",
			SSHKeyPath:        "/custom/key",
		},
		Upload: UploadConfig{
			MaxFileSizeMB: 50,
			DefaultPath:   "/custom/uploads",
		},
		Logging: LoggingConfig{
			Level:  "debug",
			Format: "json",
			Output: "file",
		},
	}
	mgr := NewConfigManager(cfg, "")

	assert.Equal(t, []string{"10.0.0.0/8"}, mgr.GetAllowedIPs())
	assert.Equal(t, 45, mgr.GetWebsocketTimeout())
	assert.Equal(t, 600, mgr.GetMultipassTimeout())
	assert.Equal(t, "/custom/socket", mgr.GetSocketPath())
	assert.Equal(t, "/custom/key", mgr.GetSSHKeyPath())
	assert.Equal(t, "debug", mgr.GetLoggingConfig().Level)
	assert.Equal(t, 50, mgr.GetUploadConfig().MaxFileSizeMB)
}

func TestLoad_EmptyConfig_AutoDetectsNetworks(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte(""), 0644))

	cfg, err := Load(configPath)
	require.NoError(t, err)

	assert.NotEmpty(t, cfg.Security.AllowedIPs)
	assert.Contains(t, cfg.Security.AllowedIPs, "127.0.0.0/8")
}

func TestLoad_ExplicitAllowedIPs(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
security:
  allowed_ips:
    - "10.0.0.0/8"
    - "172.16.0.0/12"
`
	require.NoError(t, os.WriteFile(configPath, []byte(configContent), 0644))

	cfg, err := Load(configPath)
	require.NoError(t, err)

	assert.Equal(t, []string{"10.0.0.0/8", "172.16.0.0/12"}, cfg.Security.AllowedIPs)
}
