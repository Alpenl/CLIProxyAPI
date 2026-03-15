package api

import (
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
	sdkaccess "github.com/router-for-me/CLIProxyAPI/v6/sdk/access"
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
		Port:                   0,
		AuthDir:                authDir,
		Debug:                  true,
		LoggingToFile:          false,
		UsageStatisticsEnabled: false,
	}

	authManager := auth.NewManager(nil, nil, nil)
	accessManager := sdkaccess.NewManager()

	configPath := filepath.Join(tmpDir, "config.yaml")
	return NewServer(cfg, authManager, accessManager, configPath)
}

func TestAmpProviderModelRoutes(t *testing.T) {
	testCases := []struct {
		name         string
		path         string
		wantStatus   int
		wantContains string
	}{
		{
			name:         "openai root models",
			path:         "/api/provider/openai/models",
			wantStatus:   http.StatusOK,
			wantContains: `"object":"list"`,
		},
		{
			name:         "groq root models",
			path:         "/api/provider/groq/models",
			wantStatus:   http.StatusOK,
			wantContains: `"object":"list"`,
		},
		{
			name:         "openai models",
			path:         "/api/provider/openai/v1/models",
			wantStatus:   http.StatusOK,
			wantContains: `"object":"list"`,
		},
		{
			name:         "anthropic models",
			path:         "/api/provider/anthropic/v1/models",
			wantStatus:   http.StatusOK,
			wantContains: `"data"`,
		},
		{
			name:         "google models v1",
			path:         "/api/provider/google/v1/models",
			wantStatus:   http.StatusOK,
			wantContains: `"models"`,
		},
		{
			name:         "google models v1beta",
			path:         "/api/provider/google/v1beta/models",
			wantStatus:   http.StatusOK,
			wantContains: `"models"`,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			server := newTestServer(t)

			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			req.Header.Set("Authorization", "Bearer test-key")

			rr := httptest.NewRecorder()
			server.engine.ServeHTTP(rr, req)

			if rr.Code != tc.wantStatus {
				t.Fatalf("unexpected status code for %s: got %d want %d; body=%s", tc.path, rr.Code, tc.wantStatus, rr.Body.String())
			}
			if body := rr.Body.String(); !strings.Contains(body, tc.wantContains) {
				t.Fatalf("response body for %s missing %q: %s", tc.path, tc.wantContains, body)
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

	logger := defaultRequestLoggerFactory(cfg, configPath)
	fileLogger, ok := logger.(*internallogging.FileRequestLogger)
	if !ok {
		t.Fatalf("expected *FileRequestLogger, got %T", logger)
	}

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
