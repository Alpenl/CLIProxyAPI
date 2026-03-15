package managementui

import (
	"embed"
	"io/fs"
)

//go:embed assets/*
var assets embed.FS

// HTML returns the embedded management UI entry page.
func HTML() ([]byte, error) {
	web, err := fs.Sub(assets, "assets")
	if err != nil {
		return nil, err
	}
	return fs.ReadFile(web, "index.html")
}
