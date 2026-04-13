package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadAppliesDefaults(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.yml")
	if err := os.WriteFile(cfgPath, []byte("token: test-token\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.API.Host != "0.0.0.0" || cfg.API.Port != "8443" {
		t.Fatalf("unexpected api defaults: %#v", cfg.API)
	}
	if cfg.DataPath != "/var/lib/petrel" {
		t.Fatalf("unexpected data path: %s", cfg.DataPath)
	}
	if cfg.Docker.Socket != "/var/run/docker.sock" {
		t.Fatalf("unexpected docker socket: %s", cfg.Docker.Socket)
	}
}

func TestLoadRequiresToken(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.yml")
	if err := os.WriteFile(cfgPath, []byte("panel_url: https://example.com\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	if _, err := Load(cfgPath); err == nil {
		t.Fatal("expected token validation error")
	}
}
