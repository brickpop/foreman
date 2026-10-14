// Package pty runs sessions as plain processes on the host, in a
// pseudo-terminal. It gives NO isolation: it exists for tests and development
// (wiki/product/architecture/session-runner.md). Image and Mounts are ignored,
// and sessions die with Foreman, so List returns none.
package pty

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	creack "github.com/creack/pty"

	"github.com/brickpop/foreman/internal/runner"
)

// Runner is the local PTY runner.
type Runner struct{}

var _ runner.Runner = Runner{}

// Start launches spec.Command in a new pseudo-terminal.
func (Runner) Start(ctx context.Context, spec runner.Spec) (runner.Session, error) {
	if len(spec.Command) == 0 {
		return nil, errors.New("pty: empty command")
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
	s := &session{cmd: cmd, tty: tty, labels: spec.Labels, done: make(chan struct{}), drained: make(chan struct{})}
	go s.reap()
	return s, nil
}

// List returns nothing: PTY sessions don't survive Foreman.
func (Runner) List(context.Context) ([]runner.Session, error) { return nil, nil }

type session struct {
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

func (s *session) reap() {
	err := s.cmd.Wait()
	var exit *exec.ExitError
	switch {
	case err == nil:
	case errors.As(err, &exit):
		s.exitCode = exit.ExitCode()
	default:
		s.waitErr = err
	}
	close(s.done)
	select {
	case <-s.drained:
	case <-time.After(drainGrace):
	}
	_ = s.tty.Close()
}

func (s *session) Read(p []byte) (int, error) {
	n, err := s.tty.Read(p)
	// Linux reports the end of a PTY whose process exited as EIO; it's EOF.
	if err != nil {
		if errors.Is(err, syscall.EIO) || errors.Is(err, os.ErrClosed) {
			err = io.EOF
		}
		s.drainOne.Do(func() { close(s.drained) })
	}
	return n, err
}

func (s *session) Write(p []byte) (int, error) { return s.tty.Write(p) }

func (s *session) Labels() map[string]string { return s.labels }

func (s *session) Resize(size runner.Size) error {
	return creack.Setsize(s.tty, &creack.Winsize{Cols: size.Cols, Rows: size.Rows})
}

func (s *session) Stop(grace time.Duration) error {
	select {
	case <-s.done:
		return nil
	default:
	}
	_ = s.cmd.Process.Signal(syscall.SIGTERM)
	select {
	case <-s.done:
	case <-time.After(grace):
		_ = s.cmd.Process.Kill()
		<-s.done
	}
	return nil
}

func (s *session) Wait() (int, error) {
	<-s.done
	return s.exitCode, s.waitErr
}
