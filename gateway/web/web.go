// Package web embeds the built Svelte SPA (produced by `make web` into web/dist).
package web

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var embedded embed.FS

// Dist returns the embedded build output rooted at dist/.
func Dist() fs.FS {
	sub, err := fs.Sub(embedded, "dist")
	if err != nil {
		panic(err) // dist/ is always embedded; unreachable
	}
	return sub
}
