package misc

import (
	"io"
	"os"
	"path/filepath"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/fileperm"
	log "github.com/sirupsen/logrus"
)

func CopyConfigTemplate(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() {
		if errClose := in.Close(); errClose != nil {
			log.WithError(errClose).Warn("failed to close source config file")
		}
	}()

	dir := filepath.Dir(dst)
	if err = os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if err = fileperm.BestEffortChmod(dir, 0o755); err != nil {
		return err
	}

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer func() {
		if errClose := out.Close(); errClose != nil {
			log.WithError(errClose).Warn("failed to close destination config file")
		}
	}()

	if _, err = io.Copy(out, in); err != nil {
		return err
	}
	if err = out.Sync(); err != nil {
		return err
	}
	return fileperm.BestEffortChmod(dst, 0o644)
}
