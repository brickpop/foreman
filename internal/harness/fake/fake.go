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

// Launch runs the program, with its session prompt in a file it is told about
// and the opener, if any, as its argument.
func (a Adapter) Launch(s harness.Session) (harness.Launch, error) {
	prompt := filepath.Join(s.Sandbox, ".harness", "fake", "PROMPT.md")
	command := []string{a.Program}
	if s.Opener != "" {
		command = append(command, s.Opener)
	}
	return harness.Launch{
		Command: command,
		Env:     append([]string{"FOREMAN_PROMPT_FILE=" + prompt}, s.Link.Env...),
		Files:   []harness.File{{Path: prompt, Data: []byte(s.Prompt), Mode: 0o600}},
	}, nil
}

// Command is the program.
func (a Adapter) Command() string { return a.Program }

// Newline is `\` then Enter: the fake harness continues a line ending in `\`.
func (Adapter) Newline() string { return "\\\r" }

// Connected is always true: the fake harness needs no login.
func (Adapter) Connected() bool { return true }

// Login runs the program, as if it were a login.
func (a Adapter) Login() (harness.Launch, error) {
	return harness.Launch{Command: []string{a.Program}}, nil
}

// Interpret: the fake harness sends no hooks.
func (Adapter) Interpret(string, []byte) harness.Activity { return harness.Unknown }
