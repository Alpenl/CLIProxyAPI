package logging

import (
	"path/filepath"
	"testing"

	proxyconfig "github.com/router-for-me/CLIProxyAPI/v6/internal/config"
)

func TestResolveLogDirectoryForConfigPath_UsesConfigDirectory(t *testing.T) {
	t.Setenv("WRITABLE_PATH", "")
	t.Setenv("writable_path", "")

	configPath := filepath.Join(t.TempDir(), "data", "config.yaml")
	cfg := &proxyconfig.Config{
		AuthDir: filepath.Join(filepath.Dir(configPath), "auths"),
	}

	got := ResolveLogDirectoryForConfigPath(cfg, configPath)
	want := filepath.Join(filepath.Dir(configPath), "logs")
	if got != want {
		t.Fatalf("ResolveLogDirectoryForConfigPath() = %q, want %q", got, want)
	}
}
