// Package agentbin carries foreman-agent inside Foreman's binary: a Linux
// build per CPU type (agentbin/dist/foreman-agent-<arch>, made by `just agent`),
// mounted into every agent container
// (wiki/product/architecture/team-tools.md#transport).
package agentbin

// Binary is foreman-agent for a CPU type as Go names it (amd64, arm64), or nil
// when it isn't embedded: a development build before `just agent`, or a CPU
// type Foreman doesn't ship for.
func Binary(arch string) []byte {
	data, err := dist.ReadFile("dist/foreman-agent-" + arch)
	if err != nil {
		return nil
	}
	return data
}
