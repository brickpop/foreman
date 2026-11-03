// Package fake is the adapter of the fake harness (cmd/fakeharness), so that
// everything above the harness is tested without a real one.
package fake

import (
	"os"
	"path/filepath"
	"syscall"

	"github.com/brickpop/foreman/internal/harness"
)

// Adapter launches the fake harness.
type Adapter struct {
	Program string // the fakeharness binary
}

var _ harness.Adapter = Adapter{}

// Launch runs the program, with its session prompt in a file it is told about
// and the opener, if any, as its argument. It keeps no conversation, so every
// start is fresh (and gets the opener).
func (a Adapter) Launch(s harness.Session) (harness.Launch, error) {
	prompt := filepath.Join(s.Dir, "fake", "PROMPT.md")
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
func (Adapter) Connected(string) bool { return true }

// Login runs the program, as if it were a login.
func (a Adapter) Login(string) (harness.Launch, error) {
	return harness.Launch{Command: []string{a.Program}}, nil
}

// Interpret reads the hooks the fake harness sends, named as Claude Code's.
func (Adapter) Interpret(event string, _ []byte) harness.Activity {
	switch event {
	case "SessionStart", "Stop":
		return harness.Idle
	case "UserPromptSubmit":
		return harness.Busy
	}
	return harness.Unknown
}

// Asks: the fake harness never waits for the user.
func (Adapter) Asks(string, []byte) string { return "" }

// Describe names the same hooks as Claude Code's adapter does.
func (Adapter) Describe(event string, _ []byte) string {
	return map[string]string{
		"SessionStart":     "Started",
		"UserPromptSubmit": "Working on a message",
		"Stop":             "Finished its turn",
	}[event]
}

// WriteProgram writes a shell script for the fake harness to run, in place of
// the fakeharness binary. It holds syscall.ForkLock while the file is open for
// writing: a process started meanwhile from another goroutine would inherit
// that descriptor until its exec, and running the script then fails with
// "text file busy" (seen as flaky tests).
func WriteProgram(path, script string) error {
	syscall.ForkLock.RLock()
	defer syscall.ForkLock.RUnlock()
	return os.WriteFile(path, []byte("#!/bin/sh\n"+script+"\n"), 0o755)
}
