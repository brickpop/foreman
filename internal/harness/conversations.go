package harness

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// Conversations are a harness's saved conversations in an instance's config
// dir, which outlives deployments: a restart continues only one of this
// deployment, never the last deployment's.
type Conversations struct {
	Dir  string // the instance's config dir
	Glob string // the conversation files, relative to Dir
}

// freshStart lists, in the config dir, the conversations there when the
// deployment's first session started.
const freshStart = "foreman-fresh-start"

// FreshStart is the file a deployment's first start writes: the
// conversations already there, which are never continued.
func (c Conversations) FreshStart() File {
	return File{Path: filepath.Join(c.Dir, freshStart), Data: []byte(strings.Join(c.list(), "\n")), Mode: 0o600}
}

// Continues reports whether there is a conversation of this deployment to
// continue: one that wasn't there at its fresh start. A harness asked to
// continue without one may fail (Claude Code) or pick up an older one (Pi).
func (c Conversations) Continues() bool {
	before, _ := os.ReadFile(filepath.Join(c.Dir, freshStart))
	old := strings.Split(string(before), "\n")
	for _, f := range c.list() {
		if !slices.Contains(old, f) {
			return true
		}
	}
	return false
}

func (c Conversations) list() []string {
	found, _ := filepath.Glob(filepath.Join(c.Dir, c.Glob))
	for i, f := range found {
		found[i], _ = filepath.Rel(c.Dir, f)
	}
	return found
}
