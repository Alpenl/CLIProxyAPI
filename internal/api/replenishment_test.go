package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	gin "github.com/gin-gonic/gin"
	proxyconfig "github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	"github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
	sdkconfig "github.com/router-for-me/CLIProxyAPI/v6/sdk/config"
)

func TestServerReplenishmentCallbackURL_UsesClusterServiceHostWhenBindingAllInterfaces(t *testing.T) {
	t.Setenv("KUBERNETES_SERVICE_HOST", "10.64.0.1")
	t.Setenv("HOSTNAME", "cpa-0")
	t.Setenv("CPA_KYDWBNLIWVMB_SERVICE_HOST", "10.64.187.56")
	t.Setenv("CPA_KYDWBNLIWVMB_SERVICE_PORT", "8317")
	t.Setenv("RT_SAKEXOASRZNG_SERVICE_HOST", "10.64.5.7")
	t.Setenv("RT_SAKEXOASRZNG_SERVICE_PORT", "3080")

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
		Host:    "",
		Port:    8317,
		AuthDir: authDir,
		Debug:   true,
		RemoteManagement: proxyconfig.RemoteManagement{
			SecretKey: "test-management-key",
		},
		LoggingToFile:          false,
		UsageStatisticsEnabled: false,
	}

	server := NewServer(cfg, auth.NewManager(nil, nil, nil), filepath.Join(tmpDir, "config.yaml"))

	got := server.replenishmentCallbackURL()
	want := "http://10.64.187.56:8317/v0/internal/replenishment/accounts"
	if got != want {
		t.Fatalf("replenishmentCallbackURL() = %q, want %q", got, want)
	}
}

func TestServerReplenishmentCallbackURL_PrefersObservedRequestAddress(t *testing.T) {
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
		Host:    "",
		Port:    8317,
		AuthDir: authDir,
		Debug:   true,
		RemoteManagement: proxyconfig.RemoteManagement{
			SecretKey: "test-management-key",
		},
		LoggingToFile:          false,
		UsageStatisticsEnabled: false,
	}

	server := NewServer(cfg, auth.NewManager(nil, nil, nil), filepath.Join(tmpDir, "config.yaml"))

	req := httptest.NewRequest(http.MethodGet, "/management.html", nil)
	req.Host = "10.0.0.12:8317"
	req.Header.Set("X-Forwarded-Proto", "https")
	req.Header.Set("X-Forwarded-Host", "cpa.example.com")

	rr := httptest.NewRecorder()
	server.engine.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("management request status = %d, want %d", rr.Code, http.StatusOK)
	}

	got := server.replenishmentCallbackURL()
	want := "https://cpa.example.com/v0/internal/replenishment/accounts"
	if got != want {
		t.Fatalf("replenishmentCallbackURL() = %q, want %q", got, want)
	}
}
