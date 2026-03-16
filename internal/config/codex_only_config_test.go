package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigOptional_CodexOnlyIgnoresNonCodexBlocks(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")
	content := `
port: 8317
codex-api-key:
  - api-key: "codex-key"
    base-url: "https://codex.example.com"
legacy-provider-a:
  - api-key: "legacy-key-a"
legacy-provider-b:
  - api-key: "legacy-key-b"
legacy-provider-c:
  - name: "router"
    base-url: "https://router.example.com"
    api-key-entries:
      - api-key: "compat-key"
legacy-provider-d:
  - api-key: "legacy-key-d"
legacy-provider-e:
  upstream-url: "https://legacy.example.com"
`
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cfg, err := LoadConfigOptional(configPath, false)
	if err != nil {
		t.Fatalf("LoadConfigOptional() error = %v", err)
	}

	if len(cfg.CodexKey) != 1 {
		t.Fatalf("expected 1 codex key, got %d", len(cfg.CodexKey))
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
