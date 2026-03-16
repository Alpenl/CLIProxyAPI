package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/buildinfo"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/cmd"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/logging"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/registry"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/usage"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/util"
	_ "github.com/router-for-me/CLIProxyAPI/v6/internal/translator"
	sdkAuth "github.com/router-for-me/CLIProxyAPI/v6/sdk/auth"
	coreauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
	log "github.com/sirupsen/logrus"
)

var (
	Version           = "dev"
	Commit            = "none"
	BuildDate         = "unknown"
	DefaultConfigPath = ""
)

func init() {
	logging.SetupBaseLogger()
	buildinfo.Version = Version
	buildinfo.Commit = Commit
	buildinfo.BuildDate = BuildDate
}

func main() {
	fmt.Printf("CLIProxyAPI Version: %s, Commit: %s, BuiltAt: %s\n", buildinfo.Version, buildinfo.Commit, buildinfo.BuildDate)

	var configPath string
	flag.StringVar(&configPath, "config", DefaultConfigPath, "配置文件路径")
	flag.Parse()

	resolvedConfigPath, err := resolveConfigPath(configPath)
	if err != nil {
		log.Fatalf("failed to resolve config path: %v", err)
	}

	cfg, err := config.LoadConfig(resolvedConfigPath)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	usage.SetStatisticsEnabled(cfg.UsageStatisticsEnabled)
	coreauth.SetQuotaCooldownDisabled(cfg.DisableCooling)

	if err = logging.ConfigureLogOutput(cfg); err != nil {
		log.Fatalf("failed to configure log output: %v", err)
	}
	util.SetLogLevel(cfg)

	authDir, err := util.ResolveAuthDir(cfg.AuthDir)
	if err != nil {
		log.Fatalf("failed to resolve auth directory: %v", err)
	}
	cfg.AuthDir = authDir

	// Register file-backed persistence for Codex auth JSON files.
	sdkAuth.RegisterTokenStore(sdkAuth.NewFileTokenStore())

	registry.StartModelsUpdater(context.Background())
	cmd.StartService(cfg, resolvedConfigPath, "")
}

func resolveConfigPath(raw string) (string, error) {
	if trimmed := filepath.Clean(raw); trimmed != "" && trimmed != "." {
		return filepath.Abs(trimmed)
	}
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return filepath.Abs(filepath.Join(wd, "config.yaml"))
}
