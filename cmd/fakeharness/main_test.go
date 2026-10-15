package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/brickpop/foreman/internal/agentlink"
)

func TestFakeHarness(t *testing.T) {
	var out bytes.Buffer
	code := run("", strings.NewReader("hello\n\nspam 3\nexit 7\nnever reached\n"), &out, nil)
	if code != 7 {
		t.Errorf("exit code %d, want 7", code)
	}
	got := out.String()
	for _, want := range []string{"\x1b[32mready", "you said: hello", "line 1\nline 2\nline 3\n", "bye (7)"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in %q", want, got)
		}
	}
	if strings.Contains(got, "never reached") {
		t.Error("kept reading after exit")
	}
}

func TestFakeHarnessEndOfInput(t *testing.T) {
	if code := run("", strings.NewReader("hi\n"), &bytes.Buffer{}, nil); code != 0 {
		t.Errorf("exit code %d at end of input, want 0", code)
	}
}

// A trailing `\` continues the input on the next line, as in Claude Code.
func TestFakeHarnessContinuesLines(t *testing.T) {
	var out bytes.Buffer
	run("", strings.NewReader("first\\\nsecond\\\nthird\nalone\n"), &out, nil)
	got := out.String()
	if !strings.Contains(got, "you said: first\nsecond\nthird\n") || !strings.Contains(got, "you said: alone\n") {
		t.Errorf("got %q", got)
	}
	if strings.Count(got, "you said:") != 2 {
		t.Errorf("a continued line was answered on its own: %q", got)
	}
}

// The opener is answered before any input, as if typed.
func TestFakeHarnessAnswersTheOpener(t *testing.T) {
	var out bytes.Buffer
	run("Your goal on ledger: ship", strings.NewReader("hi\n"), &out, nil)
	got := out.String()
	opener, typed := strings.Index(got, "you said: Your goal on ledger: ship\n"), strings.Index(got, "you said: hi\n")
	if opener < 0 || typed < opener {
		t.Errorf("got %q", got)
	}
}

// linkRecorder is Foreman's side: the hooks in order, and one tool.
type linkRecorder struct {
	mu    sync.Mutex
	hooks []string
}

func (l *linkRecorder) List(string) []agentlink.Tool { return nil }

func (l *linkRecorder) Call(_ context.Context, _, name string, args json.RawMessage) agentlink.Result {
	if name != "ask" {
		return agentlink.Result{Text: "unknown_tool", Error: true}
	}
	return agentlink.Result{Text: "msg-1 sent " + string(args)}
}

func (l *linkRecorder) Hook(_, event string, _ []byte) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.hooks = append(l.hooks, event)
}

// With the agent link it reports its turns as Claude Code's hooks do, and
// calls team tools.
func TestFakeHarnessHooksAndTools(t *testing.T) {
	rec := &linkRecorder{}
	link := agentlink.New([]byte("s"), func(string) bool { return true }, rec, rec)
	srv := httptest.NewServer(link.Handler())
	defer srv.Close()
	var out bytes.Buffer
	run("go", strings.NewReader("tool ask {\"teammate\":\"tove\"}\ntool nope\ntool ask not-json\n"), &out,
		&agentlink.Client{URL: srv.URL, Token: link.Token("mara@ledger")})
	got := out.String()
	for _, want := range []string{`msg-1 sent {"teammate":"tove"}`, "tool error: unknown_tool", "tool: the arguments aren't JSON"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in %q", want, got)
		}
	}
	want := "SessionStart UserPromptSubmit Stop" + strings.Repeat(" UserPromptSubmit Stop", 3)
	if strings.Join(rec.hooks, " ") != want {
		t.Errorf("hooks %v", rec.hooks)
	}
}
