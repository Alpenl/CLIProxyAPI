package fileperm

import (
	"errors"
	"os"
	"syscall"
	"testing"
)

func TestBestEffortChmod_IgnoresPermissionDenied(t *testing.T) {
	restore := swapChmodFuncForTest(func(path string, mode os.FileMode) error {
		return &os.PathError{Op: "chmod", Path: path, Err: syscall.EPERM}
	})
	defer restore()

	if err := BestEffortChmod("/data", 0o755); err != nil {
		t.Fatalf("BestEffortChmod() error = %v, want nil", err)
	}
}

func TestBestEffortChmod_ReturnsOtherErrors(t *testing.T) {
	wantErr := errors.New("boom")
	restore := swapChmodFuncForTest(func(path string, mode os.FileMode) error {
		return wantErr
	})
	defer restore()

	err := BestEffortChmod("/data/config.yaml", 0o644)
	if !errors.Is(err, wantErr) {
		t.Fatalf("BestEffortChmod() error = %v, want %v", err, wantErr)
	}
}
