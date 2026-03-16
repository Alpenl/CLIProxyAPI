package synthesizer

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
)

func TestFileSynthesizer_Synthesize_CodexFile(t *testing.T) {
	authDir := t.TempDir()
	payload, err := json.Marshal(map[string]any{
		"type":      "codex",
		"email":     "codex@example.com",
		"id_token":  "",
		"proxy_url": "socks5://127.0.0.1:1080",
		"prefix":    "prod",
	})
	if err != nil {
		t.Fatalf("failed to marshal auth file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(authDir, "codex-auth.json"), payload, 0o644); err != nil {
		t.Fatalf("failed to write auth file: %v", err)
	}

	synth := NewFileSynthesizer()
	auths, err := synth.Synthesize(&SynthesisContext{
		Config:  &config.Config{},
		AuthDir: authDir,
		Now:     time.Unix(1_700_000_000, 0).UTC(),
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(auths) != 1 {
		t.Fatalf("expected 1 auth, got %d", len(auths))
	}

	auth := auths[0]
	if auth.Provider != "codex" {
		t.Fatalf("expected codex provider, got %s", auth.Provider)
	}
	if auth.Label != "codex@example.com" {
		t.Fatalf("expected email label, got %s", auth.Label)
	}
	if auth.ProxyURL != "socks5://127.0.0.1:1080" {
		t.Fatalf("expected proxy url to be preserved, got %s", auth.ProxyURL)
	}
	if auth.Prefix != "prod" {
		t.Fatalf("expected prefix prod, got %s", auth.Prefix)
	}
	if auth.Attributes["path"] == "" {
		t.Fatal("expected auth file path to be recorded")
	}
	if auth.Attributes["auth_kind"] != "oauth" {
		t.Fatalf("expected oauth auth_kind, got %s", auth.Attributes["auth_kind"])
	}
}

func TestFileSynthesizer_Synthesize_CodexOnlyIgnoresNonCodexAuthFiles(t *testing.T) {
	authDir := t.TempDir()
	files := map[string]map[string]any{
		"claude-auth.json": {
			"type":  "claude",
			"email": "claude@example.com",
		},
		"gemini-auth.json": {
			"type":  "gemini",
			"email": "gemini@example.com",
		},
		"codex-auth.json": {
			"type":  "codex",
			"email": "codex@example.com",
		},
	}
	for name, payload := range files {
		data, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("failed to marshal %s: %v", name, err)
		}
		if err := os.WriteFile(filepath.Join(authDir, name), data, 0o644); err != nil {
			t.Fatalf("failed to write %s: %v", name, err)
		}
	}

	synth := NewFileSynthesizer()
	auths, err := synth.Synthesize(&SynthesisContext{
		Config:  &config.Config{},
		AuthDir: authDir,
		Now:     time.Unix(1_700_000_000, 0).UTC(),
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(auths) != 1 {
		t.Fatalf("expected only 1 codex auth, got %d", len(auths))
	}
	if auths[0].Provider != "codex" {
		t.Fatalf("expected codex provider, got %s", auths[0].Provider)
	}
}
