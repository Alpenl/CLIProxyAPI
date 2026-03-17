package management

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/managementui"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/replenishment"
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
