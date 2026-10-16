//go:build !release

package ui

import "embed"

//go:embed all:dist
var dist embed.FS
