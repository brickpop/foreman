// Package fake is the adapter of the fake harness (cmd/fakeharness), so that
// everything above the harness is tested without a real one.
package fake

import (
	"path/filepath"

	"github.com/brickpop/foreman/internal/harness"
)

// Adapter launches the fake harness.
type Adapter struct {
	Program string // the fakeharness binary
}

var _ harness.Adapter = Adapter{}

// Launch runs the program, with its session prompt in a file it is told about.
func (a Adapter) Launch(s harness.Session) (harness.Launch, error) {
	prompt := filepath.Join(s.Sandbox, ".harness", "fake", "PROMPT.md")
	return harness.Launch{
		Command: []string{a.Program},
		Env:     []string{"FOREMAN_PROMPT_FILE=" + prompt},
		Files:   []harness.File{{Path: prompt, Data: []byte(s.Prompt), Mode: 0o600}},
	}, nil
}
