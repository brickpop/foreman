// Package agentbin carries foreman-agent inside Foreman's binary: a Linux
// build for Foreman's own CPU type (agentbin/dist/foreman-agent, made by
// `just build` and `just dev`), mounted into every agent container
// (wiki/product/architecture/team-tools.md#transport).
package agentbin

import "embed"

//go:embed all:dist
var dist embed.FS

// Binary is foreman-agent, or nil before the first build.
func Binary() []byte {
	data, err := dist.ReadFile("dist/foreman-agent")
	if err != nil {
		return nil
	}
	return data
}
