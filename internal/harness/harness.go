// Package harness is everything harness-specific (Claude Code, the fake
// harness in tests, and later Pi, Hermes, Codex, custom commands) behind one
// interface (wiki/product/architecture/building-blocks.md#3-harness-adapter-harness).
// Epic 1 needs Launch; Deliver, Interpret and Diagnose join with the agent link.
package harness

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/brickpop/foreman/internal/runner"
)

// Session is what an adapter needs to launch one instance.
type Session struct {
	Instance string // e.g. mara@ledger
	// Sandbox is the instance's folder on the host (<project>/.sandboxes/<agent>),
	// its working directory, mounted at the same path. The adapter keeps the
	// harness's private config inside it.
	Sandbox  string
	Prompt   string // the composed session prompt
	Command  string // the harness command from the settings; "" for the adapter's default
	Continue bool   // a restart: pick up the instance's previous conversation
}

// Launch is how to start a harness: what the instance manager adds to the
// process spec (alongside the worktrees and repos), and the files to write first.
type Launch struct {
	Command []string
	Env     []string
	Mounts  []runner.Mount
	Files   []File
}

// File is written on the host before the process starts.
type File struct {
	Path string
	Data []byte
	Mode os.FileMode
}

// ErrNotConnected means the harness has no login yet: Settings → Harnesses → Connect.
var ErrNotConnected = errors.New("harness: not connected (Settings → Harnesses → Connect)")

// Adapter is one harness's adapter (claudecode.Adapter, fake.Adapter, …), as
// runner.Runner is one engine (direct.Runner, podman.Runner).
type Adapter interface {
	Launch(Session) (Launch, error)
	// Login is how to run the harness's own login (Settings → Harnesses →
	// Connect), in a terminal the CEO types into.
	Login() (Launch, error)
	// Connected reports whether the harness has a login.
	Connected() bool
}

// WriteFiles writes a launch's files, creating their folders.
func WriteFiles(files []File) error {
	for _, f := range files {
		if err := os.MkdirAll(filepath.Dir(f.Path), 0o700); err != nil {
			return err
		}
		if err := os.WriteFile(f.Path, f.Data, f.Mode); err != nil {
			return err
		}
	}
	return nil
}
