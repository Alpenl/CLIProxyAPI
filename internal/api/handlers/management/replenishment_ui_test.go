package management

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/managementui"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/replenishment"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/usage"
	coreusage "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/usage"
)

func TestGetReplenishmentStatus_ReturnsProviderPayload(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler, _, _ := newCodexManagementTestHandler(t)
	handler.SetReplenishmentCallbacks(
		func(_ context.Context) (ReplenishmentStatus, error) {
			return ReplenishmentStatus{
				Configured:         true,
				Enabled:            true,
				TargetAccountCount: 10,
				Pool: replenishment.PoolSnapshot{
					Target:   10,
					Healthy:  4,
					Warming:  1,
					Reserved: 2,
				},
				Deficit: 3,
			}, nil
		},
		nil,
	)

	router := gin.New()
	router.GET("/v0/management/replenishment/status", handler.GetReplenishmentStatus)

	req := httptest.NewRequest(http.MethodGet, "/v0/management/replenishment/status", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
	}

	payload := decodeJSONBody(t, rr)
	statusPayload, ok := payload["replenishment"].(map[string]any)
	if !ok {
		t.Fatalf("expected replenishment payload, got %#v", payload)
	}
	if got := int(statusPayload["targetAccountCount"].(float64)); got != 10 {
		t.Fatalf("targetAccountCount = %d, want 10", got)
	}
	if got := int(statusPayload["deficit"].(float64)); got != 3 {
		t.Fatalf("deficit = %d, want 3", got)
	}
}

func TestRunReplenishment_UsesTriggerCallback(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler, _, _ := newCodexManagementTestHandler(t)
	called := false
	handler.SetReplenishmentCallbacks(
		nil,
		func(_ context.Context) (ReplenishmentStatus, error) {
			called = true
			return ReplenishmentStatus{
				Configured:         true,
				Enabled:            true,
				TargetAccountCount: 10,
				CurrentJob: &replenishment.Job{
					ID:                 "job-1",
					Status:             "queued",
					RequestedSuccesses: 2,
				},
			}, nil
		},
	)

	router := gin.New()
	router.POST("/v0/management/replenishment/run", handler.RunReplenishment)

	req := httptest.NewRequest(http.MethodPost, "/v0/management/replenishment/run", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
	}
	if !called {
		t.Fatalf("expected trigger callback to be called")
	}

	payload := decodeJSONBody(t, rr)
	statusPayload, ok := payload["replenishment"].(map[string]any)
	if !ok {
		t.Fatalf("expected replenishment payload, got %#v", payload)
	}
	job, ok := statusPayload["currentJob"].(map[string]any)
	if !ok {
		t.Fatalf("expected currentJob payload, got %#v", statusPayload["currentJob"])
	}
	if got := job["id"]; got != "job-1" {
		t.Fatalf("currentJob.id = %#v, want job-1", got)
	}
}

func TestGetOverview_ReturnsAggregatedManagementPayload(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler, _, authDir := newCodexManagementTestHandler(t)
	stats := usage.NewRequestStatistics()
	for i := 0; i < 12; i++ {
		stats.Record(context.Background(), coreusage.Record{
			Provider:    "codex",
			Model:       "gpt-5-codex",
			APIKey:      "overview-test",
			RequestedAt: time.Unix(int64(1700000000+i), 0).UTC(),
			Failed:      i >= 9,
			Detail: coreusage.Detail{
				TotalTokens: 288,
			},
		})
	}
	handler.SetUsageStatistics(stats)
	handler.SetReplenishmentCallbacks(
		func(_ context.Context) (ReplenishmentStatus, error) {
			return ReplenishmentStatus{
				Configured:         true,
				Enabled:            true,
				TargetAccountCount: 10,
				Pool: replenishment.PoolSnapshot{
					Target:  10,
					Healthy: 4,
				},
				Deficit: 6,
			}, nil
		},
		nil,
	)

	codexPath := writeAuthJSONFile(t, authDir, "codex-overview.json", `{"type":"codex","email":"overview@example.com","refresh_token":"refresh-overview"}`)
	registerAuthFile(t, handler, codexPath)

	router := gin.New()
	router.GET("/v0/management/overview", handler.GetOverview)

	req := httptest.NewRequest(http.MethodGet, "/v0/management/overview", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
	}

	payload := decodeJSONBody(t, rr)
	if _, ok := payload["accounts"].([]any); !ok {
		t.Fatalf("expected accounts array, got %#v", payload["accounts"])
	}
	if usagePayload, ok := payload["usage"].(map[string]any); !ok {
		t.Fatalf("expected usage object, got %#v", payload["usage"])
	} else if got := int(usagePayload["total_requests"].(float64)); got != 12 {
		t.Fatalf("usage.total_requests = %d, want 12", got)
	}
	if replenishmentPayload, ok := payload["replenishment"].(map[string]any); !ok {
		t.Fatalf("expected replenishment object, got %#v", payload["replenishment"])
	} else if got := int(replenishmentPayload["deficit"].(float64)); got != 6 {
		t.Fatalf("replenishment.deficit = %d, want 6", got)
	}
}

func TestManagementUIHTML_ContainsReplenishmentControls(t *testing.T) {
	html, err := managementui.HTML()
	if err != nil {
		t.Fatalf("managementui.HTML() error = %v", err)
	}

	source := string(html)
	if !strings.Contains(source, "data-action=\"run-replenishment\"") {
		t.Fatalf("expected management UI to contain run-replenishment action")
	}
	if !strings.Contains(source, "自动补号状态") {
		t.Fatalf("expected management UI to contain replenishment status section")
	}
}
