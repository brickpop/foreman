package instances

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/brickpop/foreman/internal/events"
	"github.com/brickpop/foreman/internal/git"
	"github.com/brickpop/foreman/internal/harness"
	"github.com/brickpop/foreman/internal/harness/fake"
	"github.com/brickpop/foreman/internal/runner"
	"github.com/brickpop/foreman/internal/runner/direct"
	"github.com/brickpop/foreman/internal/sessionlog"
	"github.com/brickpop/foreman/internal/store"
	"github.com/brickpop/foreman/internal/terminal"
	"github.com/brickpop/foreman/internal/workspace"
)

var ctx = context.Background()

const teamFile = `team = "atlas"
name = "Atlas"
lead = "mara"

[agents.mara]
name = "Mara"
role = "team_lead"
harness = "claude-code"

[agents.tove]
name = "Tove"
role = "engineer"
harness = "claude-code"
`

func write(t *testing.T, path, content string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
}

func gitRun(t *testing.T, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@t", "-c", "commit.gpgsign=false"}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v %s", args, err, out)
	}
}

type fixture struct {
	t      *testing.T
	root   string
	m      *Manager
	db     *store.DB
	ws     *workspace.Workspace
	deps   Deps
	mu     sync.Mutex
	events []events.Event
}

// newFixture makes a workspace with project ledger (one code repo) and team
// atlas, whose harness is a shell script: script is its body.
func newFixture(t *testing.T, script string, maxSessions int) *fixture {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	root := t.TempDir()
	ws, err := workspace.Init(root)
	if err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(root, "ledger", "project", "project.toml"), `project = "ledger"`, 0o644)
	gitRun(t, "init", "--quiet", "--initial-branch=main", filepath.Join(root, "ledger", "project"))
	api := filepath.Join(root, "ledger", "ledger-api")
	gitRun(t, "init", "--quiet", "--initial-branch=main", api)
	write(t, filepath.Join(api, "README"), "hello\n", 0o644)
	gitRun(t, "-C", api, "add", "README")
	gitRun(t, "-C", api, "commit", "--quiet", "-m", "first")
	write(t, filepath.Join(root, ".foreman", "teams", "atlas.toml"), teamFile, 0o644)
	write(t, filepath.Join(root, ".foreman", "settings.toml"), fmt.Sprintf("max_sessions = %d\n", maxSessions), 0o644)
	if err := ws.Load(ctx, git.New()); err != nil {
		t.Fatal(err)
	}
	program := filepath.Join(t.TempDir(), "harness.sh")
	write(t, program, "#!/bin/sh\n"+script+"\n", 0o755)
	db, err := store.Open(ctx, filepath.Join(root, ".foreman", "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	f := &fixture{t: t, root: root, db: db, ws: ws}
	logs := sessionlog.Dir{Root: filepath.Join(root, ".foreman", "logs")}
	f.deps = Deps{
		Root: root, Runner: direct.Runner{}, Images: direct.Images{}, Git: git.New(), Config: ws,
		Deployments: db.Deployments(), Instances: db.Instances(),
		Adapters: map[string]harness.Adapter{"claude-code": fake.Adapter{Program: program}},
		OpenLog:  func(id string) (terminal.Log, error) { return logs.Open(id) },
		Publish: func(e events.Event) {
			f.mu.Lock()
			defer f.mu.Unlock()
			f.events = append(f.events, e)
		},
	}
	f.m = New(f.deps)
	t.Cleanup(func() {
		f.stopAll()
		f.m.Wait()
	})
	return f
}

func (f *fixture) stopAll() {
	deps, _ := f.db.Deployments().List(ctx)
	for _, d := range deps {
		_ = f.m.Stop(ctx, d.ID)
	}
}

func (f *fixture) status(id string) store.Instance {
	f.t.Helper()
	inst, err := f.db.Instances().Get(ctx, id)
	if err != nil {
		f.t.Fatal(err)
	}
	return inst
}

func (f *fixture) waitStatus(id, want string) store.Instance {
	f.t.Helper()
	var inst store.Instance
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		if inst = f.status(id); inst.Status == want {
			return inst
		}
	}
	f.t.Fatalf("%s is %s, never %s", id, inst.Status, want)
	return inst
}

func (f *fixture) branch(dir string) string {
	out, err := exec.Command("git", "-C", dir, "rev-parse", "--abbrev-ref", "HEAD").Output()
	if err != nil {
		f.t.Fatalf("%s: %v", dir, err)
	}
	return strings.TrimSpace(string(out))
}

// keepsRunning is a harness that waits for input and echoes its prompt file's title.
const keepsRunning = `head -n1 "$FOREMAN_PROMPT_FILE"; cat`

// output waits until id's terminal has printed want, and returns what it printed.
func (f *fixture) output(id, want string) string {
	f.t.Helper()
	term, err := f.m.Terminal(id)
	if err != nil {
		f.t.Fatal(err)
	}
	v, _ := term.Attach()
	defer v.Close()
	deadline := time.After(5 * time.Second)
	got := string(v.Snapshot())
	for !strings.Contains(got, want) {
		select {
		case chunk := <-v.Output():
			got += string(chunk)
		case <-deadline:
			f.t.Fatalf("%s never printed %q: %q", id, want, got)
		}
	}
	return got
}

// Only the lead starts, with the goal as its opener; a member sleeps, with no
// sandbox, until it is woken, and then starts without one.
func TestDeployStartsTheLeadAndMembersWhenWoken(t *testing.T) {
	f := newFixture(t, `echo "opener: [$1]"; `+keepsRunning, 12)
	dep, err := f.m.Deploy(ctx, "atlas", "ledger", "Ship invoices")
	if err != nil {
		t.Fatal(err)
	}
	if dep.ID != "atlas@ledger" {
		t.Errorf("deployment %+v", dep)
	}
	f.waitStatus("mara@ledger", Running)
	if got := f.output("mara@ledger", "# You are"); !strings.Contains(got, "opener: [Your goal on ledger: Ship invoices]") {
		t.Errorf("the lead's opener: %q", got)
	}
	time.Sleep(200 * time.Millisecond)
	if s := f.status("tove@ledger"); s.Status != Asleep || !s.StartedAt.IsZero() {
		t.Errorf("tove before any work: %+v", s)
	}
	if _, err := os.Stat(filepath.Join(f.root, "ledger", ".sandboxes", "tove")); !os.IsNotExist(err) {
		t.Errorf("an asleep member has a sandbox: %v", err)
	}
	if err := f.m.Wake(ctx, "tove@ledger"); err != nil {
		t.Fatal(err)
	}
	f.waitStatus("tove@ledger", Running)
	if got := f.output("tove@ledger", "# You are"); !strings.Contains(got, "opener: []") {
		t.Errorf("a member got an opener: %q", got)
	}
	for _, id := range []string{"mara@ledger", "tove@ledger"} {
		if inst := f.status(id); inst.StartedAt.IsZero() || inst.SandboxPath != filepath.Join(f.root, "ledger", ".sandboxes", strings.Split(id, "@")[0]) {
			t.Errorf("%s: %+v", id, inst)
		}
	}
	if b := f.branch(filepath.Join(f.root, "ledger", ".sandboxes", "mara", "ledger-api")); b != "lead/atlas/integration" {
		t.Errorf("the lead's branch %q", b)
	}
	if b := f.branch(filepath.Join(f.root, "ledger", ".sandboxes", "tove", "ledger-api")); b != "wt/tove/main" {
		t.Errorf("a member's branch %q", b)
	}
	prompt, _ := os.ReadFile(filepath.Join(f.root, "ledger", ".sandboxes", "tove", ".harness", "fake", "PROMPT.md"))
	if !strings.Contains(string(prompt), "Goal: Ship invoices") || strings.Contains(string(prompt), "## Tools") {
		t.Errorf("tove's prompt:\n%s", prompt)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.events) == 0 || f.events[0].Type != "deployment.created" {
		t.Errorf("events %+v", f.events)
	}
}

// Wake starts only an asleep instance.
func TestWake(t *testing.T) {
	f := newFixture(t, keepsRunning, 12)
	if err := f.m.Wake(ctx, "nobody@ledger"); err == nil {
		t.Error("woke an instance that doesn't exist")
	}
	if _, err := f.m.Deploy(ctx, "atlas", "ledger", "g"); err != nil {
		t.Fatal(err)
	}
	f.waitStatus("mara@ledger", Running)
	if err := f.m.Wake(ctx, "mara@ledger"); err != nil {
		t.Fatal(err)
	}
	if s := f.status("mara@ledger"); s.Status != Running || s.Restarts != 0 {
		t.Errorf("waking a running lead: %+v", s)
	}
	_ = f.m.Stop(ctx, "atlas@ledger")
	if s := f.status("tove@ledger"); s.Status != Stopped {
		t.Errorf("an asleep member after stop: %+v", s)
	}
	if err := f.m.Wake(ctx, "tove@ledger"); err != nil {
		t.Fatal(err)
	}
	time.Sleep(200 * time.Millisecond)
	if s := f.status("tove@ledger"); s.Status != Stopped {
		t.Errorf("woke a stopped instance: %+v", s)
	}
}

func TestDeployRefusals(t *testing.T) {
	f := newFixture(t, keepsRunning, 12)
	if _, err := f.m.Deploy(ctx, "nope", "ledger", "g"); !errors.Is(err, ErrInvalid) {
		t.Errorf("unknown team: %v", err)
	}
	if _, err := f.m.Deploy(ctx, "atlas", "nope", "g"); !errors.Is(err, ErrInvalid) {
		t.Errorf("unknown project: %v", err)
	}
	if _, err := f.m.Deploy(ctx, "atlas", "ledger", "g"); err != nil {
		t.Fatal(err)
	}
	if _, err := f.m.Deploy(ctx, "atlas", "ledger", "again"); !errors.Is(err, store.ErrActive) {
		t.Errorf("deployed twice: %v", err)
	}
	// A harness without an adapter.
	write(t, filepath.Join(f.root, ".foreman", "teams", "orion.toml"),
		strings.NewReplacer(`team = "atlas"`, `team = "orion"`, `"Atlas"`, `"Orion"`, "mara", "nora", "Mara", "Nora", "tove", "ivo", "Tove", "Ivo", `harness = "claude-code"`, `harness = "codex"`).Replace(teamFile), 0o644)
	if err := f.ws.Load(ctx, git.New()); err != nil {
		t.Fatal(err)
	}
	if _, err := f.m.Deploy(ctx, "orion", "ledger", "g"); err == nil || !strings.Contains(err.Error(), "isn't supported yet") {
		t.Errorf("an unsupported harness: %v", err)
	}
	// A harness without a login: refused before anything is created.
	f.m.Adapters["codex"] = disconnected{}
	if _, err := f.m.Deploy(ctx, "orion", "ledger", "g"); !errors.Is(err, ErrInvalid) || !strings.Contains(err.Error(), "codex isn't connected yet") {
		t.Errorf("a harness without a login: %v", err)
	}
	if _, err := f.db.Deployments().Get(ctx, "orion@ledger"); err == nil {
		t.Error("a refused deployment was stored")
	}
}

type disconnected struct{ fake.Adapter }

func (disconnected) Connected() bool { return false }

// A Stop that lands while an instance is starting, after its process exists
// but before it is handed over, still stops it and frees its slot.
func TestStopWhileStarting(t *testing.T) {
	f := newFixture(t, keepsRunning, 1)
	opening, release := make(chan struct{}), make(chan struct{})
	open := f.deps.OpenLog
	f.deps.OpenLog = func(id string) (terminal.Log, error) {
		if id == "mara@ledger" {
			close(opening)
			<-release
		}
		return open(id)
	}
	f.m = New(f.deps)
	if _, err := f.m.Deploy(ctx, "atlas", "ledger", "g"); err != nil {
		t.Fatal(err)
	}
	<-opening
	if err := f.m.Stop(ctx, "atlas@ledger"); err != nil {
		t.Fatal(err)
	}
	close(release)
	f.m.Wait()
	if s := f.status("mara@ledger"); s.Status != Stopped {
		t.Errorf("stopped while starting, now %s", s.Status)
	}
	if _, err := f.m.Terminal("mara@ledger"); !errors.Is(err, ErrNotRunning) {
		t.Errorf("a stopped instance kept its terminal: %v", err)
	}
	// Its one slot is free: another deployment's lead starts.
	write(t, filepath.Join(f.root, ".foreman", "teams", "orion.toml"),
		strings.NewReplacer(`team = "atlas"`, `team = "orion"`, `"Atlas"`, `"Orion"`, "mara", "nora", "Mara", "Nora", "tove", "ivo", "Tove", "Ivo").Replace(teamFile), 0o644)
	if err := f.ws.Load(ctx, git.New()); err != nil {
		t.Fatal(err)
	}
	if _, err := f.m.Deploy(ctx, "orion", "ledger", "g"); err != nil {
		t.Fatal(err)
	}
	f.waitStatus("nora@ledger", Running)
}

func TestStopKeepsTheWorktrees(t *testing.T) {
	f := newFixture(t, keepsRunning, 12)
	if _, err := f.m.Deploy(ctx, "atlas", "ledger", "g"); err != nil {
		t.Fatal(err)
	}
	_ = f.m.Wake(ctx, "tove@ledger")
	f.waitStatus("tove@ledger", Running)
	if err := f.m.Stop(ctx, "atlas@ledger"); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"mara@ledger", "tove@ledger"} {
		if s := f.status(id); s.Status != Stopped || s.Restarts != 0 {
			t.Errorf("%s after stop: %+v (a stop is not a crash)", id, s)
		}
		if _, err := f.m.Terminal(id); !errors.Is(err, ErrNotRunning) {
			t.Errorf("%s still has a terminal", id)
		}
	}
	if d, _ := f.db.Deployments().Get(ctx, "atlas@ledger"); d.Status != store.DeploymentStopped {
		t.Errorf("deployment %+v", d)
	}
	if _, err := os.Stat(filepath.Join(f.root, "ledger", ".sandboxes", "tove", "ledger-api", "README")); err != nil {
		t.Error("stopping removed the worktree")
	}
	// Redeploying starts again, fresh.
	if _, err := f.m.Deploy(ctx, "atlas", "ledger", "next goal"); err != nil {
		t.Fatal(err)
	}
	f.waitStatus("mara@ledger", Running)
}

// Beyond max_sessions, instances wait their turn.
func TestTheConcurrencyLimitQueues(t *testing.T) {
	f := newFixture(t, `read line; exit 0`, 1)
	if _, err := f.m.Deploy(ctx, "atlas", "ledger", "g"); err != nil {
		t.Fatal(err)
	}
	f.waitStatus("mara@ledger", Running)
	_ = f.m.Wake(ctx, "tove@ledger")
	time.Sleep(200 * time.Millisecond)
	if s := f.status("tove@ledger"); s.Status != Queued {
		t.Fatalf("tove is %s with one slot taken", s.Status)
	}
	// The lead's program ends: its slot goes to tove (the lead queues behind for its restart).
	term, _ := f.m.Terminal("mara@ledger")
	_ = term.Input([]byte("bye\n"))
	f.waitStatus("tove@ledger", Running)
	if s := f.status("mara@ledger"); s.Status != Queued || s.Restarts != 1 {
		t.Errorf("mara after its exit: %+v", s)
	}
}

// A crash restarts once, without the opener again; a second one stops the instance.
func TestACrashRestartsOnceThenStops(t *testing.T) {
	f := newFixture(t, `echo "[$1]" >> "$FOREMAN_PROMPT_FILE.openers"; sleep 0.2; exit 3`, 12)
	if _, err := f.m.Deploy(ctx, "atlas", "ledger", "g"); err != nil {
		t.Fatal(err)
	}
	inst := f.waitStatus("mara@ledger", Stopped)
	if inst.Restarts != 1 {
		t.Errorf("restarts %d, want 1", inst.Restarts)
	}
	openers, _ := os.ReadFile(filepath.Join(f.root, "ledger", ".sandboxes", "mara", ".harness", "fake", "PROMPT.md.openers"))
	if string(openers) != "[Your goal on ledger: g]\n[]\n" {
		t.Errorf("openers per start: %q", openers)
	}
}

// listing is a runner whose List also returns given processes, as Podman's
// does after a Foreman restart.
type listing struct {
	direct.Runner
	listed []runner.Process
}

func (l listing) List(context.Context) ([]runner.Process, error) { return l.listed, nil }

// After a Foreman restart: a running process is re-attached, a live instance
// without one restarts, and a process nobody owns is stopped.
func TestRestore(t *testing.T) {
	f := newFixture(t, keepsRunning, 12)
	dep, err := f.db.Deployments().Deploy(ctx, "atlas", "ledger", "g")
	if err != nil {
		t.Fatal(err)
	}
	for agent, status := range map[string]string{"mara": Running, "tove": Running, "ivo": Asleep} {
		_ = f.db.Instances().Put(ctx, store.Instance{
			ID: agent + "@ledger", DeploymentID: dep.ID, AgentID: agent, Status: status, Harness: "claude-code",
			SandboxPath: filepath.Join(f.root, "ledger", ".sandboxes", agent), StartedAt: time.Now().Add(-time.Hour),
		})
	}
	start := func(id string) runner.Process {
		p, err := (direct.Runner{}).Start(ctx, runner.Spec{Command: []string{"/bin/sh", "-c", "cat"}, Labels: map[string]string{"foreman.instance": id}})
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	survivor, orphan := start("mara@ledger"), start("ghost@ledger")
	f.deps.Runner = listing{listed: []runner.Process{survivor, orphan}}
	m := New(f.deps)
	f.m = m
	if err := m.Restore(ctx); err != nil {
		t.Fatal(err)
	}
	if term, err := m.Terminal("mara@ledger"); err != nil || term == nil {
		t.Errorf("the survivor wasn't re-attached: %v", err)
	} else {
		_ = term.Input([]byte("still here\n"))
	}
	f.waitStatus("tove@ledger", Running) // restarted
	if s := f.status("ivo@ledger"); s.Status != Asleep {
		t.Errorf("an asleep instance after a restore: %+v", s)
	}
	done := make(chan struct{})
	go func() { _, _ = orphan.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(15 * time.Second):
		t.Error("the orphan was left running")
	}
}

func TestListShowsActiveDeploymentsOnly(t *testing.T) {
	f := newFixture(t, keepsRunning, 12)
	if list, err := f.m.List(ctx); err != nil || len(list) != 0 {
		t.Errorf("empty: %v %v", list, err)
	}
	if _, err := f.m.Deploy(ctx, "atlas", "ledger", "g"); err != nil {
		t.Fatal(err)
	}
	if list, _ := f.m.List(ctx); len(list) != 2 || list[0].ID != "mara@ledger" {
		t.Errorf("active: %+v", list)
	}
	_ = f.m.Stop(ctx, "atlas@ledger")
	if list, _ := f.m.List(ctx); len(list) != 0 {
		t.Errorf("after stop: %+v", list)
	}
}

func TestActiveDeployments(t *testing.T) {
	f := newFixture(t, keepsRunning, 12)
	if list, err := f.m.ActiveDeployments(ctx); err != nil || len(list) != 0 {
		t.Errorf("none yet: %v %v", list, err)
	}
	if _, err := f.m.Deploy(ctx, "atlas", "ledger", "Ship it"); err != nil {
		t.Fatal(err)
	}
	if list, _ := f.m.ActiveDeployments(ctx); len(list) != 1 || list[0].Goal != "Ship it" {
		t.Errorf("active: %+v", list)
	}
	_ = f.m.Stop(ctx, "atlas@ledger")
	if list, _ := f.m.ActiveDeployments(ctx); len(list) != 0 {
		t.Errorf("after stop: %+v", list)
	}
}

// The prompt shown is the one the harness got.
func TestPromptIsWhatTheHarnessGot(t *testing.T) {
	f := newFixture(t, keepsRunning, 12)
	if _, err := f.m.Deploy(ctx, "atlas", "ledger", "Ship invoices"); err != nil {
		t.Fatal(err)
	}
	_ = f.m.Wake(ctx, "tove@ledger")
	f.waitStatus("tove@ledger", Running)
	shown, err := f.m.Prompt(ctx, "tove@ledger")
	if err != nil {
		t.Fatal(err)
	}
	given, _ := os.ReadFile(filepath.Join(f.root, "ledger", ".sandboxes", "tove", ".harness", "fake", "PROMPT.md"))
	if shown != string(given) || !strings.Contains(shown, "Goal: Ship invoices") {
		t.Errorf("shown:\n%s\ngiven:\n%s", shown, given)
	}
	if _, err := f.m.Prompt(ctx, "nobody@ledger"); err == nil {
		t.Error("a prompt for no instance")
	}
}
