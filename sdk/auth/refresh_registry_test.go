package auth

import (
	"testing"

	cliproxyauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
)

func TestRefreshRegistry_CodexOnlyProviders(t *testing.T) {
	t.Parallel()

	if lead := cliproxyauth.ProviderRefreshLead("codex", nil); lead == nil {
		t.Fatal("expected codex refresh lead to be registered")
	}

	for _, provider := range []string{
		"claude",
		"qwen",
		"iflow",
		"gemini",
		"gemini-cli",
		"antigravity",
		"kimi",
		"vertex",
		"openai-compatibility",
	} {
		if lead := cliproxyauth.ProviderRefreshLead(provider, nil); lead != nil {
			t.Fatalf("expected provider %q to have no refresh lead, got %v", provider, *lead)
		}
	}
}
