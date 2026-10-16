//go:build !release

package agentbin

import "embed"

// A development build embeds whatever `just agent` built, possibly nothing,
// so tests and tools work on a fresh checkout.
//
//go:embed all:dist
var dist embed.FS
