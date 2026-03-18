package management

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/usage"
	coreusage "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/usage"
)

func TestGetUsageStatistics_ReturnsSummarySnapshotWithoutDetails(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler, _, _ := newCodexManagementTestHandler(t)
	stats := usage.NewRequestStatistics()
	stats.Record(context.Background(), coreusage.Record{
		APIKey:      "usage-summary-test",
		Provider:    "codex",
		Model:       "gpt-5-codex",
		RequestedAt: time.Unix(1700000000, 0).UTC(),
		Detail: coreusage.Detail{
			TotalTokens: 42,
		},
	})
	handler.SetUsageStatistics(stats)

	router := gin.New()
	router.GET("/v0/management/usage", handler.GetUsageStatistics)

	req := httptest.NewRequest(http.MethodGet, "/v0/management/usage", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
	}

	payload := decodeJSONBody(t, rr)
	usagePayload, ok := payload["usage"].(map[string]any)
	if !ok {
		t.Fatalf("expected usage object, got %#v", payload["usage"])
	}

	apis, ok := usagePayload["apis"].(map[string]any)
	if !ok {
		t.Fatalf("expected apis map, got %#v", usagePayload["apis"])
	}
	apiSnapshot, ok := apis["usage-summary-test"].(map[string]any)
	if !ok {
		t.Fatalf("expected api snapshot, got %#v", apis["usage-summary-test"])
	}
	models, ok := apiSnapshot["models"].(map[string]any)
	if !ok {
		t.Fatalf("expected models map, got %#v", apiSnapshot["models"])
	}
	modelSnapshot, ok := models["gpt-5-codex"].(map[string]any)
	if !ok {
		t.Fatalf("expected model snapshot, got %#v", models["gpt-5-codex"])
	}
	if _, exists := modelSnapshot["details"]; exists {
		t.Fatalf("expected usage summary response to omit details, got %#v", modelSnapshot["details"])
	}
}
