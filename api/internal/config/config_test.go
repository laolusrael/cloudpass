package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_FileNotFound(t *testing.T) {
	_, err := Load("/nonexistent/config.yaml")
	assert.Error(t, err)
}

func TestLoad_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(configPath, []byte("invalid: yaml: content:["), 0644)

	_, err := Load(configPath)
	assert.Error(t, err)
}

func TestLoad_ValidConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
server:
  host: "127.0.0.1"
  port: 9000
security:
  allowed_ips:
    - "192.168.1.0/24"
  websocket_idle_timeout_minutes: 60
multipass:
  socket_path: "/custom/socket"
  default_timeout_seconds: 120
logging:
  level: "debug"
  format: "text"
`
	os.WriteFile(configPath, []byte(configContent), 0644)

	cfg, err := Load(configPath)
	require.NoError(t, err)

	assert.Equal(t, "127.0.0.1", cfg.Server.Host)
	assert.Equal(t, 9000, cfg.Server.Port)
	assert.Equal(t, []string{"192.168.1.0/24"}, cfg.Security.AllowedIPs)
	assert.Equal(t, 60, cfg.Security.WebsocketTimeoutMin)
	assert.Equal(t, "/custom/socket", cfg.Multipass.SocketPath)
	assert.Equal(t, 120, cfg.Multipass.DefaultTimeoutSec)
	assert.Equal(t, "debug", cfg.Logging.Level)
	assert.Equal(t, "text", cfg.Logging.Format)
}

func TestLoad_EmptyConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(configPath, []byte(""), 0644)

	cfg, err := Load(configPath)
	require.NoError(t, err)

	assert.Equal(t, "0.0.0.0", cfg.Server.Host)
	assert.Equal(t, 8080, cfg.Server.Port)
	assert.Empty(t, cfg.Security.AllowedIPs)
	assert.Equal(t, 30, cfg.Security.WebsocketTimeoutMin)
	assert.Equal(t, DetectSocketPath(), cfg.Multipass.SocketPath)
	assert.Equal(t, 300, cfg.Multipass.DefaultTimeoutSec)
	assert.Equal(t, "info", cfg.Logging.Level)
	assert.Equal(t, "json", cfg.Logging.Format)
}

func TestLoad_PartialConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
server:
  port: 3000
logging:
  level: "warn"
`
	os.WriteFile(configPath, []byte(configContent), 0644)

	cfg, err := Load(configPath)
	require.NoError(t, err)

	assert.Equal(t, "0.0.0.0", cfg.Server.Host)
	assert.Equal(t, 3000, cfg.Server.Port)
	assert.Equal(t, "warn", cfg.Logging.Level)
}

func TestLoad_EnvironmentVariables(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(configPath, []byte("server:\n  port: 8080\n"), 0644)

	os.Setenv("CLOUDPASS_HOST", "192.168.1.1")
	os.Setenv("CLOUDPASS_PORT", "9999")
	os.Setenv("CLOUDPASS_LOG_LEVEL", "trace")
	defer func() {
		os.Unsetenv("CLOUDPASS_HOST")
		os.Unsetenv("CLOUDPASS_PORT")
		os.Unsetenv("CLOUDPASS_LOG_LEVEL")
	}()

	cfg, err := Load(configPath)
	require.NoError(t, err)

	assert.Equal(t, "192.168.1.1", cfg.Server.Host)
	assert.Equal(t, 9999, cfg.Server.Port)
	assert.Equal(t, "trace", cfg.Logging.Level)
}

func TestLoad_InvalidPortEnvVar(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(configPath, []byte("server:\n  port: 8080\n"), 0644)

	os.Setenv("CLOUDPASS_PORT", "not-a-number")
	defer os.Unsetenv("CLOUDPASS_PORT")

	cfg, err := Load(configPath)
	require.NoError(t, err)

	assert.Equal(t, 8080, cfg.Server.Port)
}
