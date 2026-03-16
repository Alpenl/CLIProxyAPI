package synthesizer

import (
	"testing"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
)

func TestConfigSynthesizer_Synthesize_CodexKeys(t *testing.T) {
	synth := NewConfigSynthesizer()
	ctx := &SynthesisContext{
		Config: &config.Config{
			CodexKey: []config.CodexKey{
				{
					APIKey:   "codex-key-1",
					Prefix:   "team-a",
					BaseURL:  "https://codex.example.com",
					ProxyURL: "socks5://127.0.0.1:1080",
					Headers: map[string]string{
						"X-Test": "1",
					},
					Models: []config.CodexModel{
						{Name: "gpt-5-codex", Alias: "codex-main"},
					},
					ExcludedModels: []string{"gpt-5-codex-mini"},
				},
				{APIKey: "  "},
			},
		},
		Now:         time.Unix(1_700_000_000, 0).UTC(),
		IDGenerator: NewStableIDGenerator(),
	}

	auths, err := synth.Synthesize(ctx)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(auths) != 1 {
		t.Fatalf("expected 1 auth, got %d", len(auths))
	}

	auth := auths[0]
	if auth.Provider != "codex" {
		t.Fatalf("expected auth type codex, got %s", auth.Provider)
	}
	if auth.Label != "codex-apikey" {
		t.Fatalf("expected label codex-apikey, got %s", auth.Label)
	}
	if auth.Prefix != "team-a" {
		t.Fatalf("expected prefix team-a, got %s", auth.Prefix)
	}
	if auth.ProxyURL != "socks5://127.0.0.1:1080" {
		t.Fatalf("expected proxy url to be preserved, got %s", auth.ProxyURL)
	}
	if auth.Attributes["api_key"] != "codex-key-1" {
		t.Fatalf("expected api key to be preserved")
	}
	if auth.Attributes["base_url"] != "https://codex.example.com" {
		t.Fatalf("expected base url to be preserved")
	}
	if auth.Attributes["header:X-Test"] != "1" {
		t.Fatalf("expected custom header to be serialized")
	}
	if auth.Attributes["models_hash"] == "" {
		t.Fatal("expected models hash to be populated")
	}
	if auth.Attributes["excluded_models_hash"] == "" {
		t.Fatal("expected excluded models hash to be populated")
	}
	if auth.Attributes["auth_kind"] != "apikey" {
		t.Fatalf("expected auth_kind apikey, got %s", auth.Attributes["auth_kind"])
	}
}

func TestConfigSynthesizer_Synthesize_CodexOnlyIgnoresNonCodexProviders(t *testing.T) {
	synth := NewConfigSynthesizer()
	ctx := &SynthesisContext{
		Config: &config.Config{
			CodexKey: []config.CodexKey{{APIKey: "codex-key"}},
		},
		Now:         time.Unix(1_700_000_000, 0).UTC(),
		IDGenerator: NewStableIDGenerator(),
	}

	auths, err := synth.Synthesize(ctx)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(auths) != 1 {
		t.Fatalf("expected only 1 codex auth, got %d", len(auths))
	}
	if auths[0].Provider != "codex" {
		t.Fatalf("expected auth type codex, got %s", auths[0].Provider)
	}
}
