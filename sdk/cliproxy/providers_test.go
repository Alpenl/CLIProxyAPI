package cliproxy

import (
	"context"
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v6/sdk/config"
)

func TestAPIKeyClientProviderLoad_CodexOnly(t *testing.T) {
	provider := NewAPIKeyClientProvider()

	result, err := provider.Load(context.Background(), &config.Config{
		CodexKey: []config.CodexKey{{APIKey: "x1"}, {APIKey: "x2"}},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result == nil {
		t.Fatal("expected result")
	}
	if result.CodexKeyCount != 2 {
		t.Fatalf("expected 2 codex keys, got %d", result.CodexKeyCount)
	}
}
