package codex

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveTokenToFile_UsesReadablePermissions(t *testing.T) {
	t.Parallel()

	targetDir := filepath.Join(t.TempDir(), "auths")
	targetPath := filepath.Join(targetDir, "codex-demo.json")
	storage := &CodexTokenStorage{
		AccessToken:  "access-demo",
		RefreshToken: "refresh-demo",
		Email:        "demo@example.com",
	}

	if err := storage.SaveTokenToFile(targetPath); err != nil {
		t.Fatalf("SaveTokenToFile() error = %v", err)
	}

	dirInfo, err := os.Stat(targetDir)
	if err != nil {
		t.Fatalf("Stat(%s) error = %v", targetDir, err)
	}
	if got := dirInfo.Mode().Perm(); got != 0o755 {
		t.Fatalf("dir mode = %04o, want 0755", got)
	}

	fileInfo, err := os.Stat(targetPath)
	if err != nil {
		t.Fatalf("Stat(%s) error = %v", targetPath, err)
	}
	if got := fileInfo.Mode().Perm(); got != 0o644 {
		t.Fatalf("file mode = %04o, want 0644", got)
	}
}
