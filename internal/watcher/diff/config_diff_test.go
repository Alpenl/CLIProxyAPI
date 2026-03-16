package diff

import (
	"strings"
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
)

func TestBuildConfigChangeDetails_CodexOnly(t *testing.T) {
	oldCfg := &config.Config{
		SDKConfig: config.SDKConfig{
			APIKeys: []string{"a"},
		},
		Port:    8080,
		AuthDir: "/tmp/auth-old",
		RemoteManagement: config.RemoteManagement{
			AllowRemote: false,
			SecretKey:   "",
		},
	}

	newCfg := &config.Config{
		SDKConfig: config.SDKConfig{
			APIKeys: []string{"a", "b"},
		},
		Port:    9090,
		AuthDir: "/tmp/auth-new",
		RemoteManagement: config.RemoteManagement{
			AllowRemote: true,
			SecretKey:   "hashed-secret",
		},
	}

	details := BuildConfigChangeDetails(oldCfg, newCfg)

	expectContains(t, details, "port: 8080 -> 9090")
	expectContains(t, details, "auth-dir: /tmp/auth-old -> /tmp/auth-new")
	expectContains(t, details, "api-keys count: 1 -> 2")
	expectContains(t, details, "remote-management.allow-remote: false -> true")
	expectContains(t, details, "remote-management.secret-key: created")
}

func TestBuildConfigChangeDetails_NoChanges(t *testing.T) {
	cfg := &config.Config{
		Port: 8080,
	}
	if details := BuildConfigChangeDetails(cfg, cfg); len(details) != 0 {
		t.Fatalf("expected no change entries, got %v", details)
	}
}

func TestBuildConfigChangeDetails_NilSafe(t *testing.T) {
	if details := BuildConfigChangeDetails(nil, &config.Config{}); len(details) != 0 {
		t.Fatalf("expected empty change list when old nil, got %v", details)
	}
	if details := BuildConfigChangeDetails(&config.Config{}, nil); len(details) != 0 {
		t.Fatalf("expected empty change list when new nil, got %v", details)
	}
}

func TestBuildConfigChangeDetails_SecretTransitions(t *testing.T) {
	oldCfg := &config.Config{
		RemoteManagement: config.RemoteManagement{SecretKey: "old"},
	}
	newCfg := &config.Config{
		RemoteManagement: config.RemoteManagement{SecretKey: ""},
	}
	details := BuildConfigChangeDetails(oldCfg, newCfg)
	expectContains(t, details, "remote-management.secret-key: deleted")
}

func expectContains(t *testing.T, list []string, want string) {
	t.Helper()
	for _, entry := range list {
		if strings.Contains(entry, want) {
			return
		}
	}
	t.Fatalf("expected %q in %v", want, list)
}
