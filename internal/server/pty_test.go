package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/brickpop/foreman/internal/runner"
)

// fakeTerminal is a Terminal the test drives: it pushes output and ends it.
type fakeTerminal struct {
	snapshot []byte
	out      chan []byte

	mu     sync.Mutex
	input  []string
	sizes  []runner.Size
	code   int
	exited bool
	closed chan struct{}
	once   sync.Once
}

func newFakeTerminal(snapshot string) *fakeTerminal {
	return &fakeTerminal{snapshot: []byte(snapshot), out: make(chan []byte, 16), closed: make(chan struct{})}
}

func (f *fakeTerminal) Snapshot() []byte      { return f.snapshot }
func (f *fakeTerminal) Output() <-chan []byte { return f.out }
func (f *fakeTerminal) Exit() (int, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.code, f.exited
}
func (f *fakeTerminal) Input(p []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.input = append(f.input, string(p))
	return nil
}
func (f *fakeTerminal) Resize(s runner.Size) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sizes = append(f.sizes, s)
	return nil
}
func (f *fakeTerminal) Close() { f.once.Do(func() { close(f.closed) }) }

func (f *fakeTerminal) exit(code int) {
	f.mu.Lock()
	f.code, f.exited = code, true
	f.mu.Unlock()
	close(f.out)
}

type fakeTerminals map[string]*fakeTerminal

func (f fakeTerminals) Attach(id string) (Terminal, error) {
	if t, ok := f[id]; ok {
		return t, nil
	}
	return nil, ErrNoTerminal
}

func ptyServer(t *testing.T, terms fakeTerminals) string {
	t.Helper()
	srv := httptest.NewServer(New(Options{Guard: openGuard{}, Workspace: fakeWorkspace("/ws"), Terminals: terms, UI: builtUI}))
	t.Cleanup(srv.Close)
	return "ws" + strings.TrimPrefix(srv.URL, "http")
}

func dial(t *testing.T, url string) *websocket.Conn {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.CloseNow() })
	return conn
}

func read(t *testing.T, conn *websocket.Conn) (websocket.MessageType, string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	kind, data, err := conn.Read(ctx)
	if err != nil {
		t.Fatal(err)
	}
	return kind, string(data)
}

func eventually(t *testing.T, what string, cond func() bool) {
	t.Helper()
	for deadline := time.Now().Add(3 * time.Second); time.Now().Before(deadline); time.Sleep(5 * time.Millisecond) {
		if cond() {
			return
		}
	}
	t.Fatalf("never: %s", what)
}

func TestPTYUnknownInstanceIs404(t *testing.T) {
	url := ptyServer(t, fakeTerminals{})
	_, resp, err := websocket.Dial(context.Background(), url+"/ws/pty/nobody", nil)
	if err == nil || resp == nil || resp.StatusCode != http.StatusNotFound {
		t.Errorf("got err=%v resp=%v, want a 404 before the upgrade", err, resp)
	}
}

func TestPTYStreamsSnapshotThenLiveThenExit(t *testing.T) {
	term := newFakeTerminal("earlier \x1b[1moutput\x1b[0m")
	conn := dial(t, ptyServer(t, fakeTerminals{"mara@ledger": term})+"/ws/pty/mara@ledger")

	if kind, data := read(t, conn); kind != websocket.MessageBinary || data != "earlier \x1b[1moutput\x1b[0m" {
		t.Fatalf("first frame %v %q, want the snapshot as binary", kind, data)
	}
	term.out <- []byte("live")
	if _, data := read(t, conn); data != "live" {
		t.Fatalf("live frame %q", data)
	}
	term.exit(7)
	kind, data := read(t, conn)
	var c control
	if kind != websocket.MessageText || json.Unmarshal([]byte(data), &c) != nil || c.Type != "exit" || c.Code == nil || *c.Code != 7 {
		t.Fatalf("exit frame %v %q", kind, data)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, _, err := conn.Read(ctx); websocket.CloseStatus(err) != websocket.StatusNormalClosure {
		t.Errorf("after exit: %v, want a normal close", err)
	}
}

func TestPTYEmptySnapshotSendsNothingFirst(t *testing.T) {
	term := newFakeTerminal("")
	conn := dial(t, ptyServer(t, fakeTerminals{"x": term})+"/ws/pty/x")
	term.out <- []byte("first")
	if _, data := read(t, conn); data != "first" {
		t.Errorf("first frame %q: an empty snapshot must not send an empty frame", data)
	}
}

func TestPTYInputAndResizeReachTheTerminal(t *testing.T) {
	term := newFakeTerminal("")
	conn := dial(t, ptyServer(t, fakeTerminals{"x": term})+"/ws/pty/x")
	ctx := context.Background()
	_ = conn.Write(ctx, websocket.MessageBinary, []byte("ls\r"))
	_ = conn.Write(ctx, websocket.MessageText, []byte(`{"type":"resize","cols":132,"rows":43}`))
	// Garbage and zero sizes are ignored, and don't end the connection.
	_ = conn.Write(ctx, websocket.MessageText, []byte(`not json`))
	_ = conn.Write(ctx, websocket.MessageText, []byte(`{"type":"resize","cols":0,"rows":10}`))
	_ = conn.Write(ctx, websocket.MessageBinary, []byte("pwd\r"))
	eventually(t, "input and one resize arrive", func() bool {
		term.mu.Lock()
		defer term.mu.Unlock()
		return len(term.input) == 2
	})
	term.mu.Lock()
	defer term.mu.Unlock()
	if strings.Join(term.input, "") != "ls\rpwd\r" || len(term.sizes) != 1 || term.sizes[0] != (runner.Size{Cols: 132, Rows: 43}) {
		t.Errorf("input %q, sizes %v", term.input, term.sizes)
	}
}

func TestPTYViewerThatFellBehindIsAskedToReconnect(t *testing.T) {
	term := newFakeTerminal("")
	conn := dial(t, ptyServer(t, fakeTerminals{"x": term})+"/ws/pty/x")
	close(term.out) // the terminal dropped this viewer; the session is still running
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, _, err := conn.Read(ctx); websocket.CloseStatus(err) != closeBehind {
		t.Errorf("got %v, want close status %d", err, closeBehind)
	}
}

func TestPTYDisconnectDetachesEvenWhenTheSessionIsQuiet(t *testing.T) {
	term := newFakeTerminal("")
	conn := dial(t, ptyServer(t, fakeTerminals{"x": term})+"/ws/pty/x")
	conn.CloseNow()
	select {
	case <-term.closed:
	case <-time.After(3 * time.Second):
		t.Fatal("viewer stayed attached after the browser left")
	}
}

func (f fakeTerminals) List() []string {
	ids := make([]string, 0, len(f))
	for id := range f {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func TestInstancesListsTheSessions(t *testing.T) {
	h := New(Options{Guard: openGuard{}, Workspace: fakeWorkspace("/ws"), UI: builtUI,
		Terminals: fakeTerminals{"tove@ledger": newFakeTerminal(""), "mara@ledger": newFakeTerminal("")}})
	w := get(t, h, "/api/instances")
	if w.Code != http.StatusOK || strings.TrimSpace(w.Body.String()) != `[{"id":"mara@ledger"},{"id":"tove@ledger"}]` {
		t.Errorf("got %d %q", w.Code, w.Body.String())
	}
	empty := New(Options{Guard: openGuard{}, Workspace: fakeWorkspace("/ws"), UI: builtUI, Terminals: fakeTerminals{}})
	if w := get(t, empty, "/api/instances"); strings.TrimSpace(w.Body.String()) != "[]" {
		t.Errorf("no sessions must be an empty list, not null: %q", w.Body.String())
	}
}
