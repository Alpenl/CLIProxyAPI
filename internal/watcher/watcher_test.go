package watcher

import (
	"crypto/sha256"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	coreauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
)

func TestNormalizeAuthStripsTemporalFields(t *testing.T) {
	now := time.Now()
	auth := &coreauth.Auth{
		CreatedAt:        now,
		UpdatedAt:        now,
		LastRefreshedAt:  now,
		NextRefreshAfter: now,
		Quota: coreauth.QuotaState{
			NextRecoverAt: now,
		},
		Runtime: map[string]any{"k": "v"},
	}

	normalized := normalizeAuth(auth)
	if !normalized.CreatedAt.IsZero() || !normalized.UpdatedAt.IsZero() || !normalized.LastRefreshedAt.IsZero() || !normalized.NextRefreshAfter.IsZero() {
		t.Fatal("expected time fields to be zeroed")
	}
	if normalized.Runtime != nil {
		t.Fatal("expected runtime to be nil")
	}
	if !normalized.Quota.NextRecoverAt.IsZero() {
		t.Fatal("expected quota.NextRecoverAt to be zeroed")
	}
}

func TestSnapshotCoreAuths_UsesAuthFilesOnly(t *testing.T) {
	authDir := t.TempDir()
	fileData, err := json.Marshal(map[string]any{
		"type":  "codex",
		"email": "user@example.com",
	})
	if err != nil {
		t.Fatalf("failed to marshal auth file: %v", err)
	}
	authFile := filepath.Join(authDir, "codex.json")
	if err := os.WriteFile(authFile, fileData, 0o644); err != nil {
		t.Fatalf("failed to write auth file: %v", err)
	}

	w := &Watcher{authDir: authDir}
	w.SetConfig(&config.Config{
		AuthDir: authDir,
	})

	auths := w.SnapshotCoreAuths()
	if len(auths) != 1 {
		t.Fatalf("expected 1 auth entry from file store, got %d", len(auths))
	}

	seenFile := false
	for _, auth := range auths {
		if auth == nil {
			continue
		}
		if auth.Provider != "codex" {
			t.Fatalf("expected only auth type codex, got %s", auth.Provider)
		}
		if auth.Attributes["path"] == authFile {
			seenFile = true
		}
	}
	if !seenFile {
		t.Fatal("expected file-backed codex auth")
	}
}

func TestAddOrUpdateClientCachesHashForCodexFile(t *testing.T) {
	authDir := t.TempDir()
	filePath := filepath.Join(authDir, "codex.json")
	data, err := json.Marshal(map[string]any{
		"type":  "codex",
		"email": "hash@example.com",
	})
	if err != nil {
		t.Fatalf("failed to marshal auth file: %v", err)
	}
	if err := os.WriteFile(filePath, data, 0o644); err != nil {
		t.Fatalf("failed to write auth file: %v", err)
	}

	w := &Watcher{
		authDir:          authDir,
		config:           &config.Config{AuthDir: authDir},
		lastAuthHashes:   make(map[string]string),
		lastAuthContents: make(map[string]*coreauth.Auth),
		fileAuthsByPath:  make(map[string]map[string]*coreauth.Auth),
	}

	w.addOrUpdateClient(filePath)

	normalized := w.normalizeAuthPath(filePath)
	if got := w.lastAuthHashes[normalized]; got == "" {
		t.Fatal("expected auth hash cache entry")
	}
	sum := sha256.Sum256(data)
	expected := formatHash(sum[:])
	if w.lastAuthHashes[normalized] != expected {
		t.Fatalf("expected hash %s, got %s", expected, w.lastAuthHashes[normalized])
	}
	if len(w.fileAuthsByPath[normalized]) != 1 {
		t.Fatalf("expected 1 synthesized auth for path, got %d", len(w.fileAuthsByPath[normalized]))
	}
}

func formatHash(sum []byte) string {
	const hex = "0123456789abcdef"
	out := make([]byte, len(sum)*2)
	for i, b := range sum {
		out[i*2] = hex[b>>4]
		out[i*2+1] = hex[b&0x0f]
	}
	return string(out)
}
