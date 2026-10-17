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
	"sync/atomic"
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
	f := newFixture(t, `read line; exit 1`, 1)
	if _, err := f.m.Deploy(ctx, "atlas", "ledger", "g"); err != nil {
		t.Fatal(err)
	}
	f.waitStatus("mara@ledger", Running)
	_ = f.m.Wake(ctx, "tove@ledger")
	time.Sleep(200 * time.Millisecond)
	if s := f.status("tove@ledger"); s.Status != Queued {
		t.Fatalf("tove is %s with one slot taken", s.Status)
	}
	// The lead's program crashes: its slot goes to tove (the lead queues behind for its restart).
	term, _ := f.m.Terminal("mara@ledger")
	_ = term.Input([]byte("bye\n"))
	f.waitStatus("tove@ledger", Running)
	if s := f.status("mara@ledger"); s.Status != Queued || s.Restarts != 1 {
		t.Errorf("mara after its exit: %+v", s)
	}
}

// A crash restarts once; a second one stops the instance. The fake harness
// keeps no conversation, so its restart starts fresh and gets the goal again
// (Claude Code's continues instead: its adapter's tests).
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
	if string(openers) != "[Your goal on ledger: g]\n[Your goal on ledger: g]\n" {
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
	start := func(id, workspace string) runner.Process {
		labels := map[string]string{"foreman.instance": id}
		if workspace != "" {
			labels[workspaceLabel] = workspace
		}
		p, err := (direct.Runner{}).Start(ctx, runner.Spec{Command: []string{"/bin/sh", "-c", "cat"}, Labels: labels})
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	survivor, orphan := start("mara@ledger", f.root), start("ghost@ledger", f.root)
	// Another workspace on the machine, with the same names: never touched.
	theirs, theirOrphan := start("tove@ledger", "/elsewhere"), start("ghost2@ledger", "/elsewhere")
	// From before processes named their workspace: adopted when it matches one
	// of ours, never stopped.
	older, olderUnknown := start("tove@ledger", ""), start("ghost3@ledger", "")
	t.Cleanup(func() {
		for _, p := range []runner.Process{theirs, theirOrphan, olderUnknown} {
			_ = p.Stop(time.Second)
		}
	})
	f.deps.Runner = listing{listed: []runner.Process{survivor, orphan, theirs, theirOrphan, older, olderUnknown}}
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
	if term, err := m.Terminal("tove@ledger"); err != nil || term == nil {
		t.Errorf("the unlabelled process wasn't adopted: %v", err)
	}
	f.waitStatus("tove@ledger", Running)
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

// eventIsActivity reads a hook event's name as the activity itself.
type eventIsActivity struct{ fake.Adapter }

func (eventIsActivity) Interpret(event string, _ []byte) harness.Activity {
	return harness.Activity(event)
}

// Hook events move a running instance between running, idle and awaiting;
// they never wake, start or revive one.
func TestHooksBecomeStatuses(t *testing.T) {
	f := newFixture(t, keepsRunning, 12)
	f.deps.Adapters = map[string]harness.Adapter{"claude-code": eventIsActivity{f.deps.Adapters["claude-code"].(fake.Adapter)}}
	f.m = New(f.deps)
	if _, err := f.m.Deploy(ctx, "atlas", "ledger", "g"); err != nil {
		t.Fatal(err)
	}
	f.waitStatus("mara@ledger", Running)
	if !f.m.Live("mara@ledger") || f.m.Live("tove@ledger") {
		t.Error("live: mara runs, tove sleeps")
	}
	for _, step := range []struct{ event, want string }{
		{"idle", Idle}, {"waiting", Awaiting}, {"", Awaiting}, {"busy", Running}, {"idle", Idle},
	} {
		f.m.Hook("mara@ledger", step.event, nil)
		if s := f.status("mara@ledger"); s.Status != step.want {
			t.Errorf("after %q: %s, want %s", step.event, s.Status, step.want)
		}
	}
	f.m.Hook("tove@ledger", "busy", nil)
	if s := f.status("tove@ledger"); s.Status != Asleep {
		t.Errorf("a hook woke tove: %s", s.Status)
	}
	f.m.Hook("nobody@ledger", "busy", nil) // nothing to change, nothing to break
	if err := f.m.Stop(ctx, "atlas@ledger"); err != nil {
		t.Fatal(err)
	}
	f.m.Hook("mara@ledger", "busy", nil)
	if s := f.status("mara@ledger"); s.Status != Stopped || f.m.Live("mara@ledger") {
		t.Errorf("a hook after the stop: %s", s.Status)
	}
}

// The harness gets foreman-agent's place and the instance's own token.
func TestTheHarnessGetsTheAgentLink(t *testing.T) {
	f := newFixture(t, `echo "link=$FOREMAN_AGENT_URL $FOREMAN_AGENT_TOKEN"; cat`, 12)
	f.deps.Agent = AgentLink{Host: "/bin/foreman-agent", Path: "/opt/foreman/foreman-agent", URL: "http://h:8889",
		Token: func(id string) string { return "token-of-" + id }}
	f.m = New(f.deps)
	if _, err := f.m.Deploy(ctx, "atlas", "ledger", "g"); err != nil {
		t.Fatal(err)
	}
	f.waitStatus("mara@ledger", Running)
	f.output("mara@ledger", "link=http://h:8889 token-of-mara@ledger")
}

// A run frees only its own slot: the late end of a stopped run must not free
// the run a redeploy started in its place (seen as a flaky redeploy).
func TestARunReleasesOnlyItsOwnSlot(t *testing.T) {
	m := New(Deps{})
	old, current := &live{}, &live{}
	m.live["mara@ledger"] = current
	m.release("mara@ledger", old)
	if m.live["mara@ledger"] != current {
		t.Fatal("the old run freed the new run's slot")
	}
	m.release("mara@ledger", current)
	if m.Live("mara@ledger") {
		t.Error("the run's own release didn't free it")
	}
}

// slowGit holds the first Prepare until released: a start still under way.
type slowGit struct {
	Git
	calls   atomic.Int32
	entered chan struct{}
	release chan struct{}
}

func (g *slowGit) Prepare(ctx context.Context, repo, worktree, branch, base string) error {
	if g.calls.Add(1) == 1 {
		close(g.entered)
		<-g.release
	}
	return g.Git.Prepare(ctx, repo, worktree, branch, base)
}

// A stop while the lead is still starting, then a redeploy at once: the new
// run starts, and the old start's late end changes nothing.
func TestRedeployWhileTheStoppedRunIsStillStarting(t *testing.T) {
	f := newFixture(t, keepsRunning, 12)
	slow := &slowGit{Git: f.deps.Git, entered: make(chan struct{}), release: make(chan struct{})}
	f.deps.Git = slow
	f.m = New(f.deps)
	t.Cleanup(func() { closeOnce(slow.release) })
	if _, err := f.m.Deploy(ctx, "atlas", "ledger", "g"); err != nil {
		t.Fatal(err)
	}
	<-slow.entered
	if err := f.m.Stop(ctx, "atlas@ledger"); err != nil {
		t.Fatal(err)
	}
	if _, err := f.m.Deploy(ctx, "atlas", "ledger", "again"); err != nil {
		t.Fatal(err)
	}
	f.waitStatus("mara@ledger", Running)
	closeOnce(slow.release)
	f.m.Wait()
	if s := f.status("mara@ledger"); s.Status != Running {
		t.Errorf("after the old start ended: %+v", s)
	}
	if _, err := f.m.Terminal("mara@ledger"); err != nil {
		t.Errorf("the new run lost its terminal: %v", err)
	}
}

func closeOnce(c chan struct{}) {
	select {
	case <-c:
	default:
		close(c)
	}
}

// Without a code repo agents would have nothing to work in: refused, with
// what to do.
func TestDeployRefusesAProjectWithoutRepos(t *testing.T) {
	f := newFixture(t, keepsRunning, 12)
	if err := os.RemoveAll(filepath.Join(f.root, "ledger", "ledger-api")); err != nil {
		t.Fatal(err)
	}
	if err := f.ws.Load(ctx, git.New()); err != nil {
		t.Fatal(err)
	}
	if _, err := f.m.Deploy(ctx, "atlas", "ledger", "g"); !errors.Is(err, ErrInvalid) || !strings.Contains(err.Error(), "no code repo") {
		t.Errorf("%v", err)
	}
	if deps, _ := f.db.Deployments().List(ctx); len(deps) != 0 {
		t.Errorf("deployed anyway: %+v", deps)
	}
}

// A clean exit (the CEO's /exit) is done for now: asleep, holding no slot,
// until the CEO's Start (or work) wakes it, picking up its conversation.
func TestACleanExitSleepsAndStartResumes(t *testing.T) {
	f := newFixture(t, `echo "[$1]" >> "$FOREMAN_PROMPT_FILE.openers"; read line; exit 0`, 12)
	if _, err := f.m.Deploy(ctx, "atlas", "ledger", "g"); err != nil {
		t.Fatal(err)
	}
	f.waitStatus("mara@ledger", Running)
	term, _ := f.m.Terminal("mara@ledger")
	_ = term.Input([]byte("/exit\n"))
	inst := f.waitStatus("mara@ledger", Asleep)
	if inst.Restarts != 0 || f.m.Live("mara@ledger") {
		t.Errorf("after /exit: %+v, live %v", inst, f.m.Live("mara@ledger"))
	}
	time.Sleep(200 * time.Millisecond) // no restart on its own
	if s := f.status("mara@ledger"); s.Status != Asleep {
		t.Fatalf("restarted on its own: %s", s.Status)
	}
	if err := f.m.Wake(ctx, "mara@ledger"); err != nil {
		t.Fatal(err)
	}
	f.waitStatus("mara@ledger", Running)
}

// A stopped instance (two crashes) starts again on the CEO's Start, with its
// one restart back; a stopped deployment's instances don't.
func TestStartAgainAfterTwoCrashes(t *testing.T) {
	f := newFixture(t, `sleep 0.2; exit 3`, 12)
	if _, err := f.m.Deploy(ctx, "atlas", "ledger", "g"); err != nil {
		t.Fatal(err)
	}
	f.waitStatus("mara@ledger", Stopped)
	if err := f.m.Wake(ctx, "mara@ledger"); err != nil {
		t.Fatal(err)
	}
	if s := f.waitStatus("mara@ledger", Running); s.Restarts != 0 {
		t.Errorf("restarts %d after Start again", s.Restarts)
	}
	f.waitStatus("mara@ledger", Stopped) // crashes twice again
	if err := f.m.Stop(ctx, "atlas@ledger"); err != nil {
		t.Fatal(err)
	}
	if err := f.m.Wake(ctx, "mara@ledger"); err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)
	if s := f.status("mara@ledger"); s.Status != Stopped {
		t.Errorf("a stopped deployment's instance started: %s", s.Status)
	}
}

// Processes of another workspace, and old unlabelled ones that match nothing,
// are left running by a restore.
func TestRestoreLeavesOtherWorkspacesAlone(t *testing.T) {
	f := newFixture(t, keepsRunning, 12)
	start := func(id, workspace string) runner.Process {
		labels := map[string]string{"foreman.instance": id}
		if workspace != "" {
			labels[workspaceLabel] = workspace
		}
		p, err := (direct.Runner{}).Start(ctx, runner.Spec{Command: []string{"/bin/sh", "-c", "cat"}, Labels: labels})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = p.Stop(time.Second) })
		return p
	}
	theirs, older := start("mara@ledger", "/elsewhere"), start("ghost@ledger", "")
	f.deps.Runner = listing{listed: []runner.Process{theirs, older}}
	f.m = New(f.deps)
	if err := f.m.Restore(ctx); err != nil {
		t.Fatal(err)
	}
	for name, p := range map[string]runner.Process{"another workspace's": theirs, "an unlabelled": older} {
		done := make(chan struct{})
		go func() { _, _ = p.Wait(); close(done) }()
		select {
		case <-done:
			t.Errorf("%s process was stopped", name)
		case <-time.After(500 * time.Millisecond):
		}
	}
}

// Every process names its workspace, and its name is unique on the machine.
func TestProcessesNameTheirWorkspace(t *testing.T) {
	a := New(Deps{Root: "/ws/a"})
	b := New(Deps{Root: "/ws/b"})
	if a.processName("mara@ledger") == b.processName("mara@ledger") || !strings.HasSuffix(a.processName("mara@ledger"), "-mara@ledger") {
		t.Errorf("names %q %q", a.processName("mara@ledger"), b.processName("mara@ledger"))
	}
}

// While the engine isn't ready (Podman not running), deploying and the CEO's
// Start refuse, and nothing is created or started; once it is, both work.
func TestDeployAndWakeWaitForTheEngine(t *testing.T) {
	f := newFixture(t, keepsRunning, 12)
	var blocked atomic.Bool
	blocked.Store(true)
	f.m.Blocked = func() error {
		if blocked.Load() {
			return errors.New("podman isn't running")
		}
		return nil
	}
	if _, err := f.m.Deploy(ctx, "atlas", "ledger", "g"); !errors.Is(err, ErrNotReady) || !strings.Contains(err.Error(), "podman isn't running") {
		t.Fatalf("deploy: %v", err)
	}
	if deps, _ := f.db.Deployments().List(ctx); len(deps) != 0 {
		t.Fatalf("deployed anyway: %+v", deps)
	}
	blocked.Store(false)
	if _, err := f.m.Deploy(ctx, "atlas", "ledger", "g"); err != nil {
		t.Fatal(err)
	}
	f.waitStatus("mara@ledger", Running)
	blocked.Store(true)
	if err := f.m.Wake(ctx, "tove@ledger"); !errors.Is(err, ErrNotReady) {
		t.Fatalf("wake: %v", err)
	}
	if inst, _ := f.db.Instances().Get(ctx, "tove@ledger"); inst.Status != Asleep {
		t.Errorf("woken anyway: %s", inst.Status)
	}
	blocked.Store(false)
	if err := f.m.Wake(ctx, "tove@ledger"); err != nil {
		t.Fatal(err)
	}
	f.waitStatus("tove@ledger", Running)
}

// Foreman exiting cleanly puts every agent to sleep, a running one and a
// queued one alike, with no process left and its restart untouched. Nothing
// starts after it; on the next run, work wakes them as usual.
func TestShutdownPutsEveryAgentToSleep(t *testing.T) {
	f := newFixture(t, keepsRunning, 1)
	if _, err := f.m.Deploy(ctx, "atlas", "ledger", "g"); err != nil {
		t.Fatal(err)
	}
	f.waitStatus("mara@ledger", Running)
	if err := f.m.Wake(ctx, "tove@ledger"); err != nil {
		t.Fatal(err)
	}
	f.waitStatus("tove@ledger", Queued) // the one slot is mara's
	term, err := f.m.Terminal("mara@ledger")
	if err != nil {
		t.Fatal(err)
	}
	f.m.Shutdown(ctx)
	select {
	case <-term.Done():
	default:
		t.Error("mara's process outlived the shutdown")
	}
	for _, id := range []string{"mara@ledger", "tove@ledger"} {
		if s := f.status(id); s.Status != Asleep || s.Restarts != 0 || f.m.Live(id) {
			t.Errorf("%s after the shutdown: %+v, live %v", id, s, f.m.Live(id))
		}
	}
	if err := f.m.Wake(ctx, "tove@ledger"); !errors.Is(err, ErrNotReady) {
		t.Errorf("woken while stopping: %v", err)
	}
	if _, err := f.m.Deploy(ctx, "atlas", "ledger", "again"); !errors.Is(err, ErrNotReady) {
		t.Errorf("deployed while stopping: %v", err)
	}
	time.Sleep(100 * time.Millisecond) // nothing starts on its own
	if s := f.status("tove@ledger"); s.Status != Asleep {
		t.Fatalf("tove started: %s", s.Status)
	}

	// The next run: nothing to re-attach, and work wakes them.
	f.m = New(f.deps)
	if err := f.m.Restore(ctx); err != nil {
		t.Fatal(err)
	}
	if s := f.status("mara@ledger"); s.Status != Asleep {
		t.Fatalf("restored as %s", s.Status)
	}
	if err := f.m.Wake(ctx, "mara@ledger"); err != nil {
		t.Fatal(err)
	}
	f.waitStatus("mara@ledger", Running)
}

// An agent still starting when Foreman exits ends asleep too, its process
// stopped as soon as it is launched.
func TestShutdownWhileStarting(t *testing.T) {
	f := newFixture(t, keepsRunning, 12)
	slow := &slowGit{Git: f.deps.Git, entered: make(chan struct{}), release: make(chan struct{})}
	f.deps.Git = slow
	f.m = New(f.deps)
	t.Cleanup(func() { closeOnce(slow.release) })
	if _, err := f.m.Deploy(ctx, "atlas", "ledger", "g"); err != nil {
		t.Fatal(err)
	}
	<-slow.entered
	done := make(chan struct{})
	go func() { f.m.Shutdown(ctx); close(done) }()
	select {
	case <-done:
		t.Fatal("the shutdown didn't wait for the start under way")
	case <-time.After(100 * time.Millisecond):
	}
	closeOnce(slow.release)
	select {
	case <-done:
	case <-time.After(15 * time.Second):
		t.Fatal("the shutdown never ended")
	}
	if s := f.status("mara@ledger"); s.Status != Asleep || f.m.Live("mara@ledger") {
		t.Errorf("after the shutdown: %+v, live %v", s, f.m.Live("mara@ledger"))
	}
}

// Deployed without a goal, the lead starts with no opener: the CEO gives the
// goal in its session.
func TestNoGoalNoOpener(t *testing.T) {
	f := newFixture(t, `echo "opener: [$1]"; `+keepsRunning, 12)
	if _, err := f.m.Deploy(ctx, "atlas", "ledger", ""); err != nil {
		t.Fatal(err)
	}
	f.waitStatus("mara@ledger", Running)
	f.output("mara@ledger", "opener: []")
}

// unstoppable starts real processes whose Stop fails, as when Podman stops
// answering; it keeps them to end at cleanup.
type unstoppable struct {
	runner.Runner
	mu    sync.Mutex
	procs []runner.Process
}

type stuck struct{ runner.Process }

func (stuck) Stop(time.Duration) error { return errors.New("podman isn't answering") }

func (u *unstoppable) Start(ctx context.Context, spec runner.Spec) (runner.Process, error) {
	p, err := u.Runner.Start(ctx, spec)
	if err != nil {
		return nil, err
	}
	u.mu.Lock()
	defer u.mu.Unlock()
	u.procs = append(u.procs, p)
	return stuck{p}, nil
}

// An agent whose stop fails keeps its status, so the next start re-attaches
// it as after a crash rather than leave it running unseen as asleep.
func TestShutdownKeepsWhatItCouldntStop(t *testing.T) {
	f := newFixture(t, keepsRunning, 12)
	u := &unstoppable{Runner: f.deps.Runner}
	t.Cleanup(func() {
		for _, p := range u.procs {
			_ = p.Stop(time.Second)
		}
	})
	f.deps.Runner = u
	f.m = New(f.deps)
	if _, err := f.m.Deploy(ctx, "atlas", "ledger", "g"); err != nil {
		t.Fatal(err)
	}
	f.waitStatus("mara@ledger", Running)
	f.m.Shutdown(ctx)
	if s := f.status("mara@ledger"); s.Status != Running {
		t.Errorf("after a failed stop: %s", s.Status)
	}
}
