package handlers

import (
	"testing"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/registry"
	coreauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
	sdkconfig "github.com/router-for-me/CLIProxyAPI/v6/sdk/config"
)

func TestGetRequestDetails_PreservesSuffix(t *testing.T) {
	modelRegistry := registry.GetGlobalRegistry()
	now := time.Now().Unix()

	modelRegistry.RegisterClient("test-request-details-codex-primary", "codex", []*registry.ModelInfo{
		{ID: "gpt-5.3-codex", Created: now + 30},
		{ID: "gpt-5.2", Created: now + 20},
	})
	modelRegistry.RegisterClient("test-request-details-codex-secondary", "codex", []*registry.ModelInfo{
		{ID: "gpt-5-codex-mini", Created: now + 10},
	})

	// Ensure cleanup of all test registrations.
	clientIDs := []string{
		"test-request-details-codex-primary",
		"test-request-details-codex-secondary",
	}
	for _, clientID := range clientIDs {
		id := clientID
		t.Cleanup(func() {
			modelRegistry.UnregisterClient(id)
		})
	}

	handler := NewBaseAPIHandlers(&sdkconfig.SDKConfig{}, coreauth.NewManager(nil, nil, nil))

	tests := []struct {
		name       string
		inputModel string
		wantModel  string
		wantErr    bool
	}{
		{
			name:       "numeric suffix preserved",
			inputModel: "gpt-5.2(8192)",
			wantModel:  "gpt-5.2(8192)",
			wantErr:    false,
		},
		{
			name:       "level suffix preserved",
			inputModel: "gpt-5.2(high)",
			wantModel:  "gpt-5.2(high)",
			wantErr:    false,
		},
		{
			name:       "no suffix unchanged",
			inputModel: "gpt-5-codex-mini",
			wantModel:  "gpt-5-codex-mini",
			wantErr:    false,
		},
		{
			name:       "unknown model with suffix",
			inputModel: "unknown-model(8192)",
			wantModel:  "",
			wantErr:    true,
		},
		{
			name:       "auto suffix resolved",
			inputModel: "auto(high)",
			wantModel:  "gpt-5.3-codex(high)",
			wantErr:    false,
		},
		{
			name:       "special suffix none preserved",
			inputModel: "gpt-5.2(none)",
			wantModel:  "gpt-5.2(none)",
			wantErr:    false,
		},
		{
			name:       "special suffix auto preserved",
			inputModel: "gpt-5.3-codex(auto)",
			wantModel:  "gpt-5.3-codex(auto)",
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, model, errMsg := handler.getRequestDetails(tt.inputModel)
			if (errMsg != nil) != tt.wantErr {
				t.Fatalf("getRequestDetails() error = %v, wantErr %v", errMsg, tt.wantErr)
			}
			if errMsg != nil {
				return
			}
			if provider != "codex" {
				t.Fatalf("getRequestDetails() provider = %v, want %v", provider, "codex")
			}
			if model != tt.wantModel {
				t.Fatalf("getRequestDetails() model = %v, want %v", model, tt.wantModel)
			}
		})
	}
}
