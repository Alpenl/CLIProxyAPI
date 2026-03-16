package auth

import (
	"context"
	"testing"

	internalconfig "github.com/router-for-me/CLIProxyAPI/v6/internal/config"
)

func TestLookupAPIKeyUpstreamModel_Codex(t *testing.T) {
	t.Parallel()

	cfg := &internalconfig.Config{
		CodexKey: []internalconfig.CodexKey{
			{
				APIKey:  "k",
				BaseURL: "https://example.com",
				Models: []internalconfig.CodexModel{
					{Name: "gpt-5-codex-20260301", Alias: "gpt-5-codex"},
					{Name: "gpt-5-codex-mini(low)", Alias: "gpt-5-codex-mini"},
				},
			},
		},
	}

	mgr := NewManager(nil, nil, nil)
	mgr.SetConfig(cfg)

	ctx := context.Background()
	_, _ = mgr.Register(ctx, &Auth{
		ID:       "a1",
		Provider: "codex",
		Attributes: map[string]string{
			"api_key":  "k",
			"base_url": "https://example.com",
		},
	})

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "alias with suffix", input: "gpt-5-codex(8192)", want: "gpt-5-codex-20260301(8192)"},
		{name: "alias without suffix", input: "gpt-5-codex", want: "gpt-5-codex-20260301"},
		{name: "config suffix priority", input: "gpt-5-codex-mini(high)", want: "gpt-5-codex-mini(low)"},
		{name: "uppercase alias", input: "GPT-5-CODEX", want: "gpt-5-codex-20260301"},
		{name: "upstream direct", input: "gpt-5-codex-20260301", want: "gpt-5-codex-20260301"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := mgr.lookupAPIKeyUpstreamModel("a1", tt.input); got != tt.want {
				t.Fatalf("lookupAPIKeyUpstreamModel(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestAPIKeyModelAlias_ConfigHotReload_Codex(t *testing.T) {
	t.Parallel()

	mgr := NewManager(nil, nil, nil)
	mgr.SetConfig(&internalconfig.Config{
		CodexKey: []internalconfig.CodexKey{
			{APIKey: "k", Models: []internalconfig.CodexModel{{Name: "gpt-5-codex-20260301", Alias: "gpt-5-codex"}}},
		},
	})

	ctx := context.Background()
	_, _ = mgr.Register(ctx, &Auth{ID: "a1", Provider: "codex", Attributes: map[string]string{"api_key": "k"}})

	if got := mgr.lookupAPIKeyUpstreamModel("a1", "gpt-5-codex"); got != "gpt-5-codex-20260301" {
		t.Fatalf("before reload got %q", got)
	}

	mgr.SetConfig(&internalconfig.Config{
		CodexKey: []internalconfig.CodexKey{
			{APIKey: "k", Models: []internalconfig.CodexModel{{Name: "gpt-5-codex-20260401", Alias: "gpt-5-codex"}}},
		},
	})

	if got := mgr.lookupAPIKeyUpstreamModel("a1", "gpt-5-codex"); got != "gpt-5-codex-20260401" {
		t.Fatalf("after reload got %q", got)
	}
}

func TestApplyAPIKeyModelAlias_CodexOnly(t *testing.T) {
	t.Parallel()

	cfg := &internalconfig.Config{
		CodexKey: []internalconfig.CodexKey{
			{APIKey: "k", Models: []internalconfig.CodexModel{{Name: "gpt-5-codex-20260301", Alias: "gpt-5-codex"}}},
		},
	}

	mgr := NewManager(nil, nil, nil)
	mgr.SetConfig(cfg)

	ctx := context.Background()
	apiKeyAuth := &Auth{ID: "a1", Provider: "codex", Attributes: map[string]string{"api_key": "k"}}
	oauthAuth := &Auth{ID: "oauth-auth", Provider: "codex", Attributes: map[string]string{"auth_kind": "oauth"}}
	_, _ = mgr.Register(ctx, apiKeyAuth)

	if got := mgr.applyAPIKeyModelAlias(apiKeyAuth, "gpt-5-codex(8192)"); got != "gpt-5-codex-20260301(8192)" {
		t.Fatalf("api key alias got %q", got)
	}
	if got := mgr.applyAPIKeyModelAlias(oauthAuth, "gpt-5-codex"); got != "gpt-5-codex" {
		t.Fatalf("oauth auth should passthrough, got %q", got)
	}
}
