package config

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server    ServerConfig    `yaml:"server"`
	Security  SecurityConfig  `yaml:"security"`
	Multipass MultipassConfig `yaml:"multipass"`
	Logging   LoggingConfig   `yaml:"logging"`
}

type ServerConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

type SecurityConfig struct {
	AllowedIPs          []string `yaml:"allowed_ips"`
	WebsocketTimeoutMin int      `yaml:"websocket_idle_timeout_minutes"`
}

type MultipassConfig struct {
	SocketPath        string `yaml:"socket_path"`
	DefaultTimeoutSec int    `yaml:"default_timeout_seconds"`
	SSHKeyPath        string `yaml:"ssh_key_path"`
	PassphraseEnv     string `yaml:"passphrase_env"`
	Passphrase        string `yaml:"-"`
}

type LoggingConfig struct {
	Level  string          `yaml:"level"`
	Format string          `yaml:"format"`
	Output string          `yaml:"output"`
	File   FileLogConfig   `yaml:"file"`
	Syslog SyslogConfig    `yaml:"syslog"`
	Levels ComponentLevels `yaml:"levels"`
}

type FileLogConfig struct {
	Path       string `yaml:"path"`
	MaxSizeMB  int    `yaml:"max_size_mb"`
	MaxBackups int    `yaml:"max_backups"`
	MaxAgeDays int    `yaml:"max_age_days"`
	Compress   bool   `yaml:"compress"`
}

type SyslogConfig struct {
	Address string `yaml:"address"`
	Network string `yaml:"network"`
}

type ComponentLevels struct {
	API       string `yaml:"api"`
	Multipass string `yaml:"multipass"`
	Websocket string `yaml:"websocket"`
}

var defaultConfig = Config{
	Server: ServerConfig{
		Host: "0.0.0.0",
		Port: 8080,
	},
	Security: SecurityConfig{
		AllowedIPs:          []string{},
		WebsocketTimeoutMin: 30,
	},
	Multipass: MultipassConfig{
		SocketPath:        "",
		DefaultTimeoutSec: 1800,
		SSHKeyPath:        "",
		PassphraseEnv:     "",
	},
	Logging: LoggingConfig{
		Level:  "info",
		Format: "console",
		Output: "stdout",
		File: FileLogConfig{
			Path:       "/var/log/cloudpass.log",
			MaxSizeMB:  10,
			MaxBackups: 5,
			MaxAgeDays: 30,
			Compress:   true,
		},
		Syslog: SyslogConfig{
			Address: "localhost:514",
			Network: "udp",
		},
		Levels: ComponentLevels{
			API:       "info",
			Multipass: "info",
			Websocket: "info",
		},
	},
}

func DetectMultipass() (bool, string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "multipass", "version")
	output, err := cmd.Output()
	if err != nil {
		return false, ""
	}

	parts := strings.Fields(string(output))
	if len(parts) >= 2 {
		return true, parts[1]
	}
	return true, ""
}

func GetMultipassVersion() string {
	_, version := DetectMultipass()
	return version
}

func DetectSocketPath() string {
	switch runtime.GOOS {
	case "windows":
		return `\\.\pipe\multipass`
	case "darwin":
		return "/var/run/multipass_socket"
	case "linux":
		socketPaths := []string{
			"/var/run/multipass_socket",
			"/run/multipass.socket",
			"/tmp/multipass.socket",
		}
		for _, p := range socketPaths {
			if _, err := os.Stat(p); err == nil {
				return p
			}
		}
		return "/var/run/multipass_socket"
	default:
		return "/var/run/multipass_socket"
	}
}

func EnsureConfig(path string, reset bool) (*Config, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve config path: %w", err)
	}

	exists := true
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		exists = false
	}

	if reset && exists {
		fmt.Printf("Config file exists at %s\n", absPath)
		fmt.Print("Reset to defaults? [y/N]: ")
		var response string
		if _, err := fmt.Scanln(&response); err != nil {
			fmt.Println("Aborted.")
			os.Exit(0)
		}
		if strings.ToLower(response) != "y" && strings.ToLower(response) != "yes" {
			fmt.Println("Aborted.")
			os.Exit(0)
		}
		if err := os.Remove(absPath); err != nil {
			return nil, fmt.Errorf("failed to remove config: %w", err)
		}
		exists = false
	}

	if exists {
		return Load(absPath)
	}

	mpAvailable, mpVersion := DetectMultipass()
	if !mpAvailable {
		fmt.Println("Warning: multipass not found. Install multipass to use cloudpass.")
	}

	cfg := defaultConfig
	cfg.Multipass.SocketPath = DetectSocketPath()

	if mpAvailable {
		fmt.Printf("Detected multipass %s\n", mpVersion)
	} else {
		fmt.Println("Multipass not detected")
	}

	cfgBytes, err := yaml.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal config: %w", err)
	}

	dir := filepath.Dir(absPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create config directory: %w", err)
	}

	if err := os.WriteFile(absPath, cfgBytes, 0644); err != nil {
		return nil, fmt.Errorf("failed to write config: %w", err)
	}

	fmt.Printf("Generated config file at %s\n", absPath)

	return &cfg, nil
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	cfg := defaultConfig

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	if cfg.Multipass.SocketPath == "" {
		cfg.Multipass.SocketPath = DetectSocketPath()
	}

	if cfg.Multipass.PassphraseEnv != "" {
		cfg.Multipass.Passphrase = os.Getenv(cfg.Multipass.PassphraseEnv)
		if cfg.Multipass.Passphrase == "" {
			fmt.Fprintf(os.Stderr, "Warning: passphrase_env '%s' is set but the environment variable is not defined\n", cfg.Multipass.PassphraseEnv)
			fmt.Fprintf(os.Stderr, "Multipass authentication may fail. Set CLOUDPASS_MULTIPASS_PASS or the configured env var.\n")
		}
	}

	if v := os.Getenv("CLOUDPASS_HOST"); v != "" {
		cfg.Server.Host = v
	}
	if v := os.Getenv("CLOUDPASS_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			cfg.Server.Port = port
		}
	}
	if v := os.Getenv("CLOUDPASS_LOG_LEVEL"); v != "" {
		cfg.Logging.Level = v
	}

	return &cfg, nil
}

func Save(cfg *Config, path string) error {
	var buf bytes.Buffer
	encoder := yaml.NewEncoder(&buf)
	encoder.SetIndent(2)
	if err := encoder.Encode(cfg); err != nil {
		return err
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	dir := filepath.Dir(absPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return os.WriteFile(absPath, buf.Bytes(), 0644)
}
