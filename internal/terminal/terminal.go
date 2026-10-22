// Package terminal is one live process and its buffer: it keeps the output in
// a session log, fans it out to every viewer, and takes input
// (wiki/product/architecture/building-blocks.md#2-terminal-terminal). It does
// not decide whether a process should run, or restart it. It depends only on
// runner.Process and Log, so both are swappable.
package terminal

import (
	"errors"
	"io"
	"log/slog"
	"sync"
	"time"

	"github.com/brickpop/foreman/internal/runner"
)

// Log is where a process's raw output is kept, for replay.
type Log interface {
	io.Writer
	// Tail returns at most the last max bytes written.
	Tail(max int64) ([]byte, error)
	Close() error
}

var (
	// ErrExited means the process has ended and takes no more input.
	ErrExited = errors.New("terminal: process has exited")
	// ErrSealed means the process is being stopped and takes no more input.
	ErrSealed = errors.New("terminal: closed to input")
)

const (
	// SnapshotMax is how much recent output a newly attached viewer receives.
	SnapshotMax = 1 << 20
	// viewerBuffer is how many output chunks a viewer may lag behind before it
	// is dropped; a slow viewer must never stall the process.
	viewerBuffer = 1024
)

// Terminal is one running process, its log and its viewers.
type Terminal struct {
	process runner.Process
	log     Log
	name    string // for the server log only
	done    chan struct{}

	mu        sync.Mutex
	viewers   map[*View]struct{}
	exited    bool
	exitCode  int
	lastInput time.Time
	sealed    bool
}

// New takes over process and log: it reads the output until the process
// exits, then closes the log.
func New(name string, process runner.Process, log Log) *Terminal {
	t := &Terminal{process: process, log: log, name: name, done: make(chan struct{}), viewers: map[*View]struct{}{}}
	go t.pump()
	return t
}

// pump copies the output to the log and to every viewer, under one lock with
// Attach, so a viewer sees the snapshot and then the live output with no gap
// and no duplicate.
func (t *Terminal) pump() {
	buf := make([]byte, 32*1024)
	for {
		n, err := t.process.Read(buf)
		if n > 0 {
			chunk := append([]byte(nil), buf[:n]...)
			t.mu.Lock()
			if _, werr := t.log.Write(chunk); werr != nil {
				slog.Warn("session log write failed", "terminal", t.name, "err", werr)
			}
			for v := range t.viewers {
				select {
				case v.out <- chunk:
				default:
					delete(t.viewers, v)
					v.end()
				}
			}
			t.mu.Unlock()
		}
		if err != nil {
			break
		}
	}
	code, err := t.process.Wait()
	if err != nil {
		slog.Warn("process wait failed", "terminal", t.name, "err", err)
	}
	t.mu.Lock()
	t.exited, t.exitCode = true, code
	for v := range t.viewers {
		delete(t.viewers, v)
		v.end()
	}
	t.mu.Unlock()
	_ = t.log.Close()
	close(t.done)
}

// Attach returns a viewer: the recent output, then the live output until the
// process exits or the viewer falls behind.
func (t *Terminal) Attach() (*View, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	// Until the server-side screen is wired in, the snapshot is the log's tail
	// (wiki/backlog/terminal-fidelity.md).
	snapshot, err := t.log.Tail(SnapshotMax)
	if err != nil {
		return nil, err
	}
	v := &View{term: t, snapshot: snapshot, out: make(chan []byte, viewerBuffer)}
	if t.exited {
		v.end()
	} else {
		t.viewers[v] = struct{}{}
	}
	return v, nil
}

// Input sends bytes to the process, as typed.
func (t *Terminal) Input(p []byte) error {
	t.mu.Lock()
	exited, sealed := t.exited, t.sealed
	if !exited && !sealed {
		t.lastInput = time.Now()
	}
	t.mu.Unlock()
	switch {
	case exited:
		return ErrExited
	case sealed:
		return ErrSealed
	}
	_, err := t.process.Write(p)
	return err
}

// LastInput is when input last came; zero before any.
func (t *Terminal) LastInput() time.Time {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.lastInput
}

// Seal closes the terminal to input before its process is stopped, unless
// input came after since: then it reports false and stays open. Checked and
// closed in one step, so any input is either seen here or refused.
func (t *Terminal) Seal(since time.Time) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.lastInput.After(since) {
		return false
	}
	t.sealed = true
	return true
}

// Unseal opens the terminal to input again: its process wasn't stopped.
func (t *Terminal) Unseal() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.sealed = false
}

// Resize sets the terminal size. Viewers share one terminal, so the last one
// to resize wins.
func (t *Terminal) Resize(size runner.Size) error {
	if t.hasExited() {
		return ErrExited
	}
	return t.process.Resize(size)
}

// Done is closed once the process has exited and its output is fully logged.
func (t *Terminal) Done() <-chan struct{} { return t.done }

// ExitCode reports the exit code once the process has exited.
func (t *Terminal) ExitCode() (code int, exited bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.exitCode, t.exited
}

func (t *Terminal) hasExited() bool {
	_, exited := t.ExitCode()
	return exited
}

// View is one viewer of a terminal.
type View struct {
	term     *Terminal
	snapshot []byte
	out      chan []byte
	once     sync.Once
}

func (v *View) end() { v.once.Do(func() { close(v.out) }) }

// Snapshot recreates the screen as it was when the viewer attached; show it
// before the live output.
func (v *View) Snapshot() []byte { return v.snapshot }

// Output delivers live output; it is closed when the process exits, when the
// viewer falls too far behind (attach again to catch up), or on Close.
func (v *View) Output() <-chan []byte { return v.out }

// Close detaches the viewer.
func (v *View) Close() {
	v.term.mu.Lock()
	delete(v.term.viewers, v)
	v.term.mu.Unlock()
	v.end()
}
