// Package ui embeds the built web UI (ui/dist/app, produced by `just build`).
package ui

import "io/fs"

// UI returns the built app, with index.html at its root. A development build
// before the first UI build holds only a placeholder, and the server says how
// to build it; a release build doesn't compile without the app.
func UI() fs.FS {
	sub, err := fs.Sub(dist, "dist/app")
	if err != nil {
		panic(err) // only fails on an invalid path, and this one is a valid literal
	}
	return sub
}
