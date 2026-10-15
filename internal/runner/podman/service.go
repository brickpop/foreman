package podman

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// The Podman service: Foreman makes sure it runs, so that starting `foreman`
// is the only step (wiki/product/architecture/session-runner.md#the-podman-service).

// ErrNoPodman means Podman isn't installed (or, on macOS, has no machine).
var ErrNoPodman = errors.New("podman is not installed: see the README's prerequisites")

// Installed reports whether Foreman could run agents in Podman: the CLI is on
// the PATH and, on macOS, a machine exists. It starts nothing (Foreman starts
// the service or machine itself when it launches with engine = "podman").
func Installed(ctx context.Context) error {
	if os.Getenv("FOREMAN_PODMAN_SOCKET") != "" {
		return nil
	}
	if _, err := exec.LookPath("podman"); err != nil {
		return ErrNoPodman
	}
	if runtime.GOOS == "darwin" {
		if _, err := podmanOutput(ctx, "machine", "inspect", "--format", "{{.State}}"); err != nil {
			return fmt.Errorf("%w (no podman machine: run `podman machine init` once)", ErrNoPodman)
		}
	}
	return nil
}

// findSocket returns Podman's API socket, starting the service when nothing
// answers. FOREMAN_PODMAN_SOCKET overrides the search. stop ends a service
// Foreman started (nil when it was already running).
func findSocket(ctx context.Context) (socket string, stop func(), err error) {
	if s := os.Getenv("FOREMAN_PODMAN_SOCKET"); s != "" {
		return s, nil, nil
	}
	if _, err := exec.LookPath("podman"); err != nil {
		return "", nil, ErrNoPodman
	}
	if runtime.GOOS == "darwin" {
		socket, err := machineSocket(ctx)
		return socket, nil, err
	}
	return linuxSocket(ctx)
}

// machineSocket starts the default podman machine if needed and returns the
// API socket it exposes. Foreman never stops the machine: other tools may use
// it, and the agents' containers live in it.
func machineSocket(ctx context.Context) (string, error) {
	state, err := podmanOutput(ctx, "machine", "inspect", "--format", "{{.State}}")
	if err != nil {
		return "", fmt.Errorf("%w (no podman machine? run `podman machine init` once)", err)
	}
	if state != "running" {
		if _, err := podmanOutput(ctx, "machine", "start"); err != nil {
			return "", err
		}
	}
	return podmanOutput(ctx, "machine", "inspect", "--format", "{{.ConnectionInfo.PodmanSocket.Path}}")
}

// linuxSocket uses the rootless socket, or starts `podman system service` on
// it as a child that ends with Foreman (stop).
func linuxSocket(ctx context.Context) (string, func(), error) {
	dir := os.Getenv("XDG_RUNTIME_DIR")
	if dir == "" {
		dir = fmt.Sprintf("/run/user/%d", os.Getuid())
	}
	socket := filepath.Join(dir, "podman", "podman.sock")
	if answers(socket) {
		return socket, nil, nil
	}
	cmd := exec.Command("podman", "system", "service", "--time=0", "unix://"+socket)
	if err := cmd.Start(); err != nil {
		return "", nil, fmt.Errorf("starting podman system service: %w", err)
	}
	stop := func() { _ = cmd.Process.Kill(); _ = cmd.Wait() }
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(100 * time.Millisecond) {
		if ctx.Err() != nil {
			stop()
			return "", nil, ctx.Err()
		}
		if answers(socket) {
			return socket, stop, nil
		}
	}
	stop()
	return "", nil, fmt.Errorf("the Podman service didn't come up on %s", socket)
}

func answers(socket string) bool {
	conn, err := net.DialTimeout("unix", socket, time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

func podmanOutput(ctx context.Context, args ...string) (string, error) {
	out, err := exec.CommandContext(ctx, "podman", args...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("podman %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}
