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
gemini-api-key:
  - api-key: "gemini-key"
claude-api-key:
  - api-key: "claude-key"
openai-compatibility:
  - name: "router"
    base-url: "https://router.example.com"
    api-key-entries:
      - api-key: "compat-key"
vertex-api-key:
  - api-key: "vertex-key"
ampcode:
  upstream-url: "https://amp.example.com"
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
