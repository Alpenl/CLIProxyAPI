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

	models := registry.GetAvailableCodexModels()
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
		ID:       "auth-other",
		Provider: "other",
		Status:   coreauth.StatusActive,
	}

	registry := GlobalModelRegistry()
	registry.UnregisterClient(auth.ID)
	t.Cleanup(func() {
		registry.UnregisterClient(auth.ID)
	})

	service.registerModelsForAuth(auth)

	if models := registry.GetModelsForClient(auth.ID); len(models) != 0 {
		t.Fatalf("expected no models to be registered for non-codex auth, got %d", len(models))
	}
}
