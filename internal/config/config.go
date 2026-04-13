package config

import (
	"errors"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

const (
	defaultHost       = "0.0.0.0"
	defaultPort       = "8443"
	defaultDataPath   = "/var/lib/petrel"
	defaultDockerSock = "/var/run/docker.sock"
)

// Config represents the wings daemon configuration.
type Config struct {
	PanelURL string       `yaml:"panel_url"`
	Token    string       `yaml:"token"`
	API      APIConfig    `yaml:"api"`
	DataPath string       `yaml:"data_path"`
	Docker   DockerConfig `yaml:"docker"`
}

// APIConfig configures the wings HTTP API endpoint.
type APIConfig struct {
	Host    string `yaml:"host"`
	Port    string `yaml:"port"`
	TLSCert string `yaml:"tls_cert"`
	TLSKey  string `yaml:"tls_key"`
}

// DockerConfig configures Docker integration.
type DockerConfig struct {
	Socket  string `yaml:"socket"`
	Network string `yaml:"network"`
}

// Load reads configuration from path, applies defaults, and validates required fields.
func Load(path string) (Config, error) {
	cfg := Config{
		API: APIConfig{
			Host: defaultHost,
			Port: defaultPort,
		},
		DataPath: defaultDataPath,
		Docker: DockerConfig{
			Socket: defaultDockerSock,
		},
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("reading config file: %w", err)
	}
	if err := yaml.Unmarshal(content, &cfg); err != nil {
		return Config{}, fmt.Errorf("parsing config yaml: %w", err)
	}

	if cfg.API.Host == "" {
		cfg.API.Host = defaultHost
	}
	if cfg.API.Port == "" {
		cfg.API.Port = defaultPort
	}
	if cfg.DataPath == "" {
		cfg.DataPath = defaultDataPath
	}
	if cfg.Docker.Socket == "" {
		cfg.Docker.Socket = defaultDockerSock
	}
	if cfg.Token == "" {
		return Config{}, errors.New("token is required")
	}

	return cfg, nil
}
