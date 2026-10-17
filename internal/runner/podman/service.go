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
	"strconv"
	"strings"
	"time"
)

// The Podman service: Foreman makes sure it runs, so that starting `foreman`
// is the only step (wiki/product/architecture/session-runner.md#the-podman-service).

var (
	// ErrNoPodman means the podman CLI isn't installed.
	ErrNoPodman = errors.New("podman is not installed: see the README's Install section")
	// ErrNoMachine means macOS has no Podman machine yet.
	ErrNoMachine = errors.New("there is no Podman machine yet")
	// ErrNotRunning means Podman doesn't answer: not connected yet, or the
	// macOS machine is stopped.
	ErrNotRunning = errors.New("podman isn't running")
)

// State is how far Podman is from running agents: the setup checklist's
// Podman steps (wiki/product/features/first-run.md).
type State struct {
	Installed bool   // the podman CLI is on the PATH
	Machine   string // macOS: "none", "creating", "stopped" or "running"; "" elsewhere
	Running   bool   // its API answers
	Version   string // when it answers
	Error     string // why the last connection failed
	Log       string // the machine creation's output, while and after it runs
	// Bounds is the size a new machine may have (macOS, no machine yet).
	Bounds *MachineBounds
}

// State checks Podman now. It starts nothing.
func (r *Runner) State(ctx context.Context) State {
	var s State
	r.mu.Lock()
	c, job, lastErr := r.c, r.machine, r.lastErr
	r.mu.Unlock()
	if lastErr != nil {
		s.Error = lastErr.Error()
	}
	s.Log = job.log
	if c != nil {
		if v, err := c.version(ctx); err == nil {
			s.Installed, s.Running, s.Version = true, true, v
			if runtime.GOOS == "darwin" {
				s.Machine = "running"
			}
			return s
		}
	}
	if socketOverride() != "" {
		s.Installed = true
		return s
	}
	if _, err := exec.LookPath("podman"); err != nil {
		return s
	}
	s.Installed = true
	if runtime.GOOS != "darwin" {
		return s
	}
	switch state, err := machineState(ctx); {
	case job.running:
		s.Machine = "creating"
	case errors.Is(err, ErrNoMachine):
		s.Machine = "none"
		if b, err := hostBounds(ctx); err == nil {
			s.Bounds = &b
		} else {
			s.Error = err.Error()
		}
	case err != nil:
		s.Machine, s.Error = "stopped", err.Error()
	case state == "running":
		s.Machine = "running"
	default:
		s.Machine = "stopped"
	}
	return s
}

// Installed reports whether Foreman could run agents in Podman: the CLI is on
// the PATH and, on macOS, a machine exists. It starts nothing.
func Installed(ctx context.Context) error {
	s := Open().State(ctx)
	switch {
	case !s.Installed:
		return ErrNoPodman
	case s.Machine == "none" || s.Machine == "creating":
		return ErrNoMachine
	}
	return nil
}

// Connect reaches Podman, unless it already answers. On Linux it starts the
// rootless service when nothing answers (a child of Foreman, ended by Close).
// On macOS it starts the machine only when start is set (Foreman's launch,
// the CEO's Start), so a machine the CEO stopped stays stopped.
func (r *Runner) Connect(ctx context.Context, start bool) error {
	r.connecting.Lock()
	defer r.connecting.Unlock()
	if c, err := r.client(); err == nil {
		if _, err := c.version(ctx); err == nil {
			return nil
		}
	}
	socket, stop, err := findSocket(ctx, start)
	if err == nil {
		if _, verr := newClient(socket).version(ctx); verr != nil {
			if stop != nil {
				stop()
			}
			stop, err = nil, fmt.Errorf("%w: its API on %s: %v", ErrNotRunning, socket, verr)
		}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.stopService != nil {
		r.stopService() // a service Foreman started before, which stopped answering
	}
	r.c, r.stopService, r.lastErr = nil, stop, err
	if err == nil {
		r.c = newClient(socket)
	}
	return err
}

func socketOverride() string { return os.Getenv("FOREMAN_PODMAN_SOCKET") }

// findSocket returns Podman's API socket, starting the service when nothing
// answers, and on macOS the machine when start is set. FOREMAN_PODMAN_SOCKET
// overrides the search. stop ends a service Foreman started (nil when it was
// already running).
func findSocket(ctx context.Context, start bool) (socket string, stop func(), err error) {
	if s := socketOverride(); s != "" {
		return s, nil, nil
	}
	if _, err := exec.LookPath("podman"); err != nil {
		return "", nil, ErrNoPodman
	}
	if runtime.GOOS == "darwin" {
		socket, err := machineSocket(ctx, start)
		return socket, nil, err
	}
	return linuxSocket(ctx)
}

// machineState is the default machine's state ("running", "stopped", …), or
// ErrNoMachine.
func machineState(ctx context.Context) (string, error) {
	out, err := podmanOutput(ctx, "machine", "list", "--format", "{{.Name}} {{.Running}}")
	if err != nil {
		return "", err
	}
	if out == "" {
		return "", ErrNoMachine
	}
	state, err := podmanOutput(ctx, "machine", "inspect", "--format", "{{.State}}")
	if err != nil {
		return "", err
	}
	return state, nil
}

// machineSocket returns the API socket the default machine exposes, starting
// it first when start is set. Foreman never stops the machine: other tools may
// use it, and the agents' containers live in it.
func machineSocket(ctx context.Context, start bool) (string, error) {
	state, err := machineState(ctx)
	if err != nil {
		return "", err
	}
	if state != "running" {
		if !start {
			return "", fmt.Errorf("%w: the Podman machine is %s", ErrNotRunning, state)
		}
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
	ownGroup(cmd)
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

// MachineBounds is the size a new Podman machine may have (the CEO's rule,
// 2026-10-16): memory from 4 GiB, in whole GiB, up to 60% of the Mac's; CPUs
// from 1 to all of the Mac's; each prefilled with half the Mac, within bounds.
type MachineBounds struct {
	MinMemory, MaxMemory, Memory int // GiB
	MaxCPUs, CPUs                int
}

// ErrBadSize is a machine size outside its bounds.
var ErrBadSize = errors.New("podman: machine size out of bounds")

// minMachineMemory is the smallest machine Foreman creates, in GiB.
const minMachineMemory = 4

// machineBounds are the bounds on a Mac with hostMiB of memory and cpus cores;
// MaxMemory < MinMemory when it is too small for any.
func machineBounds(hostMiB uint64, cpus int) MachineBounds {
	maxMemory := int(hostMiB * 6 / 10 / 1024)
	half := int(hostMiB / 2 / 1024)
	return MachineBounds{
		MinMemory: minMachineMemory, MaxMemory: maxMemory,
		Memory:  max(minMachineMemory, min(half, maxMemory)),
		MaxCPUs: cpus, CPUs: max(1, cpus/2),
	}
}

func hostBounds(ctx context.Context) (MachineBounds, error) {
	out, err := exec.CommandContext(ctx, "sysctl", "-n", "hw.memsize").Output()
	if err != nil {
		return MachineBounds{}, fmt.Errorf("the Mac's memory: %w", err)
	}
	bytes, err := strconv.ParseUint(strings.TrimSpace(string(out)), 10, 64)
	if err != nil {
		return MachineBounds{}, fmt.Errorf("the Mac's memory: %w", err)
	}
	return machineBounds(bytes>>20, runtime.NumCPU()), nil
}

// machineJob is a machine creation: its output, kept for State.
type machineJob struct {
	running bool
	log     string
}

// CreateMachine creates and starts the default Podman machine (macOS) with
// cpus and memory (GiB), in the background: State reports "creating" and
// Podman's output meanwhile, then the machine. Connect reaches it afterwards.
func (r *Runner) CreateMachine(ctx context.Context, cpus, memory int) error {
	if runtime.GOOS != "darwin" {
		return errors.New("podman: a machine is only needed on macOS")
	}
	b, err := hostBounds(ctx)
	if err != nil {
		return err
	}
	if memory < b.MinMemory || memory > b.MaxMemory || cpus < 1 || cpus > b.MaxCPUs {
		return fmt.Errorf("%w: memory %d–%d GiB, CPUs 1–%d", ErrBadSize, b.MinMemory, b.MaxMemory, b.MaxCPUs)
	}
	if _, err := machineState(ctx); !errors.Is(err, ErrNoMachine) {
		return errors.New("podman: a machine exists already")
	}
	r.mu.Lock()
	if r.machine.running {
		r.mu.Unlock()
		return errors.New("podman: the machine is being created")
	}
	r.machine = machineJob{running: true}
	r.mu.Unlock()
	cmd := exec.Command("podman", "machine", "init", "--now",
		"--cpus", strconv.Itoa(cpus), "--memory", strconv.Itoa(memory*1024))
	cmd.Stdout = r.machineLog()
	cmd.Stderr = cmd.Stdout
	if err := cmd.Start(); err != nil {
		r.endMachine(err)
		return err
	}
	go func() { r.endMachine(cmd.Wait()) }()
	return nil
}

func (r *Runner) endMachine(err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.machine.running = false
	if err != nil {
		r.machine.log += "\n" + err.Error() + "\n"
	}
}

// machineLog appends to the creation's log, keeping its end.
func (r *Runner) machineLog() logWriter { return logWriter{r: r} }

type logWriter struct{ r *Runner }

func (w logWriter) Write(b []byte) (int, error) {
	w.r.mu.Lock()
	defer w.r.mu.Unlock()
	w.r.machine.log += string(b)
	if len(w.r.machine.log) > maxLog {
		w.r.machine.log = w.r.machine.log[len(w.r.machine.log)-maxLog:]
	}
	return len(b), nil
}
