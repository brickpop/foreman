//go:build release

package agentbin

import "embed"

// A release build (`just build`) doesn't compile without both CPU types, so it
// never ships without the team tools.
//
//go:embed dist/foreman-agent-amd64 dist/foreman-agent-arm64
var dist embed.FS
