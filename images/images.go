// Package images holds the Containerfiles Foreman builds its agent images
// from, embedded in the binary (wiki/product/architecture/tech-stack.md#agent-images).
package images

import _ "embed"

// Harness is the phase 1 image: one image for every project, with every
// harness (Claude Code, Pi, Codex).
//
//go:embed Containerfile
var Harness []byte
