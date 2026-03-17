package fileperm

import (
	"errors"
	"io/fs"
	"os"
	"syscall"
)

var chmodFunc = os.Chmod

// BestEffortChmod applies the requested mode and ignores permission errors
// returned by filesystems that disallow chmod on mounted paths.
func BestEffortChmod(path string, mode os.FileMode) error {
	err := chmodFunc(path, mode)
	if err == nil {
		return nil
	}
	if isIgnorablePermissionError(err) {
		return nil
	}
	return err
}

func isIgnorablePermissionError(err error) bool {
	if err == nil {
		return false
	}
	return os.IsPermission(err) ||
		errors.Is(err, fs.ErrPermission) ||
		errors.Is(err, syscall.EPERM) ||
		errors.Is(err, syscall.EACCES)
}

func swapChmodFuncForTest(fn func(string, os.FileMode) error) func() {
	previous := chmodFunc
	chmodFunc = fn
	return func() {
		chmodFunc = previous
	}
}
