package api

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	gin "github.com/gin-gonic/gin"
	proxyconfig "github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	internallogging "github.com/router-for-me/CLIProxyAPI/v6/internal/logging"
	"github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
	cliproxyexecutor "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/executor"
	sdkconfig "github.com/router-for-me/CLIProxyAPI/v6/sdk/config"
)

type serverTestExecutor struct {
	backendID string
}

func (e serverTestExecutor) Identifier() string { return e.backendID }

func (e serverTestExecutor) Execute(ctx context.Context, authRecord *auth.Auth, req cliproxyexecutor.Request, opts cliproxyexecutor.Options) (cliproxyexecutor.Response, error) {
	return cliproxyexecutor.Response{Payload: []byte(`{"ok":true}`)}, nil
}

func (e serverTestExecutor) ExecuteStream(ctx context.Context, authRecord *auth.Auth, req cliproxyexecutor.Request, opts cliproxyexecutor.Options) (*cliproxyexecutor.StreamResult, error) {
	return nil, nil
}

func (e serverTestExecutor) Refresh(ctx context.Context, authRecord *auth.Auth) (*auth.Auth, error) {
	return authRecord, nil
}

func (e serverTestExecutor) CountTokens(ctx context.Context, authRecord *auth.Auth, req cliproxyexecutor.Request, opts cliproxyexecutor.Options) (cliproxyexecutor.Response, error) {
	return cliproxyexecutor.Response{}, nil
}

func (e serverTestExecutor) HttpRequest(ctx context.Context, authRecord *auth.Auth, req *http.Request) (*http.Response, error) {
	return nil, nil
}

func newTestServer(t *testing.T) *Server {
	t.Helper()

	gin.SetMode(gin.TestMode)

	tmpDir := t.TempDir()
	authDir := filepath.Join(tmpDir, "auth")
	if err := os.MkdirAll(authDir, 0o700); err != nil {
		t.Fatalf("failed to create auth dir: %v", err)
	}

	cfg := &proxyconfig.Config{
		SDKConfig: sdkconfig.SDKConfig{
			APIKeys: []string{"test-key"},
		},
		Port:    0,
		AuthDir: authDir,
		Debug:   true,
		RemoteManagement: proxyconfig.RemoteManagement{
			SecretKey: "test-management-key",
		},
		LoggingToFile:          false,
		UsageStatisticsEnabled: false,
	}

	authManager := auth.NewManager(nil, nil, nil)

	configPath := filepath.Join(tmpDir, "config.yaml")
	return NewServer(cfg, authManager, configPath)
}

func newBootstrapTestServer(t *testing.T) *Server {
	t.Helper()

	gin.SetMode(gin.TestMode)

	tmpDir := t.TempDir()
	authDir := filepath.Join(tmpDir, "auth")
	if err := os.MkdirAll(authDir, 0o700); err != nil {
		t.Fatalf("failed to create auth dir: %v", err)
	}

	cfg := &proxyconfig.Config{
		Port:    0,
		AuthDir: authDir,
		Debug:   true,
		RemoteManagement: proxyconfig.RemoteManagement{
			AllowRemote: true,
			SecretKey:   "",
		},
		UsageStatisticsEnabled: true,
	}

	authManager := auth.NewManager(nil, nil, nil)
	configPath := filepath.Join(tmpDir, "config.yaml")
	return NewServer(cfg, authManager, configPath)
}

func TestServerRegistersOnlyCodexRoutes(t *testing.T) {
	testCases := []struct {
		name    string
		method  string
		path    string
		body    io.Reader
		headers map[string]string
		wantAny []int
	}{
		{
			name:    "models endpoint exists",
			method:  http.MethodGet,
			path:    "/v1/models",
			wantAny: []int{http.StatusUnauthorized, http.StatusOK},
		},
		{
			name:   "responses endpoint exists",
			method: http.MethodPost,
			path:   "/v1/responses",
			body:   bytes.NewBufferString(`{}`),
			headers: map[string]string{
				"Content-Type": "application/json",
			},
			wantAny: []int{http.StatusUnauthorized, http.StatusBadRequest, http.StatusOK},
		},
		{
			name:    "usage management endpoint exists",
			method:  http.MethodGet,
			path:    "/v0/management/usage",
			wantAny: []int{http.StatusForbidden, http.StatusUnauthorized, http.StatusOK},
		},
		{
			name:    "codex accounts endpoint exists",
			method:  http.MethodGet,
			path:    "/v0/management/codex/accounts",
			wantAny: []int{http.StatusForbidden, http.StatusUnauthorized, http.StatusOK},
		},
		{
			name:   "codex import directory endpoint exists",
			method: http.MethodPost,
			path:   "/v0/management/codex/import-directory",
			body:   bytes.NewBufferString(`{}`),
			headers: map[string]string{
				"Content-Type": "application/json",
			},
			wantAny: []int{http.StatusForbidden, http.StatusUnauthorized, http.StatusBadRequest, http.StatusOK},
		},
		{
			name:   "codex import files endpoint exists",
			method: http.MethodPost,
			path:   "/v0/management/codex/import-files",
			body:   bytes.NewBufferString(""),
			headers: map[string]string{
				"Content-Type": "multipart/form-data; boundary=test",
			},
			wantAny: []int{http.StatusForbidden, http.StatusUnauthorized, http.StatusBadRequest, http.StatusOK},
		},
		{
			name:   "codex cleanup endpoint exists",
			method: http.MethodPost,
			path:   "/v0/management/codex/cleanup-invalid",
			body:   bytes.NewBufferString(`{}`),
			headers: map[string]string{
				"Content-Type": "application/json",
			},
			wantAny: []int{http.StatusForbidden, http.StatusUnauthorized, http.StatusOK},
		},
		{
			name:    "codex delete endpoint exists",
			method:  http.MethodDelete,
			path:    "/v0/management/codex/accounts/example.json",
			wantAny: []int{http.StatusForbidden, http.StatusUnauthorized, http.StatusBadRequest, http.StatusOK},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			server := newTestServer(t)

			req := httptest.NewRequest(tc.method, tc.path, tc.body)
			for key, value := range tc.headers {
				req.Header.Set(key, value)
			}

			rr := httptest.NewRecorder()
			server.engine.ServeHTTP(rr, req)

			for _, code := range tc.wantAny {
				if rr.Code == code {
					return
				}
			}
			t.Fatalf("unexpected status code for %s %s: got %d want one of %v; body=%s", tc.method, tc.path, rr.Code, tc.wantAny, rr.Body.String())
		})
	}
}

func TestServerRegistersBootstrapRoutesWithoutManagementSecret(t *testing.T) {
	server := newBootstrapTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/v0/management/bootstrap/status", nil)
	rr := httptest.NewRecorder()
	server.engine.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected bootstrap status endpoint to return %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
	}
}

func TestServerRunReplenishmentOnce_ImportsCompletedArchive(t *testing.T) {
	authArchive := buildCodexArchive(t, "codex-topup.json", `{"type":"codex","email":"topup@example.com","refresh_token":"refresh-topup","access_token":"access-topup"}`)

	jobStatus := "queued"
	createCalls := 0
	service := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-Management-Secret"); got != "service-secret" {
			t.Fatalf("X-Management-Secret = %q, want service-secret", got)
		}

		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/jobs":
			createCalls++
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"job":{"id":"job-1","status":"queued","requestedSuccesses":1}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/jobs/job-1":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"job":{"id":"job-1","status":"` + jobStatus + `","requestedSuccesses":1}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/jobs/job-1/archive":
			w.Header().Set("Content-Type", "application/zip")
			_, _ = w.Write(authArchive)
		default:
			http.NotFound(w, r)
		}
	}))
	defer service.Close()

	gin.SetMode(gin.TestMode)

	tmpDir := t.TempDir()
	authDir := filepath.Join(tmpDir, "auth")
	if err := os.MkdirAll(authDir, 0o700); err != nil {
		t.Fatalf("failed to create auth dir: %v", err)
	}

	cfg := &proxyconfig.Config{
		SDKConfig: sdkconfig.SDKConfig{
			APIKeys: []string{"test-key"},
		},
		Port:    0,
		AuthDir: authDir,
		Debug:   true,
		RemoteManagement: proxyconfig.RemoteManagement{
			SecretKey: "test-management-key",
		},
		LoggingToFile:          false,
		UsageStatisticsEnabled: false,
		Replenishment: proxyconfig.ReplenishmentConfig{
			Enabled:            true,
			TargetAccountCount: 1,
			ServiceURL:         service.URL,
			ServiceToken:       "service-secret",
		},
	}

	server := NewServer(cfg, auth.NewManager(nil, nil, nil), filepath.Join(tmpDir, "config.yaml"))

	if err := server.runReplenishmentOnce(context.Background()); err != nil {
		t.Fatalf("first runReplenishmentOnce() error = %v", err)
	}
	if createCalls != 1 {
		t.Fatalf("createCalls = %d, want 1", createCalls)
	}
	if got := len(server.handlers.AuthManager.List()); got != 0 {
		t.Fatalf("auth count after first run = %d, want 0", got)
	}

	jobStatus = "completed"
	if err := server.runReplenishmentOnce(context.Background()); err != nil {
		t.Fatalf("second runReplenishmentOnce() error = %v", err)
	}

	auths := server.handlers.AuthManager.List()
	if len(auths) != 1 {
		t.Fatalf("auth count after import = %d, want 1", len(auths))
	}
	if got := auths[0].FileName; got != "codex-topup.json" {
		t.Fatalf("auth file name = %q, want codex-topup.json", got)
	}
	if _, err := os.Stat(filepath.Join(authDir, "codex-topup.json")); err != nil {
		t.Fatalf("expected imported auth file: %v", err)
	}
}

func TestServerTriggerReplenishment_RunsWhenAutoDisabled(t *testing.T) {
	createCalls := 0
	service := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/jobs":
			createCalls++
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"job":{"id":"job-1","status":"queued","requestedSuccesses":2}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/jobs/job-1":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"job":{"id":"job-1","status":"queued","requestedSuccesses":2}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer service.Close()

	gin.SetMode(gin.TestMode)

	tmpDir := t.TempDir()
	authDir := filepath.Join(tmpDir, "auth")
	if err := os.MkdirAll(authDir, 0o700); err != nil {
		t.Fatalf("failed to create auth dir: %v", err)
	}

	cfg := &proxyconfig.Config{
		SDKConfig: sdkconfig.SDKConfig{
			APIKeys: []string{"test-key"},
		},
		Port:    0,
		AuthDir: authDir,
		Debug:   true,
		RemoteManagement: proxyconfig.RemoteManagement{
			SecretKey: "test-management-key",
		},
		LoggingToFile:          false,
		UsageStatisticsEnabled: false,
		Replenishment: proxyconfig.ReplenishmentConfig{
			Enabled:            false,
			TargetAccountCount: 2,
			ServiceURL:         service.URL,
			ServiceToken:       "service-secret",
		},
	}

	server := NewServer(cfg, auth.NewManager(nil, nil, nil), filepath.Join(tmpDir, "config.yaml"))

	if _, err := server.triggerReplenishment(context.Background()); err != nil {
		t.Fatalf("triggerReplenishment() error = %v", err)
	}
	if createCalls != 1 {
		t.Fatalf("createCalls = %d, want 1", createCalls)
	}
}

func TestServerRunReplenishmentOnce_ImportsStreamedAccountWithoutCreatingANewJob(t *testing.T) {
	type callbackPayload struct {
		URL   string `json:"url"`
		Token string `json:"token"`
	}
	type createJobPayload struct {
		RequestedSuccesses int              `json:"requestedSuccesses"`
		Source             string           `json:"source"`
		ZipRequired        bool             `json:"zipRequired"`
		Callback           *callbackPayload `json:"callback"`
	}

	var capturedCreate createJobPayload
	jobSuccessCount := 0
	createCalls := 0
	service := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-Management-Secret"); got != "service-secret" {
			t.Fatalf("X-Management-Secret = %q, want service-secret", got)
		}

		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/jobs":
			createCalls++
			if err := json.NewDecoder(r.Body).Decode(&capturedCreate); err != nil {
				t.Fatalf("failed to decode create payload: %v", err)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"job":{"id":"job-1","status":"queued","requestedSuccesses":2}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/jobs/job-1":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"job":{"id":"job-1","status":"running","requestedSuccesses":2,"successCount":` + strconv.Itoa(jobSuccessCount) + `}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/jobs/job-1/archive":
			t.Fatalf("archive download should not happen while job is still running")
		default:
			http.NotFound(w, r)
		}
	}))
	defer service.Close()

	gin.SetMode(gin.TestMode)

	tmpDir := t.TempDir()
	authDir := filepath.Join(tmpDir, "auth")
	if err := os.MkdirAll(authDir, 0o700); err != nil {
		t.Fatalf("failed to create auth dir: %v", err)
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to allocate callback listener: %v", err)
	}
	callbackPort := listener.Addr().(*net.TCPAddr).Port

	cfg := &proxyconfig.Config{
		SDKConfig: sdkconfig.SDKConfig{
			APIKeys: []string{"test-key"},
		},
		Host:    "127.0.0.1",
		Port:    callbackPort,
		AuthDir: authDir,
		Debug:   true,
		RemoteManagement: proxyconfig.RemoteManagement{
			SecretKey: "test-management-key",
		},
		LoggingToFile:          false,
		UsageStatisticsEnabled: false,
		Replenishment: proxyconfig.ReplenishmentConfig{
			Enabled:            true,
			TargetAccountCount: 2,
			ServiceURL:         service.URL,
			ServiceToken:       "service-secret",
		},
	}

	server := NewServer(cfg, auth.NewManager(nil, nil, nil), filepath.Join(tmpDir, "config.yaml"))
	server.handlers.AuthManager.RegisterExecutor(serverTestExecutor{backendID: "codex"})
	serveDone := make(chan error, 1)
	go func() {
		serveDone <- server.server.Serve(listener)
	}()
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = server.server.Shutdown(shutdownCtx)
		if errServe := <-serveDone; errServe != nil && !errors.Is(errServe, http.ErrServerClosed) {
			t.Errorf("server shutdown error = %v", errServe)
		}
	}()

	if err := server.runReplenishmentOnce(context.Background()); err != nil {
		t.Fatalf("first runReplenishmentOnce() error = %v", err)
	}
	if createCalls != 1 {
		t.Fatalf("createCalls after first run = %d, want 1", createCalls)
	}
	if capturedCreate.Callback == nil {
		t.Fatalf("callback payload = nil, want value")
	}
	if capturedCreate.Callback.Token != "service-secret" {
		t.Fatalf("callback token = %q, want service-secret", capturedCreate.Callback.Token)
	}
	if capturedCreate.Callback.URL != "http://127.0.0.1:"+strconv.Itoa(callbackPort)+"/v0/internal/replenishment/accounts" {
		t.Fatalf("callback URL = %q, want local callback endpoint", capturedCreate.Callback.URL)
	}

	callbackBody, err := json.Marshal(map[string]any{
		"jobId":    "job-1",
		"fileName": "codex-streamed.json",
		"account": map[string]any{
			"type":          "codex",
			"email":         "streamed@example.com",
			"refresh_token": "refresh-streamed",
			"access_token":  "access-streamed",
		},
	})
	if err != nil {
		t.Fatalf("failed to encode callback payload: %v", err)
	}

	req, err := http.NewRequest(http.MethodPost, capturedCreate.Callback.URL, bytes.NewReader(callbackBody))
	if err != nil {
		t.Fatalf("failed to create callback request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+capturedCreate.Callback.Token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("callback request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("callback status = %d, want 200, body=%s", resp.StatusCode, string(body))
	}

	auths := server.handlers.AuthManager.List()
	if len(auths) != 1 {
		t.Fatalf("auth count after callback = %d, want 1", len(auths))
	}
	if got := auths[0].FileName; got != "codex-streamed.json" {
		t.Fatalf("auth file name = %q, want codex-streamed.json", got)
	}
	if _, err = server.handlers.AuthManager.Execute(context.Background(), cliproxyexecutor.Request{
		Model: "gpt-5-codex-mini",
	}, cliproxyexecutor.Options{}); err != nil {
		t.Fatalf("execute after callback import error = %v, want nil", err)
	}

	jobSuccessCount = 1
	if err = server.runReplenishmentOnce(context.Background()); err != nil {
		t.Fatalf("second runReplenishmentOnce() error = %v", err)
	}
	if createCalls != 1 {
		t.Fatalf("createCalls after callback import = %d, want still 1", createCalls)
	}

	status, err := server.replenishmentStatus(context.Background())
	if err != nil {
		t.Fatalf("replenishmentStatus() error = %v", err)
	}
	if status.CurrentJob == nil {
		t.Fatalf("current job = nil, want running job")
	}
	if status.CurrentJob.SuccessCount != 1 {
		t.Fatalf("current job successCount = %d, want 1", status.CurrentJob.SuccessCount)
	}
	if status.Deficit != 0 {
		t.Fatalf("deficit = %d, want 0 after streamed import and reserved success", status.Deficit)
	}
}

func TestServerRunReplenishmentOnce_SkipsArchiveDownloadAfterStreamedImportWhenJobCompletes(t *testing.T) {
	type callbackPayload struct {
		URL   string `json:"url"`
		Token string `json:"token"`
	}
	type createJobPayload struct {
		RequestedSuccesses int              `json:"requestedSuccesses"`
		Source             string           `json:"source"`
		ZipRequired        bool             `json:"zipRequired"`
		Callback           *callbackPayload `json:"callback"`
	}

	var capturedCreate createJobPayload
	jobStatus := "running"
	jobSuccessCount := 0
	createCalls := 0
	service := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/jobs":
			createCalls++
			if err := json.NewDecoder(r.Body).Decode(&capturedCreate); err != nil {
				t.Fatalf("failed to decode create payload: %v", err)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"job":{"id":"job-1","status":"queued","requestedSuccesses":1}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/jobs/job-1":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"job":{"id":"job-1","status":"` + jobStatus + `","requestedSuccesses":1,"successCount":` + strconv.Itoa(jobSuccessCount) + `}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/jobs/job-1/archive":
			t.Fatalf("archive download should be skipped after streamed import already delivered all successful accounts")
		default:
			http.NotFound(w, r)
		}
	}))
	defer service.Close()

	gin.SetMode(gin.TestMode)

	tmpDir := t.TempDir()
	authDir := filepath.Join(tmpDir, "auth")
	if err := os.MkdirAll(authDir, 0o700); err != nil {
		t.Fatalf("failed to create auth dir: %v", err)
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to allocate callback listener: %v", err)
	}
	callbackPort := listener.Addr().(*net.TCPAddr).Port

	cfg := &proxyconfig.Config{
		SDKConfig: sdkconfig.SDKConfig{
			APIKeys: []string{"test-key"},
		},
		Host:    "127.0.0.1",
		Port:    callbackPort,
		AuthDir: authDir,
		Debug:   true,
		RemoteManagement: proxyconfig.RemoteManagement{
			SecretKey: "test-management-key",
		},
		LoggingToFile:          false,
		UsageStatisticsEnabled: false,
		Replenishment: proxyconfig.ReplenishmentConfig{
			Enabled:            true,
			TargetAccountCount: 1,
			ServiceURL:         service.URL,
			ServiceToken:       "service-secret",
		},
	}

	server := NewServer(cfg, auth.NewManager(nil, nil, nil), filepath.Join(tmpDir, "config.yaml"))
	serveDone := make(chan error, 1)
	go func() {
		serveDone <- server.server.Serve(listener)
	}()
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = server.server.Shutdown(shutdownCtx)
		if errServe := <-serveDone; errServe != nil && !errors.Is(errServe, http.ErrServerClosed) {
			t.Errorf("server shutdown error = %v", errServe)
		}
	}()

	if err = server.runReplenishmentOnce(context.Background()); err != nil {
		t.Fatalf("first runReplenishmentOnce() error = %v", err)
	}
	if createCalls != 1 {
		t.Fatalf("createCalls after first run = %d, want 1", createCalls)
	}
	if capturedCreate.Callback == nil {
		t.Fatalf("callback payload = nil, want value")
	}

	callbackBody, err := json.Marshal(map[string]any{
		"jobId":    "job-1",
		"fileName": "codex-streamed-complete.json",
		"account": map[string]any{
			"type":          "codex",
			"email":         "streamed-complete@example.com",
			"refresh_token": "refresh-streamed-complete",
			"access_token":  "access-streamed-complete",
		},
	})
	if err != nil {
		t.Fatalf("failed to encode callback payload: %v", err)
	}

	req, err := http.NewRequest(http.MethodPost, capturedCreate.Callback.URL, bytes.NewReader(callbackBody))
	if err != nil {
		t.Fatalf("failed to create callback request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+capturedCreate.Callback.Token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("callback request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("callback status = %d, want 200, body=%s", resp.StatusCode, string(body))
	}

	jobStatus = "completed"
	jobSuccessCount = 1
	if err = server.runReplenishmentOnce(context.Background()); err != nil {
		t.Fatalf("second runReplenishmentOnce() error = %v", err)
	}

	auths := server.handlers.AuthManager.List()
	if len(auths) != 1 {
		t.Fatalf("auth count after callback and completion = %d, want 1", len(auths))
	}
}

func TestServerUpdateClients_StartsReplenishmentLoopWhenEnabledByReload(t *testing.T) {
	createCalls := 0
	service := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/jobs":
			createCalls++
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"job":{"id":"job-1","status":"queued","requestedSuccesses":1}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/jobs/job-1":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"job":{"id":"job-1","status":"queued","requestedSuccesses":1}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer service.Close()

	gin.SetMode(gin.TestMode)

	tmpDir := t.TempDir()
	authDir := filepath.Join(tmpDir, "auth")
	if err := os.MkdirAll(authDir, 0o700); err != nil {
		t.Fatalf("failed to create auth dir: %v", err)
	}

	cfg := &proxyconfig.Config{
		SDKConfig: sdkconfig.SDKConfig{
			APIKeys: []string{"test-key"},
		},
		Port:    0,
		AuthDir: authDir,
		Debug:   true,
		RemoteManagement: proxyconfig.RemoteManagement{
			SecretKey: "test-management-key",
		},
		LoggingToFile:          false,
		UsageStatisticsEnabled: false,
		Replenishment: proxyconfig.ReplenishmentConfig{
			Enabled:              false,
			TargetAccountCount:   1,
			CheckIntervalSeconds: 1,
			ServiceURL:           service.URL,
			ServiceToken:         "service-secret",
		},
	}

	server := NewServer(cfg, auth.NewManager(nil, nil, nil), filepath.Join(tmpDir, "config.yaml"))
	defer server.stopReplenishmentLoop()

	updated := *cfg
	updated.Replenishment.Enabled = true

	server.UpdateClients(&updated)

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if createCalls > 0 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}

	t.Fatalf("createCalls = %d, want replenishment loop to start and create a job after reload", createCalls)
}

func TestServerRemovesLegacyRoutes(t *testing.T) {
	testCases := []struct {
		name   string
		method string
		path   string
	}{
		{name: "legacy messages endpoint removed", method: http.MethodPost, path: "/v1/messages"},
		{name: "legacy token count endpoint removed", method: http.MethodPost, path: "/v1/messages/count_tokens"},
		{name: "legacy beta models endpoint removed", method: http.MethodGet, path: "/v1beta/models"},
		{name: "legacy routed models endpoint removed", method: http.MethodGet, path: "/api/provider/openai/models"},
		{name: "legacy oauth callback removed", method: http.MethodGet, path: "/anthropic/callback"},
		{name: "legacy management callback relay removed", method: http.MethodPost, path: "/v0/management/oauth-callback"},
		{name: "chat completions endpoint removed", method: http.MethodPost, path: "/v1/chat/completions"},
		{name: "completions endpoint removed", method: http.MethodPost, path: "/v1/completions"},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			server := newTestServer(t)
			req := httptest.NewRequest(tc.method, tc.path, nil)
			rr := httptest.NewRecorder()
			server.engine.ServeHTTP(rr, req)

			if rr.Code != http.StatusNotFound {
				t.Fatalf("expected %s %s to be removed with status %d, got %d: %s", tc.method, tc.path, http.StatusNotFound, rr.Code, rr.Body.String())
			}
		})
	}
}

func buildCodexArchive(t *testing.T, name string, payload string) []byte {
	t.Helper()

	var archive bytes.Buffer
	writer := zip.NewWriter(&archive)

	accountFile, err := writer.Create("accounts/" + name)
	if err != nil {
		t.Fatalf("failed to create account entry: %v", err)
	}
	if _, err = accountFile.Write([]byte(payload)); err != nil {
		t.Fatalf("failed to write account entry: %v", err)
	}
	if err = writer.Close(); err != nil {
		t.Fatalf("failed to close archive writer: %v", err)
	}
	return archive.Bytes()
}

func TestDefaultRequestLoggerFactory_UsesResolvedLogDirectory(t *testing.T) {
	t.Setenv("WRITABLE_PATH", "")
	t.Setenv("writable_path", "")

	originalWD, errGetwd := os.Getwd()
	if errGetwd != nil {
		t.Fatalf("failed to get current working directory: %v", errGetwd)
	}

	tmpDir := t.TempDir()
	if errChdir := os.Chdir(tmpDir); errChdir != nil {
		t.Fatalf("failed to switch working directory: %v", errChdir)
	}
	defer func() {
		if errChdirBack := os.Chdir(originalWD); errChdirBack != nil {
			t.Fatalf("failed to restore working directory: %v", errChdirBack)
		}
	}()

	// Force ResolveLogDirectory to fallback to auth-dir/logs by making ./logs not a writable directory.
	if errWriteFile := os.WriteFile(filepath.Join(tmpDir, "logs"), []byte("not-a-directory"), 0o644); errWriteFile != nil {
		t.Fatalf("failed to create blocking logs file: %v", errWriteFile)
	}

	configDir := filepath.Join(tmpDir, "config")
	if errMkdirConfig := os.MkdirAll(configDir, 0o755); errMkdirConfig != nil {
		t.Fatalf("failed to create config dir: %v", errMkdirConfig)
	}
	configPath := filepath.Join(configDir, "config.yaml")

	authDir := filepath.Join(tmpDir, "auth")
	if errMkdirAuth := os.MkdirAll(authDir, 0o700); errMkdirAuth != nil {
		t.Fatalf("failed to create auth dir: %v", errMkdirAuth)
	}

	cfg := &proxyconfig.Config{
		SDKConfig: proxyconfig.SDKConfig{
			RequestLog: false,
		},
		AuthDir:           authDir,
		ErrorLogsMaxFiles: 10,
	}

	logger := internallogging.NewFileRequestLogger(
		cfg.RequestLog,
		internallogging.ResolveLogDirectory(cfg),
		filepath.Dir(configPath),
		cfg.ErrorLogsMaxFiles,
	)
	fileLogger := logger

	errLog := fileLogger.LogRequestWithOptions(
		"/v1/responses",
		http.MethodPost,
		map[string][]string{"Content-Type": []string{"application/json"}},
		[]byte(`{"input":"hello"}`),
		http.StatusBadGateway,
		map[string][]string{"Content-Type": []string{"application/json"}},
		[]byte(`{"error":"upstream failure"}`),
		nil,
		nil,
		nil,
		true,
		"issue-1711",
		time.Now(),
		time.Now(),
	)
	if errLog != nil {
		t.Fatalf("failed to write forced error request log: %v", errLog)
	}

	authLogsDir := filepath.Join(authDir, "logs")
	authEntries, errReadAuthDir := os.ReadDir(authLogsDir)
	if errReadAuthDir != nil {
		t.Fatalf("failed to read auth logs dir %s: %v", authLogsDir, errReadAuthDir)
	}
	foundErrorLogInAuthDir := false
	for _, entry := range authEntries {
		if strings.HasPrefix(entry.Name(), "error-") && strings.HasSuffix(entry.Name(), ".log") {
			foundErrorLogInAuthDir = true
			break
		}
	}
	if !foundErrorLogInAuthDir {
		t.Fatalf("expected forced error log in auth fallback dir %s, got entries: %+v", authLogsDir, authEntries)
	}

	configLogsDir := filepath.Join(configDir, "logs")
	configEntries, errReadConfigDir := os.ReadDir(configLogsDir)
	if errReadConfigDir != nil && !os.IsNotExist(errReadConfigDir) {
		t.Fatalf("failed to inspect config logs dir %s: %v", configLogsDir, errReadConfigDir)
	}
	for _, entry := range configEntries {
		if strings.HasPrefix(entry.Name(), "error-") && strings.HasSuffix(entry.Name(), ".log") {
			t.Fatalf("unexpected forced error log in config dir %s", configLogsDir)
		}
	}
}

func TestManagementControlPanel_ServesEmbeddedCodexUI(t *testing.T) {
	server := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/management.html", nil)
	rr := httptest.NewRecorder()
	server.engine.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	if !strings.Contains(body, "Codex 控制台") {
		t.Fatalf("expected embedded Codex UI content, body=%s", body)
	}
	if !strings.Contains(body, "仪表盘") {
		t.Fatalf("expected dashboard navigation item, body=%s", body)
	}
	if !strings.Contains(body, "账号额度") {
		t.Fatalf("expected quota navigation item, body=%s", body)
	}
	if !strings.Contains(body, "账号导入") {
		t.Fatalf("expected import navigation item, body=%s", body)
	}
	if !strings.Contains(body, "操作日志") {
		t.Fatalf("expected logs navigation item, body=%s", body)
	}
	if !strings.Contains(body, "账号清单") {
		t.Fatalf("expected account list title, body=%s", body)
	}
	if !strings.Contains(body, "筛选账号") {
		t.Fatalf("expected account filter label, body=%s", body)
	}
	if !strings.Contains(body, "显示状态") {
		t.Fatalf("expected quota status filter label, body=%s", body)
	}
	if !strings.Contains(body, "选择文件") {
		t.Fatalf("expected custom file picker label, body=%s", body)
	}
	if !strings.Contains(body, "选择目录") {
		t.Fatalf("expected custom directory picker label, body=%s", body)
	}
	if !strings.Contains(body, "auth-shell") {
		t.Fatalf("expected auth shell structure, body=%s", body)
	}
	if !strings.Contains(body, "backend-shell") {
		t.Fatalf("expected backend shell structure, body=%s", body)
	}
	if !strings.Contains(body, "管理登录") {
		t.Fatalf("expected management login title, body=%s", body)
	}
	if !strings.Contains(body, "进入后台") {
		t.Fatalf("expected login submit label, body=%s", body)
	}
	if !strings.Contains(body, "page-header-grid") {
		t.Fatalf("expected stable page header grid structure, body=%s", body)
	}
	if !strings.Contains(body, "quota-toolbar-grid") {
		t.Fatalf("expected stable quota toolbar grid structure, body=%s", body)
	}
	if !strings.Contains(body, "overview-list") {
		t.Fatalf("expected stable overview list structure, body=%s", body)
	}
	if !strings.Contains(body, "align-items: start;") {
		t.Fatalf("expected app shell to avoid stretching sidebar height, body=%s", body)
	}
	if !strings.Contains(body, "height: fit-content;") {
		t.Fatalf("expected sidebar height to follow its own content instead of filling the viewport, body=%s", body)
	}
	if !strings.Contains(body, "max-height: calc(100vh - 24px);") {
		t.Fatalf("expected sidebar to remain capped by viewport height, body=%s", body)
	}
	if !strings.Contains(body, "quota-table") {
		t.Fatalf("expected stable quota table structure, body=%s", body)
	}
	if !strings.Contains(body, "actions-grid") {
		t.Fatalf("expected stable action grid structure, body=%s", body)
	}
	if !strings.Contains(body, "file-picker-grid") {
		t.Fatalf("expected stable file picker grid structure, body=%s", body)
	}
	if !strings.Contains(body, "align-items: stretch;") {
		t.Fatalf("expected full-height backend stretch layout, body=%s", body)
	}
	if !strings.Contains(body, "grid-template-rows: auto minmax(0, 1fr);") {
		t.Fatalf("expected main shell viewport row layout, body=%s", body)
	}
	if !strings.Contains(body, "elements.view.dataset.route = route;") {
		t.Fatalf("expected route-aware view sizing logic, body=%s", body)
	}
	if !strings.Contains(body, "scrollbar-gutter: stable both-edges;") {
		t.Fatalf("expected stable scrollbar gutter to prevent sidebar shifting, body=%s", body)
	}
	if !strings.Contains(body, "grid-auto-rows: max-content;") {
		t.Fatalf("expected dashboard rows to keep natural height, body=%s", body)
	}
	if !strings.Contains(body, "calc(100% - 20px)") {
		t.Fatalf("expected app shell width to avoid viewport scrollbar jitter, body=%s", body)
	}
	if !strings.Contains(body, "QUOTA_AUTO_REFRESH_INTERVAL_MS = 60 * 60 * 1000") {
		t.Fatalf("expected hourly quota auto refresh interval, body=%s", body)
	}
	if !strings.Contains(body, "refresh-quota-all") {
		t.Fatalf("expected bulk quota refresh action, body=%s", body)
	}
	if strings.Contains(body, "<th>授权方式</th>") {
		t.Fatalf("expected authorization column removed, body=%s", body)
	}
	if strings.Contains(body, "<th>登录状态</th>") {
		t.Fatalf("expected login status column removed, body=%s", body)
	}
	if strings.Contains(body, "<th>登录凭据</th>") {
		t.Fatalf("expected credential column removed, body=%s", body)
	}
	if !strings.Contains(body, "<th>套餐</th>") {
		t.Fatalf("expected plan column kept, body=%s", body)
	}
	if !strings.Contains(body, "<th>健康状态</th>") {
		t.Fatalf("expected health column kept, body=%s", body)
	}
	if !strings.Contains(body, "<th>额度</th>") {
		t.Fatalf("expected unified quota header, body=%s", body)
	}
	if !strings.Contains(body, "quota-inline-grid") {
		t.Fatalf("expected inline quota meter layout, body=%s", body)
	}
	if !strings.Contains(body, "remaining == null ? \"100%\"") {
		t.Fatalf("expected unused quota to render as 100 percent, body=%s", body)
	}
	if !strings.Contains(body, "plus/team 显示 5H + 7D，free 仅显示 7D") {
		t.Fatalf("expected plan-specific quota hint, body=%s", body)
	}
	if !strings.Contains(body, "quota-table-wrap") {
		t.Fatalf("expected dedicated quota table wrapper, body=%s", body)
	}
	if !strings.Contains(body, "overflow-x: hidden;") {
		t.Fatalf("expected horizontal scrolling disabled for quota table, body=%s", body)
	}
	if !strings.Contains(body, "width: 360px;") {
		t.Fatalf("expected tightened quota column width for single-screen layout, body=%s", body)
	}
	if !strings.Contains(body, ".quota-table th,") {
		t.Fatalf("expected quota table specific alignment rule, body=%s", body)
	}
	if !strings.Contains(body, "align-items: center;") {
		t.Fatalf("expected centered table content layout, body=%s", body)
	}
	if !strings.Contains(body, "class=\"quota-cell\"") {
		t.Fatalf("expected dedicated quota cell class, body=%s", body)
	}
	if !strings.Contains(body, "quota-meter-top") {
		t.Fatalf("expected horizontal quota meter header, body=%s", body)
	}
	if !strings.Contains(body, "grid-template-columns: auto minmax(0, 1fr) auto;") {
		t.Fatalf("expected horizontal quota meter layout, body=%s", body)
	}
	if !strings.Contains(body, "justify-items: stretch;") {
		t.Fatalf("expected quota cell content to stretch horizontally, body=%s", body)
	}
}

func TestRootRedirectsToManagementControlPanel(t *testing.T) {
	server := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	server.engine.ServeHTTP(rr, req)

	if rr.Code != http.StatusTemporaryRedirect {
		t.Fatalf("expected status %d, got %d: %s", http.StatusTemporaryRedirect, rr.Code, rr.Body.String())
	}
	if got := rr.Header().Get("Location"); got != "/management.html" {
		t.Fatalf("expected redirect location %q, got %q", "/management.html", got)
	}
}
