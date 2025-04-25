package main

import (
	"embed"
	"io/fs"
)

//go:embed static
var staticFiles embed.FS

// GetStaticFS returns a filesystem with the static files
func GetStaticFS() (fs.FS, error) {
	return fs.Sub(staticFiles, "static")
}
