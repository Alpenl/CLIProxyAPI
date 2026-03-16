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

	if lead := cliproxyauth.ProviderRefreshLead("other", nil); lead != nil {
		t.Fatalf("expected unsupported provider to have no refresh lead, got %v", *lead)
	}
}
