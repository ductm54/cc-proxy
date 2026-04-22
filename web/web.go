package web

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var webEmbed embed.FS

func WebFS() (fs.FS, error) {
	return fs.Sub(webEmbed, "dist")
}
