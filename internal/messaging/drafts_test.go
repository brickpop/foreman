package messaging

import (
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/brickpop/foreman/internal/events"
)

// draftFixture: Mara (idle) is where the CEO writes; Tove asks her things.
// The countdown is short: 150 ms, counted after 40 ms without a key.
type draftFixture struct {
	*fixture
	mu     sync.Mutex
	drafts []Draft
}

func newDraftFixture(t *testing.T) *draftFixture {
	f := &draftFixture{fixture: newFixture(t)}
	f.m.Hold, f.m.Quiet, f.m.Tick = 150*time.Millisecond, 40*time.Millisecond, 5*time.Millisecond
	f.m.Publish = func(e events.Event) {
		if d, ok := e.Data.(Draft); ok {
			f.mu.Lock()
			f.drafts = append(f.drafts, d)
			f.mu.Unlock()
		}
	}
	return f
}

func (f *draftFixture) lastDraft() Draft {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.drafts) == 0 {
		return Draft{}
	}
	return f.drafts[len(f.drafts)-1]
}

func (f *draftFixture) ask(request string) {
	f.t.Helper()
	if res := f.call("tove@ledger", "ask", map[string]string{"teammate": "mara", "request": request}); res.Error {
		f.t.Fatal(res.Text)
	}
}

// still says nothing was typed into Mara's session for a while.
func (f *draftFixture) still(d time.Duration) {
	f.t.Helper()
	time.Sleep(d)
	if got := f.s.of("mara@ledger"); got != "" {
		f.t.Fatalf("typed onto the CEO's draft: %q", got)
	}
}

func TestTypedTracksTheDraft(t *testing.T) {
	m := New(Deps{})
	length := func() int {
		m.mu.Lock()
		defer m.mu.Unlock()
		return m.draft("x").length
	}
	for _, tc := range []struct {
		keys string
		want int
	}{
		{"héllo", 5},              // characters, not bytes
		{"\x7f\x7f", 3},           // Backspace
		{"\x1b[D\x1b[A\x1bOB", 3}, // arrows change nothing
		{"\x1b[200~pasted\x1b[201~", 9},
		{"\t", 9},       // other controls neither
		{"\x15", 0},     // Ctrl+U clears
		{"ab\x03", 0},   // Ctrl+C clears
		{"ab\r", 0},     // Enter sends
		{"one\rtwo", 3}, // a new draft after the Enter
		{"\x7f\x7f\x7f\x7f", 0},
	} {
		m.Typed("x", []byte(tc.keys))
		if got := length(); got != tc.want || m.Drafting("x") != (tc.want > 0) {
			t.Errorf("after %q: %d (drafting %v), want %d", tc.keys, got, m.Drafting("x"), tc.want)
		}
	}
	if m.Drafting("nobody") {
		t.Error("a draft in a prompt nobody typed into")
	}
}

// A message waits while the CEO has a draft; clearing the line lets it go.
func TestAMessageWaitsOnTheDraftUntilCleared(t *testing.T) {
	f := newDraftFixture(t)
	f.m.Typed("mara@ledger", []byte("Tic, plea"))
	f.ask("build cal")
	f.eventually("the notice", func() bool { return f.lastDraft().Waiting == 1 })
	if f.state("msg-1") != "queued" {
		t.Errorf("state %s", f.state("msg-1"))
	}
	f.still(80 * time.Millisecond)
	f.m.Typed("mara@ledger", []byte("\x15"))
	f.eventually("delivered once cleared", func() bool { return strings.HasPrefix(f.s.of("mara@ledger"), "Message msg-1 from Tove") })
	if d := f.lastDraft(); d.Waiting != 0 {
		t.Errorf("the notice stayed: %+v", d)
	}
}

// Sending the draft sends it first: the message follows.
func TestSendingTheDraftLetsTheMessageFollow(t *testing.T) {
	f := newDraftFixture(t)
	f.m.Typed("mara@ledger", []byte("Tic, please"))
	f.ask("build cal")
	f.eventually("the notice", func() bool { return f.lastDraft().Waiting == 1 })
	f.m.Typed("mara@ledger", []byte("\r"))
	// The Enter started nothing (Mara stays idle in this fixture): after the
	// quiet time the message goes, on a clean line.
	f.eventually("delivered after the send", func() bool { return strings.HasPrefix(f.s.of("mara@ledger"), "Message msg-1") })
}

// Once the CEO's line went out (a paused countdown, sent from the Line input,
// found 2026-10-30), the notice stays until the messages are delivered at the
// agent's next idle, counting any that came meanwhile: it never just vanishes.
func TestTheNoticeStaysUntilTheHeldMessagesAreDelivered(t *testing.T) {
	f := newDraftFixture(t)
	f.m.Typed("mara@ledger", []byte("half a th"))
	f.ask("build cal")
	f.eventually("the notice", func() bool { return f.lastDraft().Waiting == 1 })
	f.m.PauseCountdown("mara@ledger", true)
	if err := f.m.Line("mara@ledger", "ought"); err != nil {
		t.Fatal(err)
	}
	f.setStatus("mara@ledger", "running") // the line started her turn
	if d := f.lastDraft(); d.Waiting != 1 || d.OnDraft || d.Paused || d.Counting {
		t.Errorf("after the send: %+v", d)
	}
	f.ask("and tests")
	f.eventually("counted", func() bool { return f.lastDraft().Waiting == 2 })
	time.Sleep(80 * time.Millisecond) // past the quiet time: she's still busy
	if got := f.s.of("mara@ledger"); strings.Contains(got, "msg-") || f.lastDraft().Waiting != 2 {
		t.Fatalf("typed %q, notice %+v", got, f.lastDraft())
	}
	f.setStatus("mara@ledger", "idle")
	f.m.StatusChanged("mara@ledger", "idle")
	f.eventually("delivered", func() bool { return strings.Contains(f.s.of("mara@ledger"), "msg-2") })
	f.eventually("the notice goes", func() bool { return f.lastDraft().Waiting == 0 })
}

// A notice for messages that went another way (the deployment stopped) goes.
func TestTheNoticeGoesWhenNothingWaitsAnyMore(t *testing.T) {
	f := newDraftFixture(t)
	f.m.Typed("mara@ledger", []byte("draft"))
	f.ask("build cal")
	f.eventually("the notice", func() bool { return f.lastDraft().Waiting == 1 })
	f.setStatus("mara@ledger", "running")
	f.m.Typed("mara@ledger", []byte("\r"))
	if _, err := f.db.Messages().Expire(ctx, "atlas@ledger", "deployment_stopped"); err != nil {
		t.Fatal(err)
	}
	f.m.Deliver("mara@ledger")
	if d := f.lastDraft(); d.Waiting != 0 {
		t.Errorf("%+v", d)
	}
}

// The countdown runs only while the CEO isn't typing; at zero the message
// goes after the draft, marked as unfinished.
func TestTheCountdownWaitsForTheCEOThenAppends(t *testing.T) {
	f := newDraftFixture(t)
	f.m.Typed("mara@ledger", []byte("Tic, ple"))
	f.ask("build cal")
	f.eventually("the notice", func() bool { return f.lastDraft().Waiting == 1 })
	// Typing for longer than the whole countdown keeps it from running out.
	for range 30 {
		f.m.Typed("mara@ledger", []byte("a"))
		time.Sleep(10 * time.Millisecond)
	}
	if got := f.s.of("mara@ledger"); got != "" {
		t.Fatalf("delivered while the CEO typed: %q", got)
	}
	if d := f.lastDraft(); d.Counting {
		t.Errorf("counting while typing: %+v", d)
	}
	f.eventually("delivered at zero", func() bool { return strings.HasSuffix(f.s.of("mara@ledger"), "\r") && f.s.of("mara@ledger") != "" })
	nl := "\\\r"
	want := nl + nl + "---" + nl + nl + draftNote + nl + nl + "Message msg-1 from Tove (ask): build cal"
	if got := f.s.of("mara@ledger"); !strings.HasPrefix(got, want) || strings.Count(got, "\r") != 7 {
		t.Errorf("typed %q", got)
	}
	// The draft went with it: the next message isn't held.
	f.ask("and tests")
	f.eventually("the next one", func() bool { return strings.Contains(f.s.of("mara@ledger"), "msg-2") })
}

func TestAPausedCountdownWaitsUntilResumed(t *testing.T) {
	f := newDraftFixture(t)
	f.m.Typed("mara@ledger", []byte("Tic"))
	f.ask("build cal")
	f.eventually("the notice", func() bool { return f.lastDraft().Waiting == 1 })
	f.m.PauseCountdown("mara@ledger", true)
	if d := f.lastDraft(); !d.Paused || d.Counting {
		t.Errorf("%+v", d)
	}
	f.still(400 * time.Millisecond) // well past the countdown
	f.m.PauseCountdown("mara@ledger", false)
	f.eventually("delivered once resumed", func() bool { return strings.Contains(f.s.of("mara@ledger"), draftNote) })
}

// The CEO's line goes out with a pause before each newline and the Enter,
// so the harness sends it; a message held on a draft follows.
func TestTheLineIsTypedThenSent(t *testing.T) {
	f := newDraftFixture(t)
	if err := f.m.Line("mara@ledger", "two\r\nlines"); err != nil {
		t.Fatal(err)
	}
	if got := f.s.of("mara@ledger"); got != "two\\\rlines\r" {
		t.Errorf("typed %q", got)
	}
	f.s.mu.Lock()
	f.s.typed["mara@ledger"] = ""
	f.s.mu.Unlock()
	f.m.Typed("mara@ledger", []byte("half a th"))
	f.ask("build cal")
	f.eventually("the notice", func() bool { return f.lastDraft().Waiting == 1 })
	if err := f.m.Line("mara@ledger", ""); err != nil { // an Enter: sends the draft
		t.Fatal(err)
	}
	f.eventually("the message follows", func() bool { return strings.HasPrefix(f.s.of("mara@ledger"), "\rMessage msg-1") })
}

// A session that starts again has an empty prompt: its old draft holds nothing.
func TestARestartForgetsTheDraft(t *testing.T) {
	f := newDraftFixture(t)
	f.m.Typed("mara@ledger", []byte("draft"))
	f.m.StatusChanged("mara@ledger", "starting")
	f.ask("build cal")
	f.eventually("delivered at once", func() bool { return strings.HasPrefix(f.s.of("mara@ledger"), "Message msg-1") })
}
