package cliproxy

import (
	"strings"
	"testing"

	coreauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
	"github.com/router-for-me/CLIProxyAPI/v6/sdk/config"
)

func TestRegisterModelsForAuth_UsesPreMergedExcludedModelsAttribute(t *testing.T) {
	service := &Service{cfg: &config.Config{}}
	auth := &coreauth.Auth{
		ID:       "auth-codex",
		Provider: "codex",
		Status:   coreauth.StatusActive,
		Attributes: map[string]string{
			"auth_kind":       "oauth",
			"excluded_models": "gpt-5-codex-mini",
			"plan_type":       "pro",
		},
	}

	registry := GlobalModelRegistry()
	registry.UnregisterClient(auth.ID)
	t.Cleanup(func() {
		registry.UnregisterClient(auth.ID)
	})

	service.registerModelsForAuth(auth)

	models := registry.GetAvailableModelsByProvider("codex")
	if len(models) == 0 {
		t.Fatal("expected codex models to be registered")
	}

	for _, model := range models {
		if model == nil {
			continue
		}
		modelID := strings.TrimSpace(model.ID)
		if strings.EqualFold(modelID, "gpt-5-codex-mini") {
			t.Fatalf("expected model %q to be excluded by auth attribute", modelID)
		}
	}

}

func TestRegisterModelsForAuth_IgnoresNonCodexProvider(t *testing.T) {
	service := &Service{cfg: &config.Config{}}
	auth := &coreauth.Auth{
		ID:       "auth-gemini",
		Provider: "gemini",
		Status:   coreauth.StatusActive,
	}

	registry := GlobalModelRegistry()
	registry.UnregisterClient(auth.ID)
	t.Cleanup(func() {
		registry.UnregisterClient(auth.ID)
	})

	service.registerModelsForAuth(auth)

	if models := registry.GetAvailableModelsByProvider("gemini"); len(models) != 0 {
		t.Fatalf("expected no gemini models to be registered, got %d", len(models))
	}
}
