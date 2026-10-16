//go:build release

package ui

import "embed"

//go:embed dist/app/index.html all:dist/app
var dist embed.FS
