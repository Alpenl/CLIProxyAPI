package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadOrCreateConfig_CreatesBootstrapConfigWhenMissing(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	cfg, created, err := LoadOrCreateConfig(configPath)
	if err != nil {
		t.Fatalf("LoadOrCreateConfig() error = %v", err)
	}
	if !created {
		t.Fatalf("LoadOrCreateConfig() created = false, want true")
	}
	if cfg == nil {
		t.Fatalf("LoadOrCreateConfig() cfg = nil")
	}
	if cfg.Port != 8317 {
		t.Fatalf("Port = %d, want 8317", cfg.Port)
	}
	if !cfg.RemoteManagement.AllowRemote {
		t.Fatalf("RemoteManagement.AllowRemote = false, want true")
	}
	if cfg.RemoteManagement.SecretKey != "" {
		t.Fatalf("RemoteManagement.SecretKey = %q, want empty", cfg.RemoteManagement.SecretKey)
	}
	if !cfg.UsageStatisticsEnabled {
		t.Fatalf("UsageStatisticsEnabled = false, want true")
	}
	if len(cfg.APIKeys) != 0 {
		t.Fatalf("APIKeys len = %d, want 0", len(cfg.APIKeys))
	}
	if strings.TrimSpace(cfg.AuthDir) == "" {
		t.Fatalf("AuthDir is empty")
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", configPath, err)
	}
	content := string(data)
	if !strings.Contains(content, "remote-management:") {
		t.Fatalf("generated config missing remote-management block: %s", content)
	}
	if !strings.Contains(content, "secret-key: \"\"") {
		t.Fatalf("generated config missing empty secret-key field: %s", content)
	}
	if strings.Contains(content, "disable-control-panel:") {
		t.Fatalf("generated config should not include disable-control-panel: %s", content)
	}
	if strings.Contains(content, "panel-github-repository:") {
		t.Fatalf("generated config should not include panel-github-repository: %s", content)
	}
	if strings.Contains(content, "ws-auth:") {
		t.Fatalf("generated config should not include ws-auth: %s", content)
	}
	if strings.Contains(content, "quota-exceeded:") {
		t.Fatalf("generated config should not include quota-exceeded: %s", content)
	}
}
