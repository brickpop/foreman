package terminal

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/brickpop/foreman/internal/runner"
)

// --- doubles -----------------------------------------------------------------

// fakeProcess's output is whatever the test writes to it; it exits when the
// test calls exit.
type fakeProcess struct {
	outR *io.PipeReader
	outW *io.PipeWriter

	mu     sync.Mutex
	input  bytes.Buffer
	sizes  []runner.Size
	code   int
	exited chan struct{}
}

func newFakeProcess() *fakeProcess {
	r, w := io.Pipe()
	return &fakeProcess{outR: r, outW: w, exited: make(chan struct{})}
}

func (f *fakeProcess) print(t *testing.T, s string) {
	t.Helper()
	if _, err := f.outW.Write([]byte(s)); err != nil {
		t.Fatal(err)
	}
}

func (f *fakeProcess) exit(code int) {
	f.mu.Lock()
	f.code = code
	f.mu.Unlock()
	_ = f.outW.Close()
	close(f.exited)
}

func (f *fakeProcess) Read(p []byte) (int, error) { return f.outR.Read(p) }
func (f *fakeProcess) Write(p []byte) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.input.Write(p)
}
func (f *fakeProcess) Labels() map[string]string { return nil }
func (f *fakeProcess) Resize(s runner.Size) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sizes = append(f.sizes, s)
	return nil
}
func (f *fakeProcess) Stop(time.Duration) error { return nil }
func (f *fakeProcess) Wait() (int, error) {
	<-f.exited
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.code, nil
}

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

// start returns a terminal on a fake process, its process and its log.
func start(t *testing.T) (*Terminal, *fakeProcess, *memLog) {
	t.Helper()
	p, l := newFakeProcess(), &memLog{}
	return New("test", p, l), p, l
}

func attach(t *testing.T, term *Terminal) *View {
	t.Helper()
	v, err := term.Attach()
	if err != nil {
		t.Fatal(err)
	}
	return v
}

// collect gathers a viewer's live output until it contains want.
func collect(t *testing.T, v *View, want string) string {
	t.Helper()
	var got strings.Builder
	deadline := time.After(3 * time.Second)
	for !strings.Contains(got.String(), want) {
		select {
		case chunk, ok := <-v.Output():
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

func waitClosed(t *testing.T, v *View) {
	t.Helper()
	deadline := time.After(3 * time.Second)
	for {
		select {
		case _, ok := <-v.Output():
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
	term, p, log := start(t)
	a, b := attach(t, term), attach(t, term)
	p.print(t, "hello \x1b[32mworld\x1b[0m")
	for _, v := range []*View{a, b} {
		if got := collect(t, v, "world\x1b[0m"); got != "hello \x1b[32mworld\x1b[0m" {
			t.Errorf("viewer got %q", got)
		}
	}
	if got, _ := log.Tail(100); string(got) != "hello \x1b[32mworld\x1b[0m" {
		t.Errorf("log has %q", got)
	}
}

// A viewer attaching mid-stream sees the earlier output once (snapshot) and
// the rest live: no gap, no duplicate.
func TestLateViewerGetsSnapshotThenLiveWithoutGapOrDuplicate(t *testing.T) {
	term, p, _ := start(t)
	early := attach(t, term)
	p.print(t, "one ")
	collect(t, early, "one")
	late := attach(t, term)
	p.print(t, "two")
	live := collect(t, late, "two")
	if got := string(late.Snapshot()) + live; got != "one two" {
		t.Errorf("late viewer saw %q, want %q", got, "one two")
	}
}

func TestInputAndResizeReachTheProcess(t *testing.T) {
	term, p, _ := start(t)
	if err := term.Input([]byte("ls\r")); err != nil {
		t.Fatal(err)
	}
	if err := term.Resize(runner.Size{Cols: 100, Rows: 30}); err != nil {
		t.Fatal(err)
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.input.String() != "ls\r" || len(p.sizes) != 1 || p.sizes[0] != (runner.Size{Cols: 100, Rows: 30}) {
		t.Errorf("process got input %q, sizes %v", p.input.String(), p.sizes)
	}
}

func TestExitEndsViewersAndIsReported(t *testing.T) {
	term, p, log := start(t)
	v := attach(t, term)
	if _, exited := term.ExitCode(); exited {
		t.Error("exited before the process did")
	}
	p.print(t, "last words")
	collect(t, v, "last words")
	p.exit(3)
	waitClosed(t, v)
	select {
	case <-term.Done():
	case <-time.After(3 * time.Second):
		t.Fatal("Done never closed")
	}
	if code, exited := term.ExitCode(); !exited || code != 3 {
		t.Errorf("ExitCode = %d, %v; want 3, true", code, exited)
	}
	if err := term.Input([]byte("x")); !errors.Is(err, ErrExited) {
		t.Errorf("input after exit: %v", err)
	}
	if err := term.Resize(runner.Size{Cols: 1, Rows: 1}); !errors.Is(err, ErrExited) {
		t.Errorf("resize after exit: %v", err)
	}
	if !log.isClosed() {
		t.Error("log left open after exit")
	}
	// Attaching after the exit still shows what happened.
	after := attach(t, term)
	if string(after.Snapshot()) != "last words" {
		t.Errorf("snapshot after exit %q", after.Snapshot())
	}
	waitClosed(t, after)
}

// A viewer that stops reading is dropped; the process and the other viewers
// carry on.
func TestSlowViewerIsDroppedWithoutStallingTheProcess(t *testing.T) {
	term, p, _ := start(t)
	slow := attach(t, term) // never reads
	fast := attach(t, term)
	// The fast viewer reads each chunk as it comes, so only the slow one lags.
	for i := 0; i < viewerBuffer*2; i++ {
		p.print(t, "y")
		collect(t, fast, "y")
	}
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
	term, p, _ := start(t)
	v := attach(t, term)
	v.Close()
	v.Close() // idempotent
	p.print(t, "after")
	if _, ok := <-v.Output(); ok {
		t.Error("closed viewer still received output")
	}
}

func TestSnapshotFailureIsReported(t *testing.T) {
	term := New("test", newFakeProcess(), failingLog{})
	if _, err := term.Attach(); err == nil {
		t.Error("attach succeeded without a snapshot")
	}
}

type failingLog struct{}

func (failingLog) Write(p []byte) (int, error) { return len(p), nil }
func (failingLog) Tail(int64) ([]byte, error)  { return nil, errors.New("disk gone") }
func (failingLog) Close() error                { return nil }
