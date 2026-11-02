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
	// Processes are the command lines of what else runs in the session
	// besides its main process, and in a container the init above it (not the
	// zombies): the helpers it keeps, and anything it started and left running,
	// orphans included. An idle agent sleeps only when
	// nothing but its helpers is left (wiki/product/architecture/agent-runtime.md#idle-agents-sleep).
	Processes(ctx context.Context) ([]string, error)
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

// Images builds the images an engine's processes run
// (wiki/product/architecture/building-blocks.md#1b-image-builder-runnerengine).
// Phase 1 has one image for every project; Prune and per-layer Refresh come
// with the full chain.
type Images interface {
	// Ensure returns the image to run project's processes on, building it
	// first if needed; concurrent callers share one build.
	Ensure(ctx context.Context, project string) (ref string, err error)
	// Refresh rebuilds without cache and with the latest base image (new
	// harness versions). Running processes keep their image.
	Refresh(ctx context.Context) error
	// Status is the state of each image, for Settings → System → Maintenance.
	Status(ctx context.Context) []ImageStatus
}
