// Package supervisor owns running sessions: it starts them through a Runner,
// keeps their output in a session log, and fans it out to every attached
// viewer (wiki/product/architecture/agent-runtime.md). It depends only on the
// runner.Runner interface and on Log, so both are swappable.
package supervisor

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sort"
	"sync"

	"github.com/brickpop/foreman/internal/runner"
)

// Log is where a session's raw output is kept, for replay.
type Log interface {
	io.Writer
	// Tail returns at most the last max bytes written.
	Tail(max int64) ([]byte, error)
	Close() error
}

// LogOpener opens the log of a new session of an instance.
type LogOpener func(instance string) (Log, error)

var (
	// ErrUnknown means no session was ever started for the instance.
	ErrUnknown = errors.New("supervisor: unknown instance")
	// ErrRunning means the instance already has a running session.
	ErrRunning = errors.New("supervisor: instance already running")
	// ErrExited means the session has ended and takes no more input.
	ErrExited = errors.New("supervisor: session has exited")
)

const (
	// ReplayMax is how much scrollback a newly attached viewer receives.
	ReplayMax = 1 << 20
	// viewerBuffer is how many output chunks a viewer may lag behind before it
	// is dropped; a slow viewer must never stall the session.
	viewerBuffer = 1024
)

// Supervisor runs sessions, one per instance at a time.
type Supervisor struct {
	runner  runner.Runner
	openLog LogOpener

	mu    sync.Mutex
	terms map[string]*terminal
}

// New returns a supervisor that starts sessions with r and logs them with open.
func New(r runner.Runner, open LogOpener) *Supervisor {
	return &Supervisor{runner: r, openLog: open, terms: map[string]*terminal{}}
}

// Start launches a session for instance.
func (s *Supervisor) Start(ctx context.Context, instance string, spec runner.Spec) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if t, ok := s.terms[instance]; ok && !t.hasExited() {
		return ErrRunning
	}
	log, err := s.openLog(instance)
	if err != nil {
		return err
	}
	session, err := s.runner.Start(ctx, spec)
	if err != nil {
		_ = log.Close()
		return err
	}
	t := &terminal{instance: instance, session: session, log: log, viewers: map[*Attachment]struct{}{}}
	s.terms[instance] = t
	go t.pump()
	return nil
}

// List returns the ids of the instances that have a session (running or exited), sorted.
func (s *Supervisor) List() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	ids := make([]string, 0, len(s.terms))
	for id := range s.terms {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// Attach returns a view of instance's session: the recent output to replay,
// then the live output until the session exits or the viewer falls behind.
func (s *Supervisor) Attach(instance string) (*Attachment, error) {
	s.mu.Lock()
	t, ok := s.terms[instance]
	s.mu.Unlock()
	if !ok {
		return nil, ErrUnknown
	}
	return t.attach()
}

type terminal struct {
	instance string
	session  runner.Session
	log      Log

	mu       sync.Mutex
	viewers  map[*Attachment]struct{}
	exited   bool
	exitCode int
}

func (t *terminal) hasExited() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.exited
}

// pump copies the session's output to the log and to every viewer, under one
// lock with attach, so a viewer sees the replay and then the live output with
// no gap and no duplicate.
func (t *terminal) pump() {
	buf := make([]byte, 32*1024)
	for {
		n, err := t.session.Read(buf)
		if n > 0 {
			chunk := append([]byte(nil), buf[:n]...)
			t.mu.Lock()
			if _, werr := t.log.Write(chunk); werr != nil {
				slog.Warn("session log write failed", "instance", t.instance, "err", werr)
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
	code, err := t.session.Wait()
	if err != nil {
		slog.Warn("session wait failed", "instance", t.instance, "err", err)
	}
	t.mu.Lock()
	t.exited, t.exitCode = true, code
	for v := range t.viewers {
		delete(t.viewers, v)
		v.end()
	}
	t.mu.Unlock()
	_ = t.log.Close()
}

func (t *terminal) attach() (*Attachment, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	replay, err := t.log.Tail(ReplayMax)
	if err != nil {
		return nil, err
	}
	a := &Attachment{term: t, replay: replay, out: make(chan []byte, viewerBuffer)}
	if t.exited {
		a.end()
	} else {
		t.viewers[a] = struct{}{}
	}
	return a, nil
}

// Attachment is one viewer of a session.
type Attachment struct {
	term   *terminal
	replay []byte
	out    chan []byte
	once   sync.Once
}

func (a *Attachment) end() { a.once.Do(func() { close(a.out) }) }

// Replay is the recent output, to show before the live output.
func (a *Attachment) Replay() []byte { return a.replay }

// Output delivers live output; it is closed when the session exits, when the
// viewer falls too far behind (attach again to catch up), or on Close.
func (a *Attachment) Output() <-chan []byte { return a.out }

// Exit reports the exit code once the session has exited.
func (a *Attachment) Exit() (code int, exited bool) {
	a.term.mu.Lock()
	defer a.term.mu.Unlock()
	return a.term.exitCode, a.term.exited
}

// Input sends bytes to the session's terminal, as typed.
func (a *Attachment) Input(p []byte) error {
	if a.term.hasExited() {
		return ErrExited
	}
	_, err := a.term.session.Write(p)
	return err
}

// Resize sets the session's terminal size. Several viewers share one terminal,
// so the last one to resize wins.
func (a *Attachment) Resize(size runner.Size) error {
	if a.term.hasExited() {
		return ErrExited
	}
	return a.term.session.Resize(size)
}

// Close detaches the viewer.
func (a *Attachment) Close() {
	a.term.mu.Lock()
	delete(a.term.viewers, a)
	a.term.mu.Unlock()
	a.end()
}
