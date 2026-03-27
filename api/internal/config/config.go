package config

import (
	"os"
	"strconv"

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
}

type LoggingConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	cfg := &Config{
		Server: ServerConfig{
			Host: "0.0.0.0",
			Port: 8080,
		},
		Security: SecurityConfig{
			AllowedIPs:          []string{},
			WebsocketTimeoutMin: 30,
		},
		Multipass: MultipassConfig{
			SocketPath:        "/var/run/multipass_socket",
			DefaultTimeoutSec: 300,
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "json",
		},
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
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

	return cfg, nil
}
