package supervisor

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/brickpop/foreman/internal/runner"
)

// --- doubles -----------------------------------------------------------------

// fakeSession's output is whatever the test writes to it; it exits when the
// test calls exit.
type fakeSession struct {
	outR *io.PipeReader
	outW *io.PipeWriter

	mu     sync.Mutex
	input  bytes.Buffer
	sizes  []runner.Size
	code   int
	exited chan struct{}
}

func newFakeSession() *fakeSession {
	r, w := io.Pipe()
	return &fakeSession{outR: r, outW: w, exited: make(chan struct{})}
}

func (f *fakeSession) print(t *testing.T, s string) {
	t.Helper()
	if _, err := f.outW.Write([]byte(s)); err != nil {
		t.Fatal(err)
	}
}

func (f *fakeSession) exit(code int) {
	f.mu.Lock()
	f.code = code
	f.mu.Unlock()
	_ = f.outW.Close()
	close(f.exited)
}

func (f *fakeSession) Read(p []byte) (int, error) { return f.outR.Read(p) }
func (f *fakeSession) Write(p []byte) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.input.Write(p)
}
func (f *fakeSession) Labels() map[string]string { return nil }
func (f *fakeSession) Resize(s runner.Size) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sizes = append(f.sizes, s)
	return nil
}
func (f *fakeSession) Stop(time.Duration) error { return nil }
func (f *fakeSession) Wait() (int, error) {
	<-f.exited
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.code, nil
}

type fakeRunner struct {
	sessions []*fakeSession
	err      error
}

func (r *fakeRunner) Start(context.Context, runner.Spec) (runner.Session, error) {
	if r.err != nil {
		return nil, r.err
	}
	s := newFakeSession()
	r.sessions = append(r.sessions, s)
	return s, nil
}
func (r *fakeRunner) List(context.Context) ([]runner.Session, error) { return nil, nil }

type memLog struct {
	mu     sync.Mutex
	buf    bytes.Buffer
	closed bool
}

func (l *memLog) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.Write(p)
}
func (l *memLog) Tail(max int64) ([]byte, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	b := l.buf.Bytes()
	if int64(len(b)) > max {
		b = b[int64(len(b))-max:]
	}
	return append([]byte(nil), b...), nil
}
func (l *memLog) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.closed = true
	return nil
}
func (l *memLog) isClosed() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.closed
}

type fixture struct {
	sup    *Supervisor
	runner *fakeRunner
	logs   []*memLog
}

func newFixture() *fixture {
	f := &fixture{runner: &fakeRunner{}}
	f.sup = New(f.runner, func(string) (Log, error) {
		l := &memLog{}
		f.logs = append(f.logs, l)
		return l, nil
	})
	return f
}

func (f *fixture) start(t *testing.T, instance string) *fakeSession {
	t.Helper()
	if err := f.sup.Start(context.Background(), instance, runner.Spec{}); err != nil {
		t.Fatal(err)
	}
	return f.runner.sessions[len(f.runner.sessions)-1]
}

func (f *fixture) attach(t *testing.T, instance string) *Attachment {
	t.Helper()
	a, err := f.sup.Attach(instance)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

// collect gathers a viewer's live output until it contains want.
func collect(t *testing.T, a *Attachment, want string) string {
	t.Helper()
	var got strings.Builder
	deadline := time.After(3 * time.Second)
	for !strings.Contains(got.String(), want) {
		select {
		case chunk, ok := <-a.Output():
			if !ok {
				t.Fatalf("output closed before %q; got %q", want, got.String())
			}
			got.Write(chunk)
		case <-deadline:
			t.Fatalf("never got %q; got %q", want, got.String())
		}
	}
	return got.String()
}

func waitClosed(t *testing.T, a *Attachment) {
	t.Helper()
	deadline := time.After(3 * time.Second)
	for {
		select {
		case _, ok := <-a.Output():
			if !ok {
				return
			}
		case <-deadline:
			t.Fatal("output never closed")
		}
	}
}

// --- tests -------------------------------------------------------------------

func TestEveryViewerGetsTheOutputAndTheLogKeepsIt(t *testing.T) {
	f := newFixture()
	s := f.start(t, "mara@ledger")
	a, b := f.attach(t, "mara@ledger"), f.attach(t, "mara@ledger")
	s.print(t, "hello \x1b[32mworld\x1b[0m")
	for _, v := range []*Attachment{a, b} {
		if got := collect(t, v, "world\x1b[0m"); got != "hello \x1b[32mworld\x1b[0m" {
			t.Errorf("viewer got %q", got)
		}
	}
	if got, _ := f.logs[0].Tail(100); string(got) != "hello \x1b[32mworld\x1b[0m" {
		t.Errorf("log has %q", got)
	}
}

// A viewer attaching mid-stream sees the earlier output once (replay) and the
// rest live: no gap, no duplicate.
func TestLateViewerGetsReplayThenLiveWithoutGapOrDuplicate(t *testing.T) {
	f := newFixture()
	s := f.start(t, "x")
	early := f.attach(t, "x")
	s.print(t, "one ")
	collect(t, early, "one")
	late := f.attach(t, "x")
	s.print(t, "two")
	live := collect(t, late, "two")
	if got := string(late.Replay()) + live; got != "one two" {
		t.Errorf("late viewer saw %q, want %q", got, "one two")
	}
}

func TestInputAndResizeReachTheSession(t *testing.T) {
	f := newFixture()
	s := f.start(t, "x")
	a := f.attach(t, "x")
	if err := a.Input([]byte("ls\r")); err != nil {
		t.Fatal(err)
	}
	if err := a.Resize(runner.Size{Cols: 100, Rows: 30}); err != nil {
		t.Fatal(err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.input.String() != "ls\r" || len(s.sizes) != 1 || s.sizes[0] != (runner.Size{Cols: 100, Rows: 30}) {
		t.Errorf("session got input %q, sizes %v", s.input.String(), s.sizes)
	}
}

func TestExitEndsViewersAndIsReported(t *testing.T) {
	f := newFixture()
	s := f.start(t, "x")
	a := f.attach(t, "x")
	s.print(t, "last words")
	collect(t, a, "last words")
	s.exit(3)
	waitClosed(t, a)
	if code, exited := a.Exit(); !exited || code != 3 {
		t.Errorf("Exit = %d, %v; want 3, true", code, exited)
	}
	if err := a.Input([]byte("x")); !errors.Is(err, ErrExited) {
		t.Errorf("input after exit: %v", err)
	}
	if err := a.Resize(runner.Size{Cols: 1, Rows: 1}); !errors.Is(err, ErrExited) {
		t.Errorf("resize after exit: %v", err)
	}
	if !f.logs[0].isClosed() {
		t.Error("log left open after exit")
	}
	// Attaching after the exit still shows what happened.
	after := f.attach(t, "x")
	if string(after.Replay()) != "last words" {
		t.Errorf("replay after exit %q", after.Replay())
	}
	waitClosed(t, after)
}

func TestStartRules(t *testing.T) {
	f := newFixture()
	if _, err := f.sup.Attach("nobody"); !errors.Is(err, ErrUnknown) {
		t.Errorf("attach unknown: %v", err)
	}
	s := f.start(t, "x")
	if err := f.sup.Start(context.Background(), "x", runner.Spec{}); !errors.Is(err, ErrRunning) {
		t.Errorf("second start while running: %v", err)
	}
	a := f.attach(t, "x")
	s.exit(0)
	waitClosed(t, a)
	// After an exit the instance can run again, in a new session with a new log.
	f.start(t, "x")
	if len(f.logs) != 2 {
		t.Errorf("restart reused the log: %d logs", len(f.logs))
	}
}

func TestStartFailuresLeaveNothingBehind(t *testing.T) {
	f := newFixture()
	f.runner.err = errors.New("no podman")
	if err := f.sup.Start(context.Background(), "x", runner.Spec{}); err == nil {
		t.Fatal("start succeeded")
	}
	if !f.logs[0].isClosed() {
		t.Error("log of a session that never started was left open")
	}
	if _, err := f.sup.Attach("x"); !errors.Is(err, ErrUnknown) {
		t.Errorf("a failed start left an instance behind: %v", err)
	}

	sup := New(&fakeRunner{}, func(string) (Log, error) { return nil, errors.New("disk full") })
	if err := sup.Start(context.Background(), "x", runner.Spec{}); err == nil {
		t.Error("start succeeded without a log")
	}
}

// A viewer that stops reading is dropped; the session and the other viewers
// carry on.
func TestSlowViewerIsDroppedWithoutStallingTheSession(t *testing.T) {
	f := newFixture()
	s := f.start(t, "x")
	slow := f.attach(t, "x") // never reads
	fast := f.attach(t, "x")
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < viewerBuffer*2; i++ {
			s.print(t, "y")
		}
		s.print(t, "END")
	}()
	collect(t, fast, "END")
	<-done
	// The slow viewer's channel was closed after its buffer filled up.
	n := 0
	for range slow.Output() {
		n++
	}
	if n != viewerBuffer {
		t.Errorf("slow viewer got %d chunks before being dropped, want %d", n, viewerBuffer)
	}
}

func TestCloseDetaches(t *testing.T) {
	f := newFixture()
	s := f.start(t, "x")
	a := f.attach(t, "x")
	a.Close()
	a.Close() // idempotent
	s.print(t, "after")
	if _, ok := <-a.Output(); ok {
		t.Error("closed viewer still received output")
	}
}

func TestListIsSortedAndIncludesExitedSessions(t *testing.T) {
	f := newFixture()
	if got := f.sup.List(); len(got) != 0 {
		t.Errorf("empty supervisor lists %v", got)
	}
	f.start(t, "tove@ledger")
	s := f.start(t, "mara@ledger")
	a := f.attach(t, "mara@ledger")
	s.exit(0)
	waitClosed(t, a)
	if got := strings.Join(f.sup.List(), ","); got != "mara@ledger,tove@ledger" {
		t.Errorf("List = %s", got)
	}
}
