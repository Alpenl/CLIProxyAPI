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
		CodexKey: []config.CodexKey{
			{
				APIKey:         "old-key",
				BaseURL:        "https://old.example.com",
				ProxyURL:       "socks5://127.0.0.1:1080",
				Prefix:         "team-old",
				Websockets:     false,
				Headers:        map[string]string{"X-Old": "1"},
				Models:         []config.CodexModel{{Name: "gpt-5", Alias: "g5"}},
				ExcludedModels: []string{"gpt-5-mini"},
			},
		},
		RemoteManagement: config.RemoteManagement{
			AllowRemote:           false,
			SecretKey:             "",
			DisableControlPanel:   false,
			PanelGitHubRepository: "repo-old",
		},
	}

	newCfg := &config.Config{
		SDKConfig: config.SDKConfig{
			APIKeys: []string{"a", "b"},
		},
		Port:    9090,
		AuthDir: "/tmp/auth-new",
		CodexKey: []config.CodexKey{
			{
				APIKey:         "new-key",
				BaseURL:        "https://new.example.com",
				ProxyURL:       "socks5://127.0.0.1:2080",
				Prefix:         "team-new",
				Websockets:     true,
				Headers:        map[string]string{"X-New": "2"},
				Models:         []config.CodexModel{{Name: "gpt-5-latest", Alias: "g5"}},
				ExcludedModels: []string{"gpt-5-mini", "gpt-5-nano"},
			},
		},
		RemoteManagement: config.RemoteManagement{
			AllowRemote:           true,
			SecretKey:             "hashed-secret",
			DisableControlPanel:   true,
			PanelGitHubRepository: "repo-new",
		},
	}

	details := BuildConfigChangeDetails(oldCfg, newCfg)

	expectContains(t, details, "port: 8080 -> 9090")
	expectContains(t, details, "auth-dir: /tmp/auth-old -> /tmp/auth-new")
	expectContains(t, details, "api-keys count: 1 -> 2")
	expectContains(t, details, "codex[0].base-url: https://old.example.com -> https://new.example.com")
	expectContains(t, details, "codex[0].proxy-url: socks5://127.0.0.1:1080 -> socks5://127.0.0.1:2080")
	expectContains(t, details, "codex[0].prefix: team-old -> team-new")
	expectContains(t, details, "codex[0].websockets: false -> true")
	expectContains(t, details, "codex[0].api-key: updated")
	expectContains(t, details, "codex[0].headers: updated")
	expectContains(t, details, "codex[0].models: updated (1 -> 1 entries)")
	expectContains(t, details, "codex[0].excluded-models: updated (1 -> 2 entries)")
	expectContains(t, details, "remote-management.allow-remote: false -> true")
	expectContains(t, details, "remote-management.disable-control-panel: false -> true")
	expectContains(t, details, "remote-management.panel-github-repository: repo-old -> repo-new")
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
