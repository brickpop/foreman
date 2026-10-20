package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/brickpop/foreman/internal/agentlink"
	"github.com/brickpop/foreman/internal/cos"
	"github.com/brickpop/foreman/internal/events"
	"github.com/brickpop/foreman/internal/git"
	"github.com/brickpop/foreman/internal/harness"
	"github.com/brickpop/foreman/internal/harness/fake"
	"github.com/brickpop/foreman/internal/home"
	"github.com/brickpop/foreman/internal/instances"
	"github.com/brickpop/foreman/internal/profiles"
	"github.com/brickpop/foreman/internal/runner"
	"github.com/brickpop/foreman/internal/runner/direct"
	"github.com/brickpop/foreman/internal/sessionlog"
	"github.com/brickpop/foreman/internal/store"
	"github.com/brickpop/foreman/internal/terminal"
)

// specs records what each start asked of the engine.
type specs struct {
	runner.Runner
	got []runner.Spec
}

func (s *specs) Start(ctx context.Context, spec runner.Spec) (runner.Process, error) {
	s.got = append(s.got, spec)
	return s.Runner.Start(ctx, spec)
}

// testChief is a home's Chief of Staff on a stand-in harness: a script
// that prints its arguments, then echoes what is typed into it.
func testChief(t *testing.T) (*chiefOfStaff, *home.Home, *specs) {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("HOME", t.TempDir())
	h, err := home.Open(filepath.Join(os.Getenv("HOME"), ".foreman"))
	if err != nil {
		t.Fatal(err)
	}
	db, err := store.Open(context.Background(), h.Path("state.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := h.Load(context.Background(), home.Deps{Git: git.New(), Projects: db.Projects()}); err != nil {
		t.Fatal(err)
	}
	program := filepath.Join(t.TempDir(), "harness.sh")
	if err := os.WriteFile(program, []byte("#!/bin/sh\necho \"opened: $*\"\ncat\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	logins := profiles.Store{Root: t.TempDir()}
	if err := logins.Ensure("fake"); err != nil {
		t.Fatal(err)
	}
	logs := sessionlog.Dir{Root: t.TempDir()}
	engine := &specs{Runner: direct.Runner{}}
	c := &chiefOfStaff{
		home: h.Root(), dir: h.Path("cos"), teamsDir: h.TeamsDir(), teams: h.Teams,
		profiles: logins, adapters: map[string]harness.Adapter{"fake": fake.Adapter{Program: program}},
		images: direct.Images{}, terms: newSessions(engine, func(id string) (terminal.Log, error) { return logs.Open(id) }),
		agent:   instances.AgentLink{Host: "/host/foreman-agent", Path: "/opt/foreman-agent", URL: "http://agent", Token: func(id string) string { return "token-" + id }},
		blocked: func() error { return nil },
	}
	c.CoS = cos.New(cos.Deps{Teams: h, Publish: func(events.Event) {}, Tell: c.tell,
		Deployed: func(context.Context, string) bool { return false }})
	t.Cleanup(func() { c.terms.stopAll() })
	return c, h, engine
}

// screen waits for the session's screen to show want.
func screen(t *testing.T, c *chiefOfStaff, want string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if a, err := c.terms.Attach(cos.Instance); err == nil {
			view := a.(attachment).View
			shown := strings.Contains(string(view.Snapshot()), want)
			view.Close()
			if shown {
				return
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("the screen never showed %q", want)
}

// The session sees its own folder and the team files, read-only, and nothing
// else of the home; it starts with the opener and the default profile.
func TestTheChiefOfStaffSession(t *testing.T) {
	c, h, engine := testChief(t)
	term, err := c.Start(context.Background(), "Create a new team.")
	if err != nil || term != cos.Instance {
		t.Fatalf("start: %q %v", term, err)
	}
	screen(t, c, "opened: Create a new team.")
	spec := engine.got[0]
	want := []runner.Mount{
		{Source: h.Path("cos"), Target: h.Path("cos")},
		{Source: h.TeamsDir(), Target: h.TeamsDir(), ReadOnly: true},
		{Source: "/host/foreman-agent", Target: "/opt/foreman-agent", ReadOnly: true},
	}
	if len(spec.Mounts) != len(want) {
		t.Fatalf("mounts %+v", spec.Mounts)
	}
	for i, m := range want {
		if spec.Mounts[i] != m {
			t.Errorf("mount %d: %+v, want %+v", i, spec.Mounts[i], m)
		}
	}
	for _, m := range spec.Mounts {
		if m.Source == h.Path() || strings.HasPrefix(h.Path("secrets"), m.Source) {
			t.Errorf("mounted %s", m.Source)
		}
	}
	if spec.Dir != h.Path("cos") || spec.Labels[instances.HomeLabel] != h.Root() || !strings.HasSuffix(spec.Name, "-cos") {
		t.Errorf("spec %+v", spec)
	}
	if env := strings.Join(spec.Env, " "); !strings.Contains(env, "FOREMAN_AGENT_TOKEN=token-cos") {
		t.Errorf("env %s", env)
	}
	prompt, _ := os.ReadFile(filepath.Join(h.Path("cos"), ".harness", "fake", "PROMPT.md"))
	if !strings.Contains(string(prompt), h.TeamsDir()) {
		t.Errorf("prompt:\n%s", prompt)
	}
	if !c.Live() {
		t.Error("not live")
	}

	// Another entry point while it runs: the same session, told what for.
	if term, err := c.Start(context.Background(), "Hire someone for Atlas."); err != nil || term != cos.Instance || len(engine.got) != 1 {
		t.Fatalf("again: %q %v, %d starts", term, err, len(engine.got))
	}
	screen(t, c, "Hire someone for Atlas.")

	if err := c.Stop(); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for c.Live() && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if c.Live() {
		t.Error("still live after Stop")
	}
	// The next visit starts it again.
	if _, err := c.Start(context.Background(), ""); err != nil || len(engine.got) != 2 {
		t.Errorf("restart: %v, %d starts", err, len(engine.got))
	}
}

func TestTheChiefOfStaffWaitsForPodman(t *testing.T) {
	c, _, engine := testChief(t)
	c.blocked = func() error { return errors.New("podman isn't running") }
	if _, err := c.Start(context.Background(), ""); !errors.Is(err, instances.ErrNotReady) || len(engine.got) != 0 {
		t.Errorf("start: %v", err)
	}
}

// From propose_team to the team file, through the home's own rules.
func TestAProposalBecomesATeamFile(t *testing.T) {
	c, h, _ := testChief(t)
	if _, err := c.Start(context.Background(), ""); err != nil {
		t.Fatal(err)
	}
	calls := agentCalls{team: noTools{}, cos: c}
	if tools := calls.List(cos.Instance); len(tools) != 1 || tools[0].Name != cos.ProposeTeam {
		t.Fatalf("tools %+v", tools)
	}
	if tools := calls.List("mara@ledger"); len(tools) != 0 {
		t.Errorf("an agent got %+v", tools)
	}
	team := `{"team": "vesper", "name": "Vesper", "lead": "kestrel", "order": ["kestrel", "nyx"],
	  "agents": {"kestrel": {"name": "Kestrel", "role": "team_lead"}, "nyx": {"name": "Nyx", "role": "qa"}}}`
	// The home's rules: an id that isn't a slug is refused, never shown.
	if r := calls.Call(context.Background(), cos.Instance, cos.ProposeTeam, json.RawMessage(strings.Replace(team, `"vesper"`, `"Vesper Team"`, 1))); !r.Error {
		t.Errorf("a bad id: %+v", r)
	}
	if r := calls.Call(context.Background(), cos.Instance, cos.ProposeTeam, json.RawMessage(team)); r.Error {
		t.Fatalf("propose: %s", r.Text)
	}
	p, ok := c.Proposal()
	if !ok {
		t.Fatal("nothing on screen")
	}
	look := home.Look{Color: "#8E9AAF", Skin: "#C38A62", Hair: "curly", HairColor: "#6B4A2F", Eyes: "sleepy", Beard: "full"}
	if _, err := c.Create(context.Background(), p.ID, map[string]home.Look{"kestrel": look, "nyx": look}); err != nil {
		t.Fatal(err)
	}
	saved, err := h.Team("vesper")
	if err != nil || saved.Config.Agents["nyx"].Look != look || len(saved.Problems) != 0 {
		t.Errorf("saved %+v %v", saved, err)
	}
	// The session is told, as typed input.
	screen(t, c, "the CEO created Vesper")
	// Stopping drops whatever is on screen.
	if r := calls.Call(context.Background(), cos.Instance, cos.ProposeTeam, json.RawMessage(strings.Replace(team, `"Vesper"`, `"Vesper 2"`, 1))); r.Error {
		t.Fatalf("propose again: %s", r.Text)
	}
	_ = c.Stop()
	if _, ok := c.Proposal(); ok {
		t.Error("the proposal outlived the session")
	}
}

type noTools struct{}

func (noTools) List(string) []agentlink.Tool { return nil }
func (noTools) Call(context.Context, string, string, json.RawMessage) agentlink.Result {
	return agentlink.Result{}
}
