package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigOptional_IgnoresUnknownCredentialBlocks(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")
	content := `
port: 8317
legacy-credentials:
  - api-key: "legacy-key"
    endpoint: "https://legacy.example.com"
unknown-provider-a:
  - api-key: "legacy-key-a"
unknown-provider-b:
  - api-key: "legacy-key-b"
unknown-provider-c:
  - name: "router"
    base-url: "https://router.example.com"
    api-key-entries:
      - api-key: "compat-key"
unknown-provider-d:
  - api-key: "legacy-key-d"
unknown-provider-e:
  upstream-url: "https://legacy.example.com"
`
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cfg, err := LoadConfigOptional(configPath, false)
	if err != nil {
		t.Fatalf("LoadConfigOptional() error = %v", err)
	}

	if cfg.Port != 8317 {
		t.Fatalf("Port = %d, want 8317", cfg.Port)
	}
}

func TestLoadConfigOptional_DefaultUsageStatisticsEnabled(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")
	content := `
port: 8317
remote-management:
  secret-key: "test-secret"
api-keys:
  - "test-api-key"
`
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cfg, err := LoadConfigOptional(configPath, false)
	if err != nil {
		t.Fatalf("LoadConfigOptional() error = %v", err)
	}

	if !cfg.UsageStatisticsEnabled {
		t.Fatalf("UsageStatisticsEnabled = %t, want true", cfg.UsageStatisticsEnabled)
	}
}
