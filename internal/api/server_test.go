package api

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	gin "github.com/gin-gonic/gin"
	proxyconfig "github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	internallogging "github.com/router-for-me/CLIProxyAPI/v6/internal/logging"
	"github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
	sdkconfig "github.com/router-for-me/CLIProxyAPI/v6/sdk/config"
)

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

func TestServerRegistersOnlyCodexRoutes(t *testing.T) {
	testCases := []struct {
		name   string
		method string
		path   string
		body   io.Reader
		headers map[string]string
		wantAny []int
	}{
		{
			name:   "models endpoint exists",
			method: http.MethodGet,
			path:   "/v1/models",
			wantAny: []int{http.StatusUnauthorized, http.StatusOK},
		},
		{
			name:   "chat completions endpoint exists",
			method: http.MethodPost,
			path:   "/v1/chat/completions",
			body:   bytes.NewBufferString(`{}`),
			headers: map[string]string{
				"Content-Type": "application/json",
			},
			wantAny: []int{http.StatusUnauthorized, http.StatusBadRequest, http.StatusOK},
		},
		{
			name:   "completions endpoint exists",
			method: http.MethodPost,
			path:   "/v1/completions",
			body:   bytes.NewBufferString(`{}`),
			headers: map[string]string{
				"Content-Type": "application/json",
			},
			wantAny: []int{http.StatusUnauthorized, http.StatusBadRequest, http.StatusOK},
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
			name:   "usage management endpoint exists",
			method: http.MethodGet,
			path:   "/v0/management/usage",
			wantAny: []int{http.StatusForbidden, http.StatusUnauthorized, http.StatusOK},
		},
		{
			name:   "codex accounts endpoint exists",
			method: http.MethodGet,
			path:   "/v0/management/codex/accounts",
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
			name:   "codex delete endpoint exists",
			method: http.MethodDelete,
			path:   "/v0/management/codex/accounts/example.json",
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
		"/v1/chat/completions",
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
