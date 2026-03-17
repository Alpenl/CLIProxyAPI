package management

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	coreauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
)

func writeManagementConfigFile(t *testing.T, dir string, content string) string {
	t.Helper()

	configPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(strings.TrimSpace(content)+"\n"), 0o600); err != nil {
		t.Fatalf("failed to write config file %s: %v", configPath, err)
	}
	return configPath
}

func newConfigManagementTestHandler(t *testing.T, content string) (*Handler, string) {
	t.Helper()

	gin.SetMode(gin.TestMode)

	tmpDir := t.TempDir()
	authDir := filepath.Join(tmpDir, "auths")
	if err := os.MkdirAll(authDir, 0o700); err != nil {
		t.Fatalf("failed to create auth dir: %v", err)
	}

	configPath := writeManagementConfigFile(t, tmpDir, content)
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		t.Fatalf("failed to load config file %s: %v", configPath, err)
	}
	cfg.AuthDir = authDir

	handler := NewHandler(cfg, configPath, coreauth.NewManager(nil, nil, nil))
	return handler, configPath
}

func decodeManagementResponse(t *testing.T, rr *httptest.ResponseRecorder) map[string]any {
	t.Helper()

	var payload map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	return payload
}

func TestGetBootstrapStatus_ReportsSetupRequired(t *testing.T) {
	handler, _ := newConfigManagementTestHandler(t, `
host: ""
port: 8317
remote-management:
  allow-remote: true
  secret-key: ""
auth-dir: "./auths"
api-keys: []
usage-statistics-enabled: true
`)

	router := gin.New()
	router.GET("/v0/management/bootstrap/status", handler.GetBootstrapStatus)

	req := httptest.NewRequest(http.MethodGet, "/v0/management/bootstrap/status", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
	}

	payload := decodeManagementResponse(t, rr)
	if payload["bootstrap_required"] != true {
		t.Fatalf("bootstrap_required = %#v, want true", payload["bootstrap_required"])
	}
}

func TestUpdateBootstrapConfig_PersistsHashedSecret(t *testing.T) {
	handler, configPath := newConfigManagementTestHandler(t, `
host: ""
port: 8317
remote-management:
  allow-remote: true
  secret-key: ""
auth-dir: "./auths"
api-keys: []
usage-statistics-enabled: true
`)

	router := gin.New()
	router.PUT("/v0/management/bootstrap/config", handler.UpdateBootstrapConfig)

	body := bytes.NewBufferString(`{
  "host": "",
  "port": 8317,
  "authDir": "./auths",
  "apiKeys": [],
  "management": {
    "allowRemote": true,
    "secretKey": "bootstrap-secret"
  },
  "usageStatisticsEnabled": true,
  "routingStrategy": "round-robin"
}`)
	req := httptest.NewRequest(http.MethodPut, "/v0/management/bootstrap/config", body)
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
	}

	payload := decodeManagementResponse(t, rr)
	if payload["bootstrap_required"] != false {
		t.Fatalf("bootstrap_required = %#v, want false", payload["bootstrap_required"])
	}

	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		t.Fatalf("failed to reload config file %s: %v", configPath, err)
	}
	if cfg.RemoteManagement.SecretKey == "" {
		t.Fatalf("RemoteManagement.SecretKey is empty after bootstrap save")
	}
	if cfg.RemoteManagement.SecretKey == "bootstrap-secret" {
		t.Fatalf("RemoteManagement.SecretKey stored plaintext, want bcrypt hash")
	}
	if len(cfg.APIKeys) != 0 {
		t.Fatalf("APIKeys len = %d, want 0", len(cfg.APIKeys))
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("failed to read config file: %v", err)
	}
	if strings.Contains(string(data), "bootstrap-secret") {
		t.Fatalf("config file still contains plaintext secret: %s", string(data))
	}
}

func TestGetWebConfig_RedactsManagementSecret(t *testing.T) {
	handler, _ := newConfigManagementTestHandler(t, `
host: ""
port: 8317
remote-management:
  allow-remote: true
  secret-key: "bootstrap-secret"
auth-dir: "./auths"
api-keys:
  - "client-a"
usage-statistics-enabled: true
`)

	router := gin.New()
	router.GET("/v0/management/config", handler.GetWebConfig)

	req := httptest.NewRequest(http.MethodGet, "/v0/management/config", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
	}

	payload := decodeManagementResponse(t, rr)
	configPayload, ok := payload["config"].(map[string]any)
	if !ok {
		t.Fatalf("expected config object, got %#v", payload["config"])
	}
	managementPayload, ok := configPayload["management"].(map[string]any)
	if !ok {
		t.Fatalf("expected management object, got %#v", configPayload["management"])
	}
	if managementPayload["secretConfigured"] != true {
		t.Fatalf("secretConfigured = %#v, want true", managementPayload["secretConfigured"])
	}
	if got := managementPayload["secretKey"]; got != "" {
		t.Fatalf("secretKey = %#v, want empty string", got)
	}
}

func TestGetWebConfig_RedactsReplenishmentServiceToken(t *testing.T) {
	handler, _ := newConfigManagementTestHandler(t, `
host: ""
port: 8317
remote-management:
  allow-remote: true
  secret-key: "bootstrap-secret"
auth-dir: "./auths"
api-keys:
  - "client-a"
replenishment:
  enabled: true
  target-account-count: 12
  check-interval-seconds: 300
  quota-refresh-interval-seconds: 3600
  cleanup-invalid-accounts: true
  service-url: "http://cdx-rt:3080"
  service-token: "service-secret"
usage-statistics-enabled: true
`)

	router := gin.New()
	router.GET("/v0/management/config", handler.GetWebConfig)

	req := httptest.NewRequest(http.MethodGet, "/v0/management/config", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
	}

	payload := decodeManagementResponse(t, rr)
	configPayload, ok := payload["config"].(map[string]any)
	if !ok {
		t.Fatalf("expected config object, got %#v", payload["config"])
	}
	replenishmentPayload, ok := configPayload["replenishment"].(map[string]any)
	if !ok {
		t.Fatalf("expected replenishment object, got %#v", configPayload["replenishment"])
	}
	if replenishmentPayload["serviceTokenConfigured"] != true {
		t.Fatalf("serviceTokenConfigured = %#v, want true", replenishmentPayload["serviceTokenConfigured"])
	}
	if got := replenishmentPayload["serviceToken"]; got != "" {
		t.Fatalf("serviceToken = %#v, want empty string", got)
	}
}

func TestUpdateWebConfig_RejectsInvalidReplenishmentURL(t *testing.T) {
	handler, _ := newConfigManagementTestHandler(t, `
host: ""
port: 8317
remote-management:
  allow-remote: true
  secret-key: "bootstrap-secret"
auth-dir: "./auths"
api-keys:
  - "client-a"
usage-statistics-enabled: true
`)

	router := gin.New()
	router.PUT("/v0/management/config", handler.UpdateWebConfig)

	body := bytes.NewBufferString(`{
  "host": "",
  "port": 8317,
  "authDir": "./auths",
  "apiKeys": ["client-a"],
  "management": {
    "allowRemote": true,
    "secretKey": ""
  },
  "replenishment": {
    "enabled": true,
    "targetAccountCount": 10,
    "checkIntervalSeconds": 300,
    "quotaRefreshIntervalSeconds": 3600,
    "cleanupInvalidAccounts": true,
    "serviceUrl": "not-a-url",
    "serviceToken": ""
  },
  "usageStatisticsEnabled": true,
  "routingStrategy": "round-robin"
}`)
	req := httptest.NewRequest(http.MethodPut, "/v0/management/config", body)
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, rr.Code, rr.Body.String())
	}
}
