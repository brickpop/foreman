package messaging

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/brickpop/foreman/internal/agentlink"
	"github.com/brickpop/foreman/internal/home"
	"github.com/brickpop/foreman/internal/prompt"
	"github.com/brickpop/foreman/internal/store"
	"github.com/brickpop/foreman/internal/teamtools"
)

var ctx = context.Background()

var atlas = home.Team{ID: "atlas", Config: home.TeamConfig{
	Team: "atlas", Name: "Atlas", Lead: "mara", Order: []string{"mara", "tove", "ivo"},
	Agents: map[string]home.Soul{
		"mara": {Name: "Mara", Role: "team_lead"},
		"tove": {Name: "Tove", Role: "engineer"},
		"ivo":  {Name: "Ivo", Role: "engineer"},
	},
}}

type config struct{}

func (config) Team(id string) (home.Team, error) {
	if id != "atlas" {
		return home.Team{}, home.ErrNotFound
	}
	return atlas, nil
}

// Project: every project has a code repo, so every tool is offered.
func (config) Project(id string) (home.Project, error) {
	return home.Project{ID: id, Repos: []home.Repo{{Name: "api", Default: "main"}}}, nil
}

// sessions records what is typed into each instance and who was woken.
type sessions struct {
	mu    sync.Mutex
	typed map[string]string
	woken []string
	fail  bool
}

func (s *sessions) Wake(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.woken = append(s.woken, id)
	return nil
}

func (s *sessions) Type(id string, p []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.fail {
		return errors.New("no terminal")
	}
	s.typed[id] += string(p)
	return nil
}

func (s *sessions) wokenList() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.woken...)
}

func (s *sessions) Newline(string) string { return "\\\r" }

func (s *sessions) setFail(fail bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.fail = fail
}

func (s *sessions) of(id string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.typed[id]
}

type fixture struct {
	t  *testing.T
	db *store.DB
	s  *sessions
	m  *Messenger
	r  *teamtools.Router // the tools as agents reach them
}

// newFixture: atlas on ledger; mara idle, tove running, ivo asleep.
func newFixture(t *testing.T) *fixture {
	t.Helper()
	db, err := store.Open(ctx, filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.Deployments().Deploy(ctx, "atlas", "ledger"); err != nil {
		t.Fatal(err)
	}
	f := &fixture{t: t, db: db, s: &sessions{typed: map[string]string{}}}
	for agent, status := range map[string]string{"mara": "idle", "tove": "running", "ivo": "asleep"} {
		f.setStatus(agent+"@ledger", status)
	}
	f.m = New(Deps{Messages: db.Messages(), Instances: db.Instances(), Deployments: db.Deployments(), Config: config{}, Sessions: f.s})
	f.r = &teamtools.Router{Instances: db.Instances(), Deployments: db.Deployments(), Config: config{}, Blocks: []teamtools.Block{f.m}}
	return f
}

func (f *fixture) setStatus(id, status string) {
	f.t.Helper()
	agent, _, _ := strings.Cut(id, "@")
	if err := f.db.Instances().Put(ctx, store.Instance{ID: id, DeploymentID: "atlas@ledger", AgentID: agent, Status: status, SandboxPath: "/s", Harness: "claude-code"}); err != nil {
		f.t.Fatal(err)
	}
}

func (f *fixture) call(from, tool string, args map[string]string) agentlink.Result {
	f.t.Helper()
	raw, _ := json.Marshal(args)
	return f.r.Call(ctx, from, tool, raw)
}

// eventually waits for the deliveries started in the background.
func (f *fixture) eventually(what string, ok func() bool) {
	f.t.Helper()
	for deadline := time.Now().Add(3 * time.Second); time.Now().Before(deadline); time.Sleep(5 * time.Millisecond) {
		if ok() {
			return
		}
	}
	f.t.Fatalf("never: %s", what)
}

func (f *fixture) state(ref string) string {
	f.t.Helper()
	msg, err := f.db.Messages().Get(ctx, "atlas@ledger", ref)
	if err != nil {
		f.t.Fatal(err)
	}
	return msg.State
}

// Each tool is defined well enough for a harness to call it; that the router
// serves exactly what the prompt describes is teamtools' test.
func TestToolDefinitions(t *testing.T) {
	for _, tool := range newFixture(t).m.Tools() {
		if !json.Valid(tool.InputSchema) || tool.Description == "" || !slices.Contains(prompt.Offered(false, true), tool.Name) {
			t.Errorf("%s: a bad definition, or a tool no one is offered", tool.Name)
		}
	}
}

// An ask to an idle teammate is typed into its session at once, as one plain line.
func TestAnAskReachesAnIdleTeammateAsOneLine(t *testing.T) {
	f := newFixture(t)
	res := f.call("tove@ledger", "ask", map[string]string{"teammate": "MARA", "request": "Which month first?\nThe spec says October."})
	if res.Error || !strings.HasPrefix(res.Text, "msg-1 sent to Mara.") {
		t.Fatalf("ask: %+v", res)
	}
	want := `Message msg-1 from Tove (ask): Which month first? The spec says October. Read it with read_message("msg-1"); answer with reply("msg-1", output).` + "\r"
	f.eventually("delivered to mara", func() bool { return f.s.of("mara@ledger") == want })
	if f.state("msg-1") != store.MessageDelivered {
		t.Errorf("state %s", f.state("msg-1"))
	}
	if line := strings.TrimSuffix(f.s.of("mara@ledger"), "\r"); strings.ContainsAny(line, "\r\n⇄✓✗ⓘ⚠…") {
		t.Errorf("not one plain line: %q", line)
	}
}

// A busy teammate gets it at its next idle; several queued arrive as one line.
func TestABusyTeammateGetsABurstAtItsNextIdle(t *testing.T) {
	f := newFixture(t)
	f.call("mara@ledger", "ask", map[string]string{"teammate": "tove", "request": "build cal"})
	f.call("mara@ledger", "ask", map[string]string{"teammate": "Tove", "request": "and its tests"})
	time.Sleep(50 * time.Millisecond)
	if typed := f.s.of("tove@ledger"); typed != "" {
		t.Fatalf("typed into a busy session: %q", typed)
	}
	f.setStatus("tove@ledger", "idle")
	f.m.StatusChanged("tove@ledger", "idle")
	want := "2 new messages: msg-1 from Mara (ask), msg-2 from Mara (ask). Read each with read_message.\r"
	f.eventually("the burst", func() bool { return f.s.of("tove@ledger") == want })
	// Another idle finds nothing left: nothing typed twice.
	f.m.Deliver("tove@ledger")
	if got := f.s.of("tove@ledger"); got != want {
		t.Errorf("typed again: %q", got)
	}
}

func TestAnAsleepTeammateIsWokenThenGetsItsMessage(t *testing.T) {
	f := newFixture(t)
	f.call("mara@ledger", "ask", map[string]string{"teammate": "ivo", "request": "review the spec"})
	f.eventually("ivo woken", func() bool {
		f.s.mu.Lock()
		defer f.s.mu.Unlock()
		return slices.Equal(f.s.woken, []string{"ivo@ledger"})
	})
	if f.s.of("ivo@ledger") != "" || f.state("msg-1") != store.MessageQueued {
		t.Fatal("typed into a session that isn't up")
	}
	f.setStatus("ivo@ledger", "idle")
	f.m.StatusChanged("ivo@ledger", "idle")
	f.eventually("delivered after waking", func() bool { return strings.HasPrefix(f.s.of("ivo@ledger"), "Message msg-1 from Mara (ask)") })
}

func TestAReplyGoesBackAndReadsInFull(t *testing.T) {
	f := newFixture(t)
	f.call("mara@ledger", "ask", map[string]string{"teammate": "tove", "request": "build cal"})
	long := strings.Repeat("word ", 40) + "\nEND"
	res := f.call("tove@ledger", "reply", map[string]string{"request_id": "msg-1", "output": long, "artifact": "wt/tove/cal"})
	if res.Error || res.Text != "msg-2 sent to Mara." {
		t.Fatalf("reply: %+v", res)
	}
	f.eventually("the reply's notice", func() bool { return f.s.of("mara@ledger") != "" })
	notice := f.s.of("mara@ledger")
	if !strings.HasPrefix(notice, "Message msg-2 from Tove (reply to msg-1): word word") ||
		!strings.Contains(notice, `... Read it with read_message("msg-2"), then resume the task that asked for it.`) {
		t.Errorf("notice %q", notice)
	}
	read := f.call("mara@ledger", "read_message", map[string]string{"id": "msg-2"})
	if read.Error || !strings.Contains(read.Text, "msg-2 from Tove (reply to msg-1)") || !strings.Contains(read.Text, long) ||
		!strings.Contains(read.Text, "Artifact: wt/tove/cal") {
		t.Errorf("read %q", read.Text)
	}
	ask := f.call("tove@ledger", "read_message", map[string]string{"id": "msg-1"})
	if !strings.Contains(ask.Text, `Answer it with reply("msg-1", output).`) {
		t.Errorf("an ask's read says how to answer: %q", ask.Text)
	}
}

// Every refusal says why, and changes nothing.
func TestRefusals(t *testing.T) {
	f := newFixture(t)
	f.call("mara@ledger", "ask", map[string]string{"teammate": "tove", "request": "build cal"})
	for _, tc := range []struct {
		from, tool string
		args       map[string]string
		code       string
	}{
		{"mara@ledger", "ask", map[string]string{"teammate": "rhea", "request": "x"}, "unknown_teammate: no \"rhea\" on this team; your teammates: Tove, Ivo"},
		{"mara@ledger", "ask", map[string]string{"teammate": "Mara", "request": "x"}, "self:"},
		{"mara@ledger", "ask", map[string]string{"teammate": "tove", "request": "  "}, "empty:"},
		{"mara@ledger", "ask", map[string]string{"teammate": "tove", "request": strings.Repeat("x", teamtools.MaxBody+1)}, "too_long:"},
		{"mara@ledger", "handoff", map[string]string{"teammate": "tove"}, "unknown_tool:"},
		{"tove@ledger", "reply", map[string]string{"request_id": "msg-9", "output": "x"}, "unknown_request:"},
		{"ivo@ledger", "reply", map[string]string{"request_id": "msg-1", "output": "x"}, "not_addressee:"},
		// The asker answering its own ask is told a reply ends it.
		{"mara@ledger", "reply", map[string]string{"request_id": "msg-1", "output": "thanks"}, "not_addressee: msg-1 is your own ask to Tove; a reply comes from them and ends it."},
		{"tove@ledger", "reply", map[string]string{"request_id": "msg-1", "output": "x", "artifact": "../../etc"}, "bad_artifact:"},
		{"tove@ledger", "reply", map[string]string{"request_id": "msg-1", "output": ""}, "empty:"},
		{"ivo@ledger", "read_message", map[string]string{"id": "msg-1"}, "unknown_message:"},
		{"ivo@ledger", "read_message", map[string]string{"id": "msg-99"}, "unknown_message:"},
		{"nobody@ledger", "ask", map[string]string{"teammate": "tove", "request": "x"}, "unknown_caller:"},
	} {
		res := f.call(tc.from, tc.tool, tc.args)
		if !res.Error || !strings.HasPrefix(res.Text, tc.code) {
			t.Errorf("%s %s %v: %+v", tc.from, tc.tool, tc.args, res)
		}
	}
	if list, _ := f.db.Messages().List(ctx, "atlas@ledger"); len(list) != 1 {
		t.Errorf("a refusal stored a message: %d", len(list))
	}
	// One reply per ask.
	f.call("tove@ledger", "reply", map[string]string{"request_id": "msg-1", "output": "done"})
	if res := f.call("tove@ledger", "reply", map[string]string{"request_id": "msg-1", "output": "more"}); !strings.HasPrefix(res.Text, "already_replied:") {
		t.Errorf("a second reply: %+v", res)
	}
	// A stopped deployment takes no new messages, and what waited expires.
	f.call("mara@ledger", "ask", map[string]string{"teammate": "tove", "request": "one more"})
	if err := f.db.Deployments().Stop(ctx, "atlas@ledger", ""); err != nil {
		t.Fatal(err)
	}
	f.m.DeploymentStopped("atlas@ledger")
	if s := f.state("msg-3"); s != store.MessageExpired {
		t.Errorf("a queued message after the stop: %s", s)
	}
	if res := f.call("mara@ledger", "ask", map[string]string{"teammate": "tove", "request": "x"}); !strings.HasPrefix(res.Text, "deployment_stopped:") {
		t.Errorf("after the stop: %+v", res)
	}
}

// A delivery that can't be typed waits for the next idle.
func TestAFailedDeliveryStaysQueued(t *testing.T) {
	f := newFixture(t)
	f.s.setFail(true)
	f.call("tove@ledger", "ask", map[string]string{"teammate": "mara", "request": "x"})
	time.Sleep(50 * time.Millisecond)
	f.m.Deliver("mara@ledger")
	if s := f.state("msg-1"); s != store.MessageQueued {
		t.Fatalf("after a failed delivery: %s", s)
	}
	f.s.setFail(false)
	if err := f.m.Sweep(ctx); err != nil {
		t.Fatal(err)
	}
	if s := f.state("msg-1"); s != store.MessageDelivered || !strings.HasPrefix(f.s.of("mara@ledger"), "Message msg-1") {
		t.Errorf("after the sweep: %s %q", s, f.s.of("mara@ledger"))
	}
}

func TestPreviewIsOneLineCutOnARune(t *testing.T) {
	body := strings.Repeat("é", previewLength) + "tail"
	if got := preview(body); got != strings.Repeat("é", previewLength)+"..." {
		t.Errorf("cut: %q", got)
	}
	if got := preview("a\n\n  b\tc"); got != "a b c" {
		t.Errorf("flattened: %q", got)
	}
}

// An agent that goes to sleep with messages waiting (a clean exit mid-turn)
// is woken for them; a stopped one waits for the CEO's Start again.
func TestWaitingMessagesWakeASleeperNotAStoppedAgent(t *testing.T) {
	f := newFixture(t)
	f.call("mara@ledger", "ask", map[string]string{"teammate": "tove", "request": "build cal"}) // tove is running: queued
	f.setStatus("tove@ledger", "stopped")
	f.m.StatusChanged("tove@ledger", "stopped")
	time.Sleep(50 * time.Millisecond)
	if woken := f.s.wokenList(); len(woken) != 0 {
		t.Errorf("a stopped agent was woken: %v", woken)
	}
	f.setStatus("tove@ledger", "asleep")
	f.m.StatusChanged("tove@ledger", "asleep")
	f.eventually("woken", func() bool { return len(f.s.wokenList()) == 1 })
}

// Messages waiting when Foreman exited (its agents asleep) wake their agent
// on the next start, through the startup delivery.
func TestTheSweepWakesASleeperWithMessagesWaiting(t *testing.T) {
	f := newFixture(t)
	f.call("mara@ledger", "ask", map[string]string{"teammate": "tove", "request": "build cal"}) // tove is running: queued
	time.Sleep(50 * time.Millisecond)                                                           // its delivery found tove running
	f.setStatus("tove@ledger", "asleep")                                                        // Foreman exited, no event
	if err := f.m.Sweep(ctx); err != nil {
		t.Fatal(err)
	}
	f.eventually("woken", func() bool { return slices.Equal(f.s.wokenList(), []string{"tove@ledger"}) })
}

// Delivered hears each message once it is typed in, never one that failed.
func TestDeliveredHearsWhatWasTypedIn(t *testing.T) {
	f := newFixture(t)
	var mu sync.Mutex
	var heard []store.TeamMessage
	f.m.Delivered = func(_ context.Context, msg store.TeamMessage) {
		mu.Lock()
		defer mu.Unlock()
		heard = append(heard, msg)
	}
	count := func() int {
		mu.Lock()
		defer mu.Unlock()
		return len(heard)
	}
	f.s.setFail(true)
	f.call("tove@ledger", "ask", map[string]string{"teammate": "mara", "request": "x"})
	time.Sleep(50 * time.Millisecond)
	f.m.Deliver("mara@ledger")
	if count() != 0 {
		t.Fatalf("a failed delivery was heard: %+v", heard)
	}
	f.s.setFail(false)
	f.m.Deliver("mara@ledger")
	f.eventually("heard", func() bool { return count() == 1 })
	if msg := heard[0]; msg.Ref != "msg-1" || msg.State != store.MessageDelivered || msg.DeliveredAt.IsZero() {
		t.Errorf("heard %+v", msg)
	}
}
