// Package direct runs processes on the host, in a pseudo-terminal Foreman
// allocates. It gives NO isolation: it exists for tests and development
// (wiki/product/architecture/session-runner.md). Image and Mounts are ignored,
// and processes die with Foreman, so List returns none.
package direct

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	creack "github.com/creack/pty"

	"github.com/brickpop/foreman/internal/runner"
)

// Runner is the Direct execution engine.
type Runner struct{}

var _ runner.Runner = Runner{}

// Start launches spec.Command in a new pseudo-terminal.
func (Runner) Start(ctx context.Context, spec runner.Spec) (runner.Process, error) {
	if len(spec.Command) == 0 {
		return nil, errors.New("direct: empty command")
	}
	// Not CommandContext: a session outlives the request that started it.
	cmd := exec.Command(spec.Command[0], spec.Command[1:]...)
	cmd.Dir = spec.Dir
	// A minimal environment: the host's would leak the CEO's credentials and
	// settings into the session, which the container runner never does either.
	cmd.Env = append([]string{"TERM=xterm-256color", "PATH=" + os.Getenv("PATH"), "HOME=" + os.Getenv("HOME")}, spec.Env...)
	size := spec.Size
	if size.Cols == 0 || size.Rows == 0 {
		size = runner.Size{Cols: 120, Rows: 40} // a stable baseline before the first resize
	}
	tty, err := creack.StartWithSize(cmd, &creack.Winsize{Cols: size.Cols, Rows: size.Rows})
	if err != nil {
		return nil, err
	}
	pr := &process{cmd: cmd, tty: tty, labels: spec.Labels, done: make(chan struct{}), drained: make(chan struct{})}
	go pr.reap()
	return pr, nil
}

// List returns nothing: direct processes die with Foreman.
func (Runner) List(context.Context) ([]runner.Process, error) { return nil, nil }

type process struct {
	cmd    *exec.Cmd
	tty    *os.File
	labels map[string]string

	done     chan struct{} // closed when the process has exited
	exitCode int
	waitErr  error

	drained  chan struct{} // closed when a read has reached the end of the output
	drainOne sync.Once
}

// drainGrace bounds how long the terminal stays open after the process exits,
// waiting for a reader to drain the last output. Linux ends the read by itself
// (EIO); some platforms keep it blocked until the terminal is closed.
const drainGrace = 2 * time.Second

func (pr *process) reap() {
	err := pr.cmd.Wait()
	var exit *exec.ExitError
	switch {
	case err == nil:
	case errors.As(err, &exit):
		pr.exitCode = exit.ExitCode()
	default:
		pr.waitErr = err
	}
	close(pr.done)
	select {
	case <-pr.drained:
	case <-time.After(drainGrace):
	}
	_ = pr.tty.Close()
}

func (pr *process) Read(p []byte) (int, error) {
	n, err := pr.tty.Read(p)
	// Linux reports the end of a PTY whose process exited as EIO; it's EOF.
	if err != nil {
		if errors.Is(err, syscall.EIO) || errors.Is(err, os.ErrClosed) {
			err = io.EOF
		}
		pr.drainOne.Do(func() { close(pr.drained) })
	}
	return n, err
}

func (pr *process) Write(p []byte) (int, error) { return pr.tty.Write(p) }

func (pr *process) Labels() map[string]string { return pr.labels }

func (pr *process) Resize(size runner.Size) error {
	return creack.Setsize(pr.tty, &creack.Winsize{Cols: size.Cols, Rows: size.Rows})
}

func (pr *process) Stop(grace time.Duration) error {
	select {
	case <-pr.done:
		return nil
	default:
	}
	_ = pr.cmd.Process.Signal(syscall.SIGTERM)
	select {
	case <-pr.done:
	case <-time.After(grace):
		_ = pr.cmd.Process.Kill()
		<-pr.done
	}
	return nil
}

func (pr *process) Wait() (int, error) {
	<-pr.done
	return pr.exitCode, pr.waitErr
}

// Processes are the descendants of the process, found with ps (Linux and
// macOS both have it), zombies left out.
func (pr *process) Processes(ctx context.Context) ([]string, error) {
	out, err := exec.CommandContext(ctx, "ps", "-A", "-o", "pid=,ppid=,stat=,args=").Output()
	if err != nil {
		return nil, fmt.Errorf("direct: ps: %w", err)
	}
	return descendants(string(out), pr.cmd.Process.Pid), nil
}

// descendants reads ps's "pid ppid stat args" lines: the command lines of
// root's live descendants.
func descendants(ps string, root int) []string {
	type proc struct {
		ppid   int
		zombie bool
		args   string
	}
	procs := map[int]proc{}
	for _, line := range strings.Split(ps, "\n") {
		f := strings.Fields(line)
		if len(f) < 4 {
			continue
		}
		pid, err1 := strconv.Atoi(f[0])
		ppid, err2 := strconv.Atoi(f[1])
		if err1 != nil || err2 != nil {
			continue
		}
		procs[pid] = proc{ppid: ppid, zombie: strings.HasPrefix(f[2], "Z"), args: strings.Join(f[3:], " ")}
	}
	var list []string
	for _, p := range procs {
		if p.zombie {
			continue
		}
		// Bounded by the count: ps reads each process at its own moment, so a
		// recycled pid could make a loop.
		for up, seen := p.ppid, 0; up > 1 && seen < len(procs); up, seen = procs[up].ppid, seen+1 {
			if up == root {
				list = append(list, p.args)
				break
			}
		}
	}
	slices.Sort(list)
	return list
}
