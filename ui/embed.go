// Package ui embeds the built web UI (ui/dist/app, produced by `just build`).
package ui

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var dist embed.FS

// UI returns the built app, with index.html at its root. Before the first
// build it holds only a placeholder, and the server says how to build it.
func UI() fs.FS {
	sub, err := fs.Sub(dist, "dist/app")
	if err != nil {
		panic(err) // only fails on an invalid path, and this one is a valid literal
	}
	return sub
}
