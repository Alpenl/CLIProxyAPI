package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// LoadOrCreateConfig loads the configuration file when it exists.
// If the file is missing or empty, it writes a bootstrap configuration first.
func LoadOrCreateConfig(configFile string) (*Config, bool, error) {
	if strings.TrimSpace(configFile) == "" {
		return nil, false, fmt.Errorf("config file path is required")
	}

	info, err := os.Stat(configFile)
	switch {
	case err == nil && info.IsDir():
		return nil, false, fmt.Errorf("config path %s is a directory", configFile)
	case err == nil && info.Size() > 0:
		cfg, errLoad := LoadConfig(configFile)
		return cfg, false, errLoad
	case err == nil && info.Size() == 0:
		// empty file: overwrite with bootstrap defaults
	case os.IsNotExist(err):
		// missing file: create bootstrap defaults
	case err != nil:
		return nil, false, fmt.Errorf("stat config file: %w", err)
	}

	cfg := DefaultBootstrapConfig()
	cfg.AuthDir = defaultBootstrapAuthDir(configFile)
	if errWrite := WriteConfigFile(configFile, cfg); errWrite != nil {
		return nil, false, errWrite
	}
	loaded, errLoad := LoadConfig(configFile)
	if errLoad != nil {
		return nil, false, errLoad
	}
	return loaded, true, nil
}

// DefaultBootstrapConfig returns the generated first-run configuration used when
// the server starts without an existing config file.
func DefaultBootstrapConfig() *Config {
	return &Config{
		SDKConfig: SDKConfig{
			ProxyURL:                   "",
			APIKeys:                    []string{},
			RequestLog:                 false,
			PassthroughHeaders:         false,
			NonStreamKeepAliveInterval: 0,
		},
		Host:                   "",
		Port:                   8317,
		TLS:                    TLSConfig{},
		RemoteManagement:       RemoteManagement{AllowRemote: true, SecretKey: "", DisableControlPanel: false},
		AuthDir:                "./auths",
		Debug:                  false,
		LoggingToFile:          false,
		LogsMaxTotalSizeMB:     0,
		ErrorLogsMaxFiles:      10,
		UsageStatisticsEnabled: true,
		DisableCooling:         false,
		RequestRetry:           3,
		MaxRetryCredentials:    0,
		MaxRetryInterval:       30,
		Routing:                RoutingConfig{Strategy: "round-robin"},
		WebsocketAuth:          false,
	}
}

// BootstrapRequired reports whether the minimum first-run configuration is still missing.
func (cfg *Config) BootstrapRequired() bool {
	if cfg == nil {
		return true
	}
	if strings.TrimSpace(cfg.RemoteManagement.SecretKey) == "" {
		return true
	}
	if len(normalizeNonEmptyStrings(cfg.APIKeys)) == 0 {
		return true
	}
	if strings.TrimSpace(cfg.AuthDir) == "" {
		return true
	}
	return false
}

// WriteConfigFile writes a complete config file without requiring an existing YAML tree.
func WriteConfigFile(configFile string, cfg *Config) error {
	if cfg == nil {
		return fmt.Errorf("config is nil")
	}
	dir := filepath.Dir(configFile)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create config directory: %w", err)
		}
	}

	data := []byte(renderBootstrapConfigYAML(cfg))
	data = NormalizeCommentIndentation(data)
	if err := os.WriteFile(configFile, data, 0o600); err != nil {
		return fmt.Errorf("write config file: %w", err)
	}
	return nil
}

func normalizeNonEmptyStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, raw := range values {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func defaultBootstrapAuthDir(configFile string) string {
	baseDir := filepath.Dir(configFile)
	if strings.TrimSpace(baseDir) == "" || baseDir == "." {
		return "./auths"
	}
	return filepath.Clean(filepath.Join(baseDir, "auths"))
}

func renderBootstrapConfigYAML(cfg *Config) string {
	if cfg == nil {
		cfg = DefaultBootstrapConfig()
	}
	return fmt.Sprintf(`# Codex-first bootstrap configuration.
# This file was created automatically because no config.yaml existed at startup.
# Complete first-time setup in /management.html after the service starts.

host: %q
port: %d

tls:
  enable: %t
  cert: %q
  key: %q

remote-management:
  allow-remote: %t
  secret-key: %q

auth-dir: %q
api-keys: []

debug: %t
logging-to-file: %t
logs-max-total-size-mb: %d
error-logs-max-files: %d
usage-statistics-enabled: %t
proxy-url: %q
request-log: %t
disable-cooling: %t
request-retry: %d
max-retry-credentials: %d
max-retry-interval: %d

routing:
  strategy: %q
`,
		cfg.Host,
		cfg.Port,
		cfg.TLS.Enable,
		cfg.TLS.Cert,
		cfg.TLS.Key,
		cfg.RemoteManagement.AllowRemote,
		cfg.RemoteManagement.SecretKey,
		cfg.AuthDir,
		cfg.Debug,
		cfg.LoggingToFile,
		cfg.LogsMaxTotalSizeMB,
		cfg.ErrorLogsMaxFiles,
		cfg.UsageStatisticsEnabled,
		cfg.ProxyURL,
		cfg.RequestLog,
		cfg.DisableCooling,
		cfg.RequestRetry,
		cfg.MaxRetryCredentials,
		cfg.MaxRetryInterval,
		normalizeRoutingStrategyValue(cfg.Routing.Strategy),
	)
}

func normalizeRoutingStrategyValue(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "fill-first", "fillfirst", "ff":
		return "fill-first"
	default:
		return "round-robin"
	}
}
