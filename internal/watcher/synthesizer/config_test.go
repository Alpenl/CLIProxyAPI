package synthesizer

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
)

func TestConfigSynthesizer_Synthesize_IgnoresLegacyCredentialBlocks(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")
	content := `
port: 8317
legacy-config-credentials:
  - api-key: "sk-config-only"
    base-url: "https://legacy.example.com"
    prefix: "team-a"
    models:
      - name: "legacy-model"
        alias: "legacy-main"
`
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	synth := NewConfigSynthesizer()
	ctx := &SynthesisContext{
		Config:      cfg,
		Now:         time.Unix(1_700_000_000, 0).UTC(),
		IDGenerator: NewStableIDGenerator(),
	}

	auths, err := synth.Synthesize(ctx)
	if err != nil {
		t.Fatalf("Synthesize() error = %v", err)
	}
	if len(auths) != 0 {
		t.Fatalf("expected config synthesizer to ignore legacy credential blocks, got %d auths", len(auths))
	}
}

func TestConfigSynthesizer_Synthesize_EmptyConfig(t *testing.T) {
	synth := NewConfigSynthesizer()
	ctx := &SynthesisContext{
		Config:      &config.Config{},
		Now:         time.Unix(1_700_000_000, 0).UTC(),
		IDGenerator: NewStableIDGenerator(),
	}

	auths, err := synth.Synthesize(ctx)
	if err != nil {
		t.Fatalf("Synthesize() error = %v", err)
	}
	if len(auths) != 0 {
		t.Fatalf("expected no synthesized auths, got %d", len(auths))
	}
}
