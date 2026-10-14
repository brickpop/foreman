// Package runner defines where a harness process runs. Implementations live in
// subpackages (direct, and podman later); see
// wiki/product/architecture/session-runner.md.
package runner

import (
	"context"
	"io"
	"time"
)

// Runner starts processes: one harness process attached to a terminal.
type Runner interface {
	// Start launches the process described by spec.
	Start(ctx context.Context, spec Spec) (Process, error)
	// List returns the processes still running that this runner started
	// (found by label), so they can be re-attached after a Foreman restart.
	// Runners whose processes die with Foreman return none.
	List(ctx context.Context) ([]Process, error)
}

// Process is one running process with a terminal.
type Process interface {
	// Read returns the terminal's output (stdout and stderr, merged, raw bytes).
	// Write sends input to the terminal.
	io.ReadWriter
	// Labels are the spec's labels, so a listed session can be recognised.
	Labels() map[string]string
	// Resize propagates a terminal size change.
	Resize(Size) error
	// Stop asks the process to end, and kills it after grace.
	Stop(grace time.Duration) error
	// Wait blocks until the process exits and returns its exit code.
	Wait() (exitCode int, err error)
}

// Spec describes a session to start.
type Spec struct {
	Name    string            // unique, human-readable (e.g. the instance id)
	Image   string            // container image; ignored by runners without containers
	Command []string          // program and arguments
	Dir     string            // working directory
	Env     []string          // "KEY=value" entries, added to a minimal environment
	Mounts  []Mount           // host paths visible to the session; ignored without containers
	Size    Size              // initial terminal size
	Labels  map[string]string // carried by the session, e.g. foreman.instance=<id>
}

// Mount makes a host path visible inside a session.
type Mount struct {
	Source, Target string
	ReadOnly       bool
}

// Size is a terminal size in character cells.
type Size struct {
	Cols, Rows uint16
}
