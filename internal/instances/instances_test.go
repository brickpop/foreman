package instances

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/brickpop/foreman/internal/events"
	"github.com/brickpop/foreman/internal/git"
	"github.com/brickpop/foreman/internal/harness"
	"github.com/brickpop/foreman/internal/harness/fake"
	"github.com/brickpop/foreman/internal/home"
	"github.com/brickpop/foreman/internal/profiles"
	"github.com/brickpop/foreman/internal/runner"
	"github.com/brickpop/foreman/internal/runner/direct"
	"github.com/brickpop/foreman/internal/sessionlog"
	"github.com/brickpop/foreman/internal/store"
	"github.com/brickpop/foreman/internal/teamtools"
	"github.com/brickpop/foreman/internal/terminal"
)

var ctx = context.Background()

const teamFile = `team = "atlas"
name = "Atlas"
lead = "mara"

[agents.mara]
name = "Mara"
role = "team_lead"

[agents.tove]
name = "Tove"
role = "engineer"
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
	t        *testing.T
	root     string
	m        *Manager
	db       *store.DB
	hm       *home.Home
	deps     Deps
	mu       sync.Mutex
	events   []events.Event
	profiles profiles.Store // the CEO's profiles: default is the fake harness
}

// newFixture makes a home (.foreman in the CEO's home folder, root) with
// project ledger (one code repo, root/dev/ledger-api) and team atlas, whose
// harness is a shell script: script is its body.
func newFixture(t *testing.T, script string, maxSessions int) *fixture {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	root := t.TempDir()
	t.Setenv("HOME", root)
	hm, err := home.Open(filepath.Join(root, ".foreman"))
	if err != nil {
		t.Fatal(err)
	}
	api := filepath.Join(root, "dev", "ledger-api")
	gitRun(t, "init", "--quiet", "--initial-branch=main", api)
	write(t, filepath.Join(api, "README"), "hello\n", 0o644)
	gitRun(t, "-C", api, "add", "README")
	gitRun(t, "-C", api, "commit", "--quiet", "-m", "first")
	write(t, filepath.Join(root, "foreman", "hq", "teams", "atlas.toml"), teamFile, 0o644)
	write(t, hm.Path("settings.toml"), fmt.Sprintf("max_sessions = %d\n", maxSessions), 0o644)
	db, err := store.Open(ctx, hm.Path("state.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := hm.Load(ctx, home.Deps{Git: git.New(), Projects: db.Projects()}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := hm.NewProject(ctx, "ledger", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := hm.AddRepo(ctx, "ledger", "ledger-api", api, ""); err != nil {
		t.Fatal(err)
	}
	program := filepath.Join(t.TempDir(), "harness.sh")
	if err := fake.WriteProgram(program, script); err != nil {
		t.Fatal(err)
	}
	f := &fixture{t: t, root: root, db: db, hm: hm}
	logs := sessionlog.Dir{Root: hm.Path("logs")}
	f.profiles = profiles.Store{Root: t.TempDir()}
	if err := f.profiles.Ensure("claude-code"); err != nil {
		t.Fatal(err)
	}
	f.deps = Deps{
		Home: hm.Root(), Runner: direct.Runner{}, Images: direct.Images{}, Git: git.New(), Config: hm,
		Deployments: db.Deployments(), Instances: db.Instances(),
		Adapters: map[string]harness.Adapter{"claude-code": fake.Adapter{Program: program}},
		Profile:  f.profileOf,
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

// waitDeployment waits for the deployment id to reach the status want and
// returns it. A deployment stops in a later step than its agents, so a test
// that reads it must wait for it, not for an agent.
func (f *fixture) waitDeployment(id, want string) store.Deployment {
	f.t.Helper()
	var d store.Deployment
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		var err error
		if d, err = f.db.Deployments().Get(ctx, id); err != nil {
			f.t.Fatal(err)
		}
		if d.Status == want {
			return d
		}
	}
	f.t.Fatalf("deployment %s is %s, never %s", id, d.Status, want)
	return d
}

// waitEvent waits for the last event of type typ about the deployment id, and
// returns the deployment it carries.
func (f *fixture) waitEvent(typ, id string) store.Deployment {
	f.t.Helper()
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		f.mu.Lock()
		var found *store.Deployment
		for _, e := range f.events {
			if dep, ok := e.Data.(store.Deployment); ok && e.Type == typ && dep.ID == id {
				found = &dep
			}
		}
		f.mu.Unlock()
		if found != nil {
			return *found
		}
	}
	f.t.Fatalf("no %s event for %s", typ, id)
	return store.Deployment{}
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

// Only the lead starts, with no opener: the CEO speaks first (the goal is in
// the wiki, not a message). A member sleeps, with no sandbox, until woken.
func TestDeployStartsTheLeadAndMembersWhenWoken(t *testing.T) {
	f := newFixture(t, `echo "opener: [$1]"; `+keepsRunning, 12)
	dep, err := f.m.Deploy(ctx, "atlas", "ledger", false)
	if err != nil {
		t.Fatal(err)
	}
	if dep.ID != "atlas@ledger" {
		t.Errorf("deployment %+v", dep)
	}
	f.waitStatus("mara@ledger", Running)
	if got := f.output("mara@ledger", "# You are"); !strings.Contains(got, "opener: []") {
		t.Errorf("the lead's opener: %q", got)
	}
	time.Sleep(200 * time.Millisecond)
	if s := f.status("tove@ledger"); s.Status != Asleep || !s.StartedAt.IsZero() {
		t.Errorf("tove before any work: %+v", s)
	}
	if _, err := os.Stat(f.hm.Sandbox("ledger", "tove")); !os.IsNotExist(err) {
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
		if inst := f.status(id); inst.StartedAt.IsZero() || inst.SandboxPath != f.hm.Sandbox("ledger", strings.Split(id, "@")[0]) {
			t.Errorf("%s: %+v", id, inst)
		}
	}
	// Without the agent link it can't open a repo: it gets them all, idle on no branch.
	for _, agent := range []string{"mara", "tove"} {
		if b := f.branch(filepath.Join(f.hm.Sandbox("ledger", agent), "ledger-api")); b != "HEAD" {
			t.Errorf("%s's worktree on %q, not idle", agent, b)
		}
	}
	prompt, _ := os.ReadFile(f.hm.Path("harness", "ledger", "tove", "fake", "PROMPT.md"))
	if !strings.Contains(string(prompt), "hq/wiki/backlog/index.md, kept by your lead") || strings.Contains(string(prompt), "## Tools") {
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
	if _, err := f.m.Deploy(ctx, "atlas", "ledger", false); err != nil {
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
	if _, err := f.m.Deploy(ctx, "nope", "ledger", false); !errors.Is(err, ErrInvalid) {
		t.Errorf("unknown team: %v", err)
	}
	if _, err := f.m.Deploy(ctx, "atlas", "nope", false); !errors.Is(err, ErrInvalid) {
		t.Errorf("unknown project: %v", err)
	}
	if _, err := f.m.Deploy(ctx, "atlas", "ledger", false); err != nil {
		t.Fatal(err)
	}
	if _, err := f.m.Deploy(ctx, "atlas", "ledger", false); !errors.Is(err, store.ErrActive) {
		t.Errorf("deployed twice: %v", err)
	}
	// The project's profile decides what its agents run on: one that doesn't
	// exist, one whose harness isn't supported, one not connected; each
	// refused before anything is created.
	write(t, f.hm.HQ("teams", "orion.toml"),
		strings.NewReplacer(`team = "atlas"`, `team = "orion"`, `"Atlas"`, `"Orion"`, "mara", "nora", "Mara", "Nora", "tove", "ivo", "Tove", "Ivo").Replace(teamFile), 0o644)
	if err := f.hm.Load(ctx, home.Deps{Git: git.New(), Projects: f.db.Projects()}); err != nil {
		t.Fatal(err)
	}
	f.useProfile("ledger", "nope")
	if _, err := f.m.Deploy(ctx, "orion", "ledger", false); !errors.Is(err, ErrInvalid) || !strings.Contains(err.Error(), "no such profile: nope") {
		t.Errorf("an unknown profile: %v", err)
	}
	if _, err := f.profiles.Create("other", "codex"); err != nil {
		t.Fatal(err)
	}
	f.useProfile("ledger", "other")
	if _, err := f.m.Deploy(ctx, "orion", "ledger", false); err == nil || !strings.Contains(err.Error(), `harness "codex" isn't supported yet`) {
		t.Errorf("an unsupported harness: %v", err)
	}
	f.m.Adapters["codex"] = needsLogin{f.deps.Adapters["claude-code"].(fake.Adapter)}
	if _, err := f.m.Deploy(ctx, "orion", "ledger", false); !errors.Is(err, ErrInvalid) || !strings.Contains(err.Error(), "profile other isn't connected yet") {
		t.Errorf("a profile without a login: %v", err)
	}
	if _, err := f.db.Deployments().Get(ctx, "orion@ledger"); err == nil {
		t.Error("a refused deployment was stored")
	}
	// Logged in: the profile's folder is the one checked, and the one the
	// session is given; its agents run on its harness.
	write(t, filepath.Join(f.profiles.Dir("other"), "login"), "", 0o600)
	if _, err := f.m.Deploy(ctx, "orion", "ledger", false); err != nil {
		t.Fatal(err)
	}
	if nora := f.waitStatus("nora@ledger", Running); nora.Harness != "codex" {
		t.Errorf("nora runs on %q", nora.Harness)
	}
	// A change of profile reaches an agent deployed before it at its next
	// start: ivo, asleep since orion's deploy on other.
	if ivo := f.status("ivo@ledger"); ivo.Harness != "codex" {
		t.Fatalf("ivo deployed on %q", ivo.Harness)
	}
	f.useProfile("ledger", "default")
	if err := f.m.Wake(ctx, "ivo@ledger"); err != nil {
		t.Fatal(err)
	}
	if ivo := f.waitStatus("ivo@ledger", Running); ivo.Harness != "claude-code" {
		t.Errorf("ivo started on %q", ivo.Harness)
	}
}

// A project has one team at most: deploying another replaces it, stopped as
// by Stop (its worktrees kept, its stop announced); the same team again is
// refused, and the team stays free to work on other projects.
func TestDeployReplacesTheTeamOnTheProject(t *testing.T) {
	f := newFixture(t, keepsRunning, 12)
	write(t, f.hm.HQ("teams", "orion.toml"),
		strings.NewReplacer(`team = "atlas"`, `team = "orion"`, `"Atlas"`, `"Orion"`, "mara", "nora", "Mara", "Nora", "tove", "ivo", "Tove", "Ivo").Replace(teamFile), 0o644)
	if err := f.hm.Load(ctx, home.Deps{Git: git.New(), Projects: f.db.Projects()}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.m.Deploy(ctx, "atlas", "ledger", false); err != nil {
		t.Fatal(err)
	}
	f.waitStatus("mara@ledger", Running)
	if _, err := f.m.Deploy(ctx, "orion", "ledger", false); err != nil {
		t.Fatal(err)
	}
	f.waitStatus("nora@ledger", Running)
	if d, _ := f.db.Deployments().Get(ctx, "atlas@ledger"); d.Status != store.DeploymentStopped || d.StopReason != "" {
		t.Errorf("atlas after orion's deploy: %+v", d)
	}
	for _, id := range []string{"mara@ledger", "tove@ledger"} {
		if s := f.status(id); s.Status != Stopped {
			t.Errorf("%s after orion's deploy: %s", id, s.Status)
		}
	}
	if _, err := os.Stat(f.hm.Sandbox("ledger", "mara")); err != nil {
		t.Errorf("the replaced lead's worktrees: %v", err)
	}
	f.mu.Lock()
	var stopped, created int
	for _, e := range f.events {
		if dep, ok := e.Data.(store.Deployment); ok {
			switch {
			case e.Type == "deployment.stopped" && dep.ID == "atlas@ledger":
				stopped++
			case e.Type == "deployment.created" && dep.ID == "orion@ledger" && stopped == 1:
				created++ // after the stop
			}
		}
	}
	f.mu.Unlock()
	if stopped != 1 || created != 1 {
		t.Errorf("events: %d atlas stops, %d orion deploys after it", stopped, created)
	}
	if _, err := f.m.Deploy(ctx, "orion", "ledger", false); !errors.Is(err, store.ErrActive) {
		t.Errorf("orion twice on ledger: %v", err)
	}
	// A refused deploy replaces nothing.
	if _, err := f.m.Deploy(ctx, "nope", "ledger", false); !errors.Is(err, ErrInvalid) {
		t.Errorf("an unknown team: %v", err)
	}
	if d, _ := f.db.Deployments().Get(ctx, "orion@ledger"); d.Status != store.DeploymentActive {
		t.Errorf("orion after a refused deploy: %+v", d)
	}
	// Atlas comes back, replacing orion in turn, on its old row.
	if _, err := f.m.Deploy(ctx, "atlas", "ledger", false); err != nil {
		t.Fatal(err)
	}
	f.waitStatus("mara@ledger", Running)
	if d, _ := f.db.Deployments().Get(ctx, "orion@ledger"); d.Status != store.DeploymentStopped {
		t.Errorf("orion after atlas's return: %+v", d)
	}
}

// needsLogin is connected when its login folder holds a file named login,
// and starts only with that folder.
type needsLogin struct{ fake.Adapter }

func (needsLogin) Connected(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, "login"))
	return err == nil
}

func (a needsLogin) Launch(s harness.Session) (harness.Launch, error) {
	if !a.Connected(s.Login) {
		return harness.Launch{}, harness.ErrNotConnected
	}
	return a.Adapter.Launch(s)
}

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
	if _, err := f.m.Deploy(ctx, "atlas", "ledger", false); err != nil {
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
	write(t, f.hm.HQ("teams", "orion.toml"),
		strings.NewReplacer(`team = "atlas"`, `team = "orion"`, `"Atlas"`, `"Orion"`, "mara", "nora", "Mara", "Nora", "tove", "ivo", "Tove", "Ivo").Replace(teamFile), 0o644)
	if err := f.hm.Load(ctx, home.Deps{Git: git.New(), Projects: f.db.Projects()}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.m.Deploy(ctx, "orion", "ledger", false); err != nil {
		t.Fatal(err)
	}
	f.waitStatus("nora@ledger", Running)
}

func TestStopKeepsTheWorktrees(t *testing.T) {
	f := newFixture(t, keepsRunning, 12)
	if active, err := f.m.Active(ctx, "atlas@ledger"); err != nil || active {
		t.Errorf("active before any deploy: %v %v", active, err)
	}
	if _, err := f.m.Deploy(ctx, "atlas", "ledger", false); err != nil {
		t.Fatal(err)
	}
	if active, err := f.m.Active(ctx, "atlas@ledger"); err != nil || !active {
		t.Errorf("not active once deployed: %v %v", active, err)
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
	if active, err := f.m.Active(ctx, "atlas@ledger"); err != nil || active {
		t.Errorf("active once stopped: %v %v", active, err)
	}
	if _, err := os.Stat(filepath.Join(f.hm.Sandbox("ledger", "tove"), "ledger-api", "README")); err != nil {
		t.Error("stopping removed the worktree")
	}
	// Redeploying starts again, fresh.
	if _, err := f.m.Deploy(ctx, "atlas", "ledger", false); err != nil {
		t.Fatal(err)
	}
	f.waitStatus("mara@ledger", Running)
}

// Beyond max_sessions, instances wait their turn.
func TestTheConcurrencyLimitQueues(t *testing.T) {
	f := newFixture(t, `read line; exit 1`, 1)
	if _, err := f.m.Deploy(ctx, "atlas", "ledger", false); err != nil {
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

// With a slot free, an agent goes straight from asleep to starting: never
// queued, which would warn the CEO about slots for nothing.
func TestAFreeSlotNeverShowsQueued(t *testing.T) {
	f := newFixture(t, keepsRunning, 12)
	if _, err := f.m.Deploy(ctx, "atlas", "ledger", false); err != nil {
		t.Fatal(err)
	}
	f.waitStatus("mara@ledger", Running)
	if err := f.m.Wake(ctx, "tove@ledger"); err != nil {
		t.Fatal(err)
	}
	f.waitStatus("tove@ledger", Running)
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, e := range f.events {
		if c, ok := e.Data.(StatusChange); ok && c.Status == Queued {
			t.Errorf("%s was queued with slots free", c.ID)
		}
	}
}

// Raising the limit starts who waited, without anything else happening.
func TestARaisedLimitStartsTheQueued(t *testing.T) {
	f := newFixture(t, keepsRunning, 1)
	if _, err := f.m.Deploy(ctx, "atlas", "ledger", false); err != nil {
		t.Fatal(err)
	}
	f.waitStatus("mara@ledger", Running)
	_ = f.m.Wake(ctx, "tove@ledger")
	f.waitStatus("tove@ledger", Queued)
	settings := f.hm.Settings()
	s := settings.Settings
	s.MaxSessions = 2
	if _, err := f.hm.SaveSettings(ctx, s, settings.Version); err != nil {
		t.Fatal(err)
	}
	f.m.LimitChanged()
	f.waitStatus("tove@ledger", Running)
}

// A crash restarts once; a second one stops the instance. The fake harness
// keeps no conversation, so its restart starts fresh, with no opener either time
// (Claude Code's continues instead: its adapter's tests).
func TestACrashRestartsOnceThenStops(t *testing.T) {
	f := newFixture(t, `echo "[$1]" >> "$FOREMAN_PROMPT_FILE.openers"; sleep 0.2; exit 3`, 12)
	// Workflow hears of a session starting fresh, not of a restart that
	// continues its conversation.
	var mu sync.Mutex
	var started []string
	f.deps.Started = func(_ context.Context, inst store.Instance, lead bool) {
		mu.Lock()
		defer mu.Unlock()
		started = append(started, fmt.Sprintf("%s lead=%v", inst.ID, lead))
	}
	f.m = New(f.deps)
	if _, err := f.m.Deploy(ctx, "atlas", "ledger", false); err != nil {
		t.Fatal(err)
	}
	inst := f.waitStatus("mara@ledger", Stopped)
	mu.Lock()
	if got := strings.Join(started, "; "); got != "mara@ledger lead=true" {
		t.Errorf("started %q", got)
	}
	mu.Unlock()
	if inst.Restarts != 1 {
		t.Errorf("restarts %d, want 1", inst.Restarts)
	}
	if inst.StopReason != "it exited twice in a row (code 3)" {
		t.Errorf("stop reason %q", inst.StopReason)
	}
	openers, _ := os.ReadFile(f.hm.Path("harness", "ledger", "mara", "fake", "PROMPT.md.openers"))
	if string(openers) != "[]\n[]\n" {
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
	dep, err := f.db.Deployments().Deploy(ctx, "atlas", "ledger")
	if err != nil {
		t.Fatal(err)
	}
	for agent, status := range map[string]string{"mara": Running, "tove": Running, "ivo": Asleep} {
		_ = f.db.Instances().Put(ctx, store.Instance{
			ID: agent + "@ledger", DeploymentID: dep.ID, AgentID: agent, Status: status, Harness: "claude-code",
			SandboxPath: f.hm.Sandbox("ledger", agent), StartedAt: time.Now().Add(-time.Hour),
		})
	}
	start := func(id, home string) runner.Process {
		labels := map[string]string{"foreman.instance": id}
		if home != "" {
			labels[HomeLabel] = home
		}
		p, err := (direct.Runner{}).Start(ctx, runner.Spec{Command: []string{"/bin/sh", "-c", "cat"}, Labels: labels})
		if err != nil {
			t.Fatal(err)
		}
		// Another home's are never stopped by the manager: never left running either.
		t.Cleanup(func() { _ = p.Stop(time.Second) })
		return p
	}
	survivor, orphan := start("mara@ledger", f.hm.Root()), start("ghost@ledger", f.hm.Root())
	// Another home on the machine, with the same names, and unlabelled ones:
	// never touched.
	theirs, theirOrphan := start("tove@ledger", "/elsewhere"), start("ghost2@ledger", "/elsewhere")
	unlabelled := start("ghost3@ledger", "")
	t.Cleanup(func() {
		for _, p := range []runner.Process{theirs, theirOrphan, unlabelled} {
			_ = p.Stop(time.Second)
		}
	})
	f.deps.Runner = listing{listed: []runner.Process{survivor, orphan, theirs, theirOrphan, unlabelled}}
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
	// tove was running, with no process of this home's: it restarts.
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
	if _, err := f.m.Deploy(ctx, "atlas", "ledger", false); err != nil {
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

func TestDeploymentsIn(t *testing.T) {
	f := newFixture(t, keepsRunning, 12)
	if list, err := f.m.DeploymentsIn(ctx, store.DeploymentActive); err != nil || len(list) != 0 {
		t.Errorf("none yet: %v %v", list, err)
	}
	if _, err := f.m.Deploy(ctx, "atlas", "ledger", false); err != nil {
		t.Fatal(err)
	}
	if list, _ := f.m.DeploymentsIn(ctx, store.DeploymentActive); len(list) != 1 || list[0].ID != "atlas@ledger" {
		t.Errorf("active: %+v", list)
	}
	if list, _ := f.m.DeploymentsIn(ctx, store.DeploymentStopped); len(list) != 0 {
		t.Errorf("stopped while active: %+v", list)
	}
	_ = f.m.Stop(ctx, "atlas@ledger")
	if list, _ := f.m.DeploymentsIn(ctx, store.DeploymentActive); len(list) != 0 {
		t.Errorf("after stop: %+v", list)
	}
	if list, _ := f.m.DeploymentsIn(ctx, store.DeploymentStopped); len(list) != 1 || list[0].ID != "atlas@ledger" {
		t.Errorf("stopped: %+v", list)
	}
}

// The prompt shown is the one the harness got.
func TestPromptIsWhatTheHarnessGot(t *testing.T) {
	f := newFixture(t, keepsRunning, 12)
	if _, err := f.m.Deploy(ctx, "atlas", "ledger", false); err != nil {
		t.Fatal(err)
	}
	_ = f.m.Wake(ctx, "tove@ledger")
	f.waitStatus("tove@ledger", Running)
	shown, err := f.m.Prompt(ctx, "tove@ledger")
	if err != nil {
		t.Fatal(err)
	}
	given, _ := os.ReadFile(f.hm.Path("harness", "ledger", "tove", "fake", "PROMPT.md"))
	if shown != string(given) || !strings.Contains(shown, "hq/wiki/backlog/index.md") {
		t.Errorf("shown:\n%s\ngiven:\n%s", shown, given)
	}
	if _, err := f.m.Prompt(ctx, "nobody@ledger"); err == nil {
		t.Error("a prompt for no instance")
	}
}

// The CEO's instructions (Foreman's HQ's instructions.md) reach the harness
// at the next start.
func TestTheCEOsInstructionsReachTheHarness(t *testing.T) {
	f := newFixture(t, keepsRunning, 12)
	if _, err := f.hm.SaveInstructions(ctx, "Answer in Catalan.", f.hm.Instructions().Version); err != nil {
		t.Fatal(err)
	}
	if _, err := f.m.Deploy(ctx, "atlas", "ledger", false); err != nil {
		t.Fatal(err)
	}
	_ = f.m.Wake(ctx, "tove@ledger")
	f.waitStatus("tove@ledger", Running)
	given, _ := os.ReadFile(f.hm.Path("harness", "ledger", "tove", "fake", "PROMPT.md"))
	if !strings.HasSuffix(string(given), "\nAnswer in Catalan.\n") {
		t.Errorf("given:\n%s", given)
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
	if _, err := f.m.Deploy(ctx, "atlas", "ledger", false); err != nil {
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

// A harness that reports it is at its prompt while Foreman is still starting
// it (before its run is marked running) is heard: it is idle, not left running
// until a turn nobody starts, with its messages waiting.
func TestAHookWhileStartingIsKept(t *testing.T) {
	f := newFixture(t, keepsRunning, 12)
	f.deps.Adapters = map[string]harness.Adapter{"claude-code": eventIsActivity{f.deps.Adapters["claude-code"].(fake.Adapter)}}
	open := f.deps.OpenLog
	f.deps.OpenLog = func(id string) (terminal.Log, error) {
		if id == "mara@ledger" {
			f.m.Hook(id, "idle", nil) // its process runs; the start isn't done
		}
		return open(id)
	}
	f.m = New(f.deps)
	if _, err := f.m.Deploy(ctx, "atlas", "ledger", false); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for f.status("mara@ledger").StartedAt.IsZero() && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if s := f.status("mara@ledger"); s.StartedAt.IsZero() || s.Status != Idle {
		t.Errorf("after its start: %s (started %v)", s.Status, s.StartedAt)
	}
}

// eventIsLine describes a hook event by its name.
type eventIsLine struct{ fake.Adapter }

func (eventIsLine) Describe(event string, _ []byte) string { return event }

// Hook events give a running instance its activity line: one line of text,
// sent as an event and listed with the instance; a new run starts without one.
func TestHooksBecomeActivityLines(t *testing.T) {
	f := newFixture(t, keepsRunning, 12)
	f.deps.Adapters = map[string]harness.Adapter{"claude-code": eventIsLine{f.deps.Adapters["claude-code"].(fake.Adapter)}}
	f.m = New(f.deps)
	if _, err := f.m.Deploy(ctx, "atlas", "ledger", false); err != nil {
		t.Fatal(err)
	}
	f.waitStatus("mara@ledger", Running)
	activity := func(id string) *store.Doing {
		t.Helper()
		list, err := f.m.List(ctx)
		if err != nil {
			t.Fatal(err)
		}
		for _, i := range list {
			if i.ID == id {
				return i.Doing
			}
		}
		t.Fatalf("no %s", id)
		return nil
	}
	if a := activity("mara@ledger"); a != nil {
		t.Errorf("an activity before any hook: %+v", a)
	}
	f.m.Hook("mara@ledger", "  Bash: go test\x1b[31m ./...\n  echo done", nil)
	f.m.Hook("mara@ledger", "", nil)        // says nothing new: the line stays
	f.m.Hook("tove@ledger", "Reading", nil) // asleep: no line
	a := activity("mara@ledger")
	if a == nil || a.Line != "Bash: go test[31m ./..." || time.Since(a.At) > time.Minute {
		t.Errorf("mara %+v", a)
	}
	if a := activity("tove@ledger"); a != nil {
		t.Errorf("tove %+v", a)
	}
	f.mu.Lock()
	var sent []DoingChange
	for _, e := range f.events {
		if e.Type == "instance.doing" {
			sent = append(sent, e.Data.(DoingChange))
		}
	}
	f.mu.Unlock()
	if len(sent) != 1 || sent[0].ID != "mara@ledger" || sent[0].Doing != *a {
		t.Errorf("events %+v", sent)
	}
	f.m.Hook("mara@ledger", strings.Repeat("é", 300), nil)
	if r := []rune(activity("mara@ledger").Line); len(r) != doingMax || r[len(r)-1] != '…' {
		t.Errorf("a long line kept %d characters", len(r))
	}
	if err := f.m.Stop(ctx, "atlas@ledger"); err != nil {
		t.Fatal(err)
	}
	if _, err := f.m.Deploy(ctx, "atlas", "ledger", false); err != nil {
		t.Fatal(err)
	}
	f.waitStatus("mara@ledger", Running)
	if a := activity("mara@ledger"); a != nil {
		t.Errorf("the last run's line in the new run: %+v", a)
	}
}

// The harness gets foreman-agent's place and the instance's own token. With
// the tools, it opens repos itself: its start makes no worktree, and no branch.
func TestTheHarnessGetsTheAgentLink(t *testing.T) {
	f := newFixture(t, `echo "link=$FOREMAN_AGENT_URL $FOREMAN_AGENT_TOKEN"; cat`, 12)
	f.deps.Agent = AgentLink{Host: "/bin/foreman-agent", Path: "/opt/foreman/foreman-agent", URL: "http://h:8889",
		Token: func(id string) string { return "token-of-" + id }}
	f.m = New(f.deps)
	if _, err := f.m.Deploy(ctx, "atlas", "ledger", false); err != nil {
		t.Fatal(err)
	}
	mara := f.waitStatus("mara@ledger", Running)
	f.output("mara@ledger", "link=http://h:8889 token-of-mara@ledger")
	if _, err := os.Stat(filepath.Join(mara.SandboxPath, "ledger-api")); !os.IsNotExist(err) {
		t.Errorf("a repo checked out at start: %v", err)
	}
	repo := filepath.Join(f.root, "dev", "ledger-api")
	if out, _ := exec.Command("git", "-C", repo, "for-each-ref", "refs/heads/wt/", "refs/heads/lead/").Output(); len(out) > 0 {
		t.Errorf("branches made at start: %s", out)
	}
}

// A run frees only its own slot: the late end of a stopped run must not free
// the run a redeploy started in its place (seen as a flaky redeploy).
func TestARunReleasesOnlyItsOwnSlot(t *testing.T) {
	m := New(Deps{})
	old, current := &run{}, &run{}
	m.runs["mara@ledger"] = current
	m.release("mara@ledger", old)
	if m.runs["mara@ledger"] != current {
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

// failingSync records the submodule syncs and fails each, as offline.
type failingSync struct {
	Git
	mu     sync.Mutex
	synced []string
}

func (g *failingSync) SyncSubmodules(_ context.Context, dir string, recursive bool) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.synced = append(g.synced, fmt.Sprintf("%s recursive=%v", dir, recursive))
	return errors.New("offline")
}

// Each worktree gets its submodules, as the project says; one that can't be
// fetched doesn't keep the agent from starting.
func TestAStartChecksOutTheSubmodulesOrGoesWithout(t *testing.T) {
	f := newFixture(t, keepsRunning, 12)
	write(t, filepath.Join(f.root, "foreman", "projects", "ledger", "hq", "project.toml"), "recursive_submodules = true\n", 0o644)
	if err := f.hm.Load(ctx, home.Deps{Git: git.New(), Projects: f.db.Projects()}); err != nil {
		t.Fatal(err)
	}
	g := &failingSync{Git: f.deps.Git}
	f.deps.Git = g
	f.m = New(f.deps)
	if _, err := f.m.Deploy(ctx, "atlas", "ledger", false); err != nil {
		t.Fatal(err)
	}
	f.waitStatus("mara@ledger", Running)
	g.mu.Lock()
	defer g.mu.Unlock()
	want := []string{filepath.Join(f.hm.Sandbox("ledger", "mara"), "ledger-api") + " recursive=true"}
	if !slices.Equal(g.synced, want) {
		t.Errorf("synced %q, want %q", g.synced, want)
	}
}

// A stop while the lead is still starting, then a redeploy at once: the new
// run starts, and the old start's late end changes nothing.
func TestRedeployWhileTheStoppedRunIsStillStarting(t *testing.T) {
	f := newFixture(t, keepsRunning, 12)
	slow := &slowGit{Git: f.deps.Git, entered: make(chan struct{}), release: make(chan struct{})}
	f.deps.Git = slow
	f.m = New(f.deps)
	t.Cleanup(func() { closeOnce(slow.release) })
	if _, err := f.m.Deploy(ctx, "atlas", "ledger", false); err != nil {
		t.Fatal(err)
	}
	<-slow.entered
	if err := f.m.Stop(ctx, "atlas@ledger"); err != nil {
		t.Fatal(err)
	}
	if _, err := f.m.Deploy(ctx, "atlas", "ledger", false); err != nil {
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

// A project with no code repo (a research team's) deploys: its agents work in
// the HQ, and the lead's prompt offers commit but no git tools.
func TestDeployAProjectWithoutRepos(t *testing.T) {
	f := newFixture(t, keepsRunning, 12)
	f.deps.Agent = AgentLink{Host: "/bin/foreman-agent", Path: "/opt/foreman/foreman-agent", URL: "http://h:8889",
		Token: func(id string) string { return "token-of-" + id }}
	f.m = New(f.deps)
	if _, err := f.hm.RemoveRepo(ctx, "ledger", "ledger-api"); err != nil {
		t.Fatal(err)
	}
	if _, err := f.m.Deploy(ctx, "atlas", "ledger", false); err != nil {
		t.Fatal(err)
	}
	f.waitStatus("mara@ledger", Running)
	shown, err := f.m.Prompt(ctx, "mara@ledger")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(shown, "no code repo") || !strings.Contains(shown, "commit(") || strings.Contains(shown, "switch(") {
		t.Errorf("not a no-code prompt:\n%s", shown)
	}
}

// A hidden project is only out of sight: it takes a deployment as any other.
func TestAHiddenProjectDeploys(t *testing.T) {
	f := newFixture(t, keepsRunning, 12)
	if _, err := f.hm.SetHidden(ctx, "ledger", true); err != nil {
		t.Fatal(err)
	}
	if _, err := f.m.Deploy(ctx, "atlas", "ledger", false); err != nil {
		t.Errorf("hidden: %v", err)
	}
}

// A clean exit (the CEO's /exit) is done for now: asleep, holding no slot,
// until the CEO's Start (or work) wakes it, picking up its conversation.
func TestACleanExitSleepsAndStartResumes(t *testing.T) {
	f := newFixture(t, `echo "[$1]" >> "$FOREMAN_PROMPT_FILE.openers"; read line; exit 0`, 12)
	if _, err := f.m.Deploy(ctx, "atlas", "ledger", false); err != nil {
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
	if _, err := f.m.Deploy(ctx, "atlas", "ledger", false); err != nil {
		t.Fatal(err)
	}
	f.waitStatus("mara@ledger", Stopped)
	if err := f.m.Wake(ctx, "mara@ledger"); err != nil {
		t.Fatal(err)
	}
	if s := f.waitStatus("mara@ledger", Running); s.Restarts != 0 || s.StopReason != "" {
		t.Errorf("after Start again: restarts %d, stop reason %q", s.Restarts, s.StopReason)
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

// Processes of another home, and unlabelled ones, are left running by a
// restore.
func TestRestoreLeavesOtherHomesAlone(t *testing.T) {
	f := newFixture(t, keepsRunning, 12)
	start := func(id, home string) runner.Process {
		labels := map[string]string{"foreman.instance": id}
		if home != "" {
			labels[HomeLabel] = home
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
	for name, p := range map[string]runner.Process{"another home's": theirs, "an unlabelled": older} {
		done := make(chan struct{})
		go func() { _, _ = p.Wait(); close(done) }()
		select {
		case <-done:
			t.Errorf("%s process was stopped", name)
		case <-time.After(500 * time.Millisecond):
		}
	}
}

// Every process names its home, and its name is unique on the machine.
func TestProcessesNameTheirHome(t *testing.T) {
	a := New(Deps{Home: "/a/.foreman"})
	b := New(Deps{Home: "/b/.foreman"})
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
	if _, err := f.m.Deploy(ctx, "atlas", "ledger", false); !errors.Is(err, ErrNotReady) || !strings.Contains(err.Error(), "podman isn't running") {
		t.Fatalf("deploy: %v", err)
	}
	if deps, _ := f.db.Deployments().List(ctx); len(deps) != 0 {
		t.Fatalf("deployed anyway: %+v", deps)
	}
	blocked.Store(false)
	if _, err := f.m.Deploy(ctx, "atlas", "ledger", false); err != nil {
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
	if _, err := f.m.Deploy(ctx, "atlas", "ledger", false); err != nil {
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
	if _, err := f.m.Deploy(ctx, "atlas", "ledger", false); !errors.Is(err, ErrNotReady) {
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
	if _, err := f.m.Deploy(ctx, "atlas", "ledger", false); err != nil {
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
	if _, err := f.m.Deploy(ctx, "atlas", "ledger", false); err != nil {
		t.Fatal(err)
	}
	f.waitStatus("mara@ledger", Running)
	f.m.Shutdown(ctx)
	if s := f.status("mara@ledger"); s.Status != Running {
		t.Errorf("after a failed stop: %s", s.Status)
	}
}

// profileOf is the project's profile, as the composition root finds it.
func (f *fixture) profileOf(project string) (profiles.Profile, string, error) {
	pr, err := f.hm.Project(project)
	if err != nil {
		return profiles.Profile{}, "", err
	}
	p, err := f.profiles.Get(pr.Profile)
	return p, f.profiles.Dir(p.Name), err
}

// useProfile sets the project's profile.
func (f *fixture) useProfile(project, profile string) {
	f.t.Helper()
	if _, err := f.hm.SetProfile(ctx, project, profile); err != nil {
		f.t.Fatal(err)
	}
}

// recording is the fake harness, keeping what each launch was asked.
type recording struct {
	fake.Adapter
	mu       *sync.Mutex
	sessions *[]harness.Session
}

func (r recording) Launch(s harness.Session) (harness.Launch, error) {
	r.mu.Lock()
	*r.sessions = append(*r.sessions, s)
	r.mu.Unlock()
	return r.Adapter.Launch(s)
}

// A restart runs the agent on the project's profile as it is now: on the
// same harness it continues its conversation; on another it starts fresh.
// One that isn't up is left alone.
func TestRestartTakesTheProfileAsItIsNow(t *testing.T) {
	f := newFixture(t, keepsRunning, 12)
	var mu sync.Mutex
	var sessions []harness.Session
	last := func() harness.Session {
		mu.Lock()
		defer mu.Unlock()
		return sessions[len(sessions)-1]
	}
	base := f.deps.Adapters["claude-code"].(fake.Adapter)
	f.m.Adapters["claude-code"] = recording{base, &mu, &sessions}
	f.m.Adapters["codex"] = recording{base, &mu, &sessions}
	if _, err := f.m.Deploy(ctx, "atlas", "ledger", false); err != nil {
		t.Fatal(err)
	}
	if mara := f.waitStatus("mara@ledger", Running); mara.Profile != "default" || mara.Harness != "claude-code" {
		t.Fatalf("deployed on %q %q", mara.Profile, mara.Harness)
	}
	starts := func() int { mu.Lock(); defer mu.Unlock(); return len(sessions) }
	restart := func(want int) store.Instance {
		t.Helper()
		if err := f.m.Restart(ctx, "mara@ledger"); err != nil {
			t.Fatal(err)
		}
		deadline := time.Now().Add(5 * time.Second)
		for starts() < want && time.Now().Before(deadline) {
			time.Sleep(10 * time.Millisecond)
		}
		return f.waitStatus("mara@ledger", Running)
	}

	// Another account, the same harness: the conversation goes on.
	if _, err := f.profiles.Create("work", "claude-code"); err != nil {
		t.Fatal(err)
	}
	f.useProfile("ledger", "work")
	if mara := restart(2); mara.Profile != "work" || mara.Harness != "claude-code" || mara.Restarts != 0 {
		t.Errorf("on work: %+v", mara)
	}
	if s := last(); !s.Continue || s.Login != f.profiles.Dir("work") {
		t.Errorf("same harness: %+v", s)
	}
	// Another harness: fresh.
	if _, err := f.profiles.Create("other", "codex"); err != nil {
		t.Fatal(err)
	}
	f.useProfile("ledger", "other")
	if mara := restart(3); mara.Profile != "other" || mara.Harness != "codex" {
		t.Errorf("on other: %+v", mara)
	}
	if s := last(); s.Continue || s.Opener != "" {
		t.Errorf("another harness: %+v", s)
	}
	// Asleep: nothing to restart; it starts on the profile anyway.
	if err := f.m.Restart(ctx, "tove@ledger"); err != nil || f.status("tove@ledger").Status != Asleep || starts() != 3 {
		t.Errorf("an asleep agent restarted: %v", err)
	}
	if err := f.m.Restart(ctx, "nope@ledger"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("unknown: %v", err)
	}
}

// A redeploy continues each agent's last conversation on the project, as a
// restart does, unless deployed fresh. Only an agent that started counts,
// and only when the folder's last user was the same team: another team's
// agent of the same id starts fresh, and so does the first team after it.
func TestRedeployContinues(t *testing.T) {
	f := newFixture(t, keepsRunning, 12)
	var mu sync.Mutex
	var sessions []harness.Session
	launches := func(id string) []harness.Session {
		mu.Lock()
		defer mu.Unlock()
		var of []harness.Session
		for _, s := range sessions {
			if s.Instance == id {
				of = append(of, s)
			}
		}
		return of
	}
	f.m.Adapters["claude-code"] = recording{f.deps.Adapters["claude-code"].(fake.Adapter), &mu, &sessions}
	continuing := func(team string, want ...string) {
		t.Helper()
		got, err := f.m.Continuing(ctx, team, "ledger")
		if err != nil || !slices.Equal(got, want) {
			t.Errorf("%s continuing: %v %v, want %v", team, got, err, want)
		}
	}
	deploy := func(team string, fresh bool) {
		t.Helper()
		if _, err := f.m.Deploy(ctx, team, "ledger", fresh); err != nil {
			t.Fatal(err)
		}
	}
	stop := func(team string) {
		t.Helper()
		if err := f.m.Stop(ctx, team+"@ledger"); err != nil {
			t.Fatal(err)
		}
		f.waitStatus("mara@ledger", Stopped)
	}
	// The last launch of id continued (or not), and it is the n-th.
	started := func(id string, n int, cont bool) {
		t.Helper()
		f.waitStatus(id, Running)
		got := launches(id)
		if len(got) != n || got[n-1].Continue != cont {
			t.Errorf("%s: %d launches, last %+v; want %d, continuing %v", id, len(got), got[len(got)-1], n, cont)
		}
	}

	continuing("atlas") // never deployed
	deploy("atlas", false)
	started("mara@ledger", 1, false)
	stop("atlas")
	// Tove slept the whole deployment: nothing of hers to continue.
	continuing("atlas", "mara")
	deploy("atlas", false)
	started("mara@ledger", 2, true)
	if err := f.m.Wake(ctx, "tove@ledger"); err != nil {
		t.Fatal(err)
	}
	started("tove@ledger", 1, false)
	stop("atlas")

	continuing("atlas", "mara", "tove")
	deploy("atlas", true) // Start fresh
	started("mara@ledger", 3, false)
	// A restart after it continues the fresh conversation.
	if err := f.m.Restart(ctx, "mara@ledger"); err != nil {
		t.Fatal(err)
	}
	for deadline := time.Now().Add(5 * time.Second); len(launches("mara@ledger")) < 4 && time.Now().Before(deadline); {
		time.Sleep(10 * time.Millisecond)
	}
	started("mara@ledger", 4, true)
	stop("atlas")

	// Another team whose lead is also mara: the same harness folder.
	write(t, f.hm.HQ("teams", "orion.toml"),
		strings.NewReplacer(`team = "atlas"`, `team = "orion"`, `"Atlas"`, `"Orion"`, "tove", "ivo", "Tove", "Ivo").Replace(teamFile), 0o644)
	if err := f.hm.Load(ctx, home.Deps{Git: git.New(), Projects: f.db.Projects()}); err != nil {
		t.Fatal(err)
	}
	continuing("orion")
	deploy("orion", false)
	started("mara@ledger", 5, false)
	stop("orion")
	// Orion's mara used the folder last; tove slept since Start fresh, so
	// she is still to start fresh.
	continuing("atlas")
	deploy("atlas", false)
	started("mara@ledger", 6, false)
}

// specs is the Direct engine, keeping what each start was asked.
type specs struct {
	direct.Runner
	mu  sync.Mutex
	got []runner.Spec
}

func (s *specs) Start(ctx context.Context, spec runner.Spec) (runner.Process, error) {
	s.mu.Lock()
	s.got = append(s.got, spec)
	s.mu.Unlock()
	return s.Runner.Start(ctx, spec)
}

func (s *specs) last() runner.Spec {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.got[len(s.got)-1]
}

// An agent gets its sandbox, each repo's .git read-only at its real path
// (which its worktree's link names, a symlinked repo folder included), and
// the HQ at its real path: writable for the lead only, its .git
// read-only for all; the project's files (the inbox read-only, shared
// writable, both made); and its private harness folder. Nothing else of the
// machine, nor of the home.
func TestTheMountsAreTheSandboxAndTheReposOnly(t *testing.T) {
	f := newFixture(t, keepsRunning, 12)
	link := filepath.Join(f.root, "linked-api")
	if err := os.Symlink(filepath.Join(f.root, "dev", "ledger-api"), link); err != nil {
		t.Fatal(err)
	}
	if _, err := f.hm.LocateRepo(ctx, "ledger", "ledger-api", link); err != nil {
		t.Fatal(err)
	}
	rec := &specs{}
	f.deps.Runner = rec
	f.m = New(f.deps)
	if _, err := f.m.Deploy(ctx, "atlas", "ledger", false); err != nil {
		t.Fatal(err)
	}
	f.waitStatus("mara@ledger", Running)
	realAPI, _ := filepath.EvalSymlinks(filepath.Join(f.root, "dev", "ledger-api"))
	hq := filepath.Join(f.root, "foreman", "projects", "ledger", "hq")
	mara := f.hm.Sandbox("ledger", "mara")
	harnessDir := f.hm.Path("harness", "ledger", "mara")
	inbox := filepath.Join(f.root, "foreman", "projects", "ledger", "files", "inbox")
	shared := filepath.Join(f.root, "foreman", "projects", "ledger", "files", "shared")
	want := []runner.Mount{
		{Source: mara, Target: mara},
		{Source: filepath.Join(realAPI, ".git"), Target: filepath.Join(realAPI, ".git"), ReadOnly: true},
		// At its real path, as everything else: no mount point in the sandbox.
		{Source: hq, Target: hq},
		{Source: filepath.Join(hq, ".git"), Target: filepath.Join(hq, ".git"), ReadOnly: true},
		{Source: inbox, Target: inbox, ReadOnly: true},
		{Source: shared, Target: shared},
		{Source: harnessDir, Target: harnessDir},
	}
	if got := rec.last(); !slices.Equal(got.Mounts, want) || got.Labels[HomeLabel] != f.hm.Root() {
		t.Errorf("the lead's mounts:\n%+v\nwant\n%+v\nlabels %v", got.Mounts, want, got.Labels)
	}
	for _, dir := range []string{inbox, shared} {
		if info, err := os.Stat(dir); err != nil || !info.IsDir() {
			t.Errorf("%s isn't made before its mount: %v", dir, err)
		}
	}
	if info, err := os.Stat(harnessDir); err != nil || info.Mode().Perm() != 0o700 {
		t.Errorf("the harness folder is readable by others: %v", info.Mode())
	}
	if err := f.m.Wake(ctx, "tove@ledger"); err != nil {
		t.Fatal(err)
	}
	f.waitStatus("tove@ledger", Running)
	if got := rec.last(); got.Mounts[2] != (runner.Mount{Source: hq, Target: hq, ReadOnly: true}) {
		t.Errorf("a member's HQ: %+v", got.Mounts[2])
	}
}

// A repo that moved keeps its agents from starting until it is located:
// then their worktrees, made at the old place, are relinked and reused, with
// the agents' work in them.
func TestAMovedRepoStopsItsAgentsUntilLocated(t *testing.T) {
	f := newFixture(t, keepsRunning, 12)
	if _, err := f.m.Deploy(ctx, "atlas", "ledger", false); err != nil {
		t.Fatal(err)
	}
	f.waitStatus("mara@ledger", Running)
	worktree := filepath.Join(f.hm.Sandbox("ledger", "mara"), "ledger-api")
	write(t, filepath.Join(worktree, "work.txt"), "mara's\n", 0o644)
	if err := f.m.Stop(ctx, "atlas@ledger"); err != nil {
		t.Fatal(err)
	}
	moved := filepath.Join(f.root, "src", "ledger-api")
	if err := os.MkdirAll(filepath.Dir(moved), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(f.root, "dev", "ledger-api"), moved); err != nil {
		t.Fatal(err)
	}
	// Nothing reloaded the home: the start checks the repos itself.
	if _, err := f.m.Deploy(ctx, "atlas", "ledger", false); !errors.Is(err, ErrInvalid) || !strings.Contains(err.Error(), "is missing") {
		t.Fatalf("deploying with a missing repo: %v", err)
	}
	if _, err := f.hm.LocateRepo(ctx, "ledger", "ledger-api", moved); err != nil {
		t.Fatal(err)
	}
	if _, err := f.m.Deploy(ctx, "atlas", "ledger", false); err != nil {
		t.Fatal(err)
	}
	f.waitStatus("mara@ledger", Running)
	if b := f.branch(worktree); b != "HEAD" {
		t.Errorf("the relinked worktree's branch %q", b)
	}
	if data, _ := os.ReadFile(filepath.Join(worktree, "work.txt")); string(data) != "mara's\n" {
		t.Errorf("the agent's work: %q", data)
	}
}

// The same team on two projects that share a repo, at once: each agent has its
// own worktree on each (the CEO, 2026-11-03), idle on no branch, so nothing
// collides. Stopped on one, it starts again on the other all the same.
func TestATeamWorksOnTwoProjectsSharingARepo(t *testing.T) {
	f := newFixture(t, keepsRunning, 12)
	if _, _, err := f.hm.NewProject(ctx, "suite", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := f.hm.AddRepo(ctx, "suite", "api", filepath.Join(f.root, "dev", "ledger-api"), ""); err != nil {
		t.Fatal(err)
	}
	write(t, f.hm.HQ("teams", "orion.toml"),
		strings.NewReplacer(`team = "atlas"`, `team = "orion"`, `"Atlas"`, `"Orion"`, "mara", "nora", "Mara", "Nora", "tove", "ivo", "Tove", "Ivo").Replace(teamFile), 0o644)
	if err := f.hm.Load(ctx, home.Deps{Git: git.New(), Projects: f.db.Projects()}); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"ledger", "suite"} {
		if _, err := f.m.Deploy(ctx, "atlas", p, false); err != nil {
			t.Fatalf("atlas on %s: %v", p, err)
		}
	}
	ledger, suite := f.waitStatus("mara@ledger", Running), f.waitStatus("mara@suite", Running)
	if b := f.branch(filepath.Join(ledger.SandboxPath, "ledger-api")); b != "HEAD" {
		t.Errorf("on ledger: %q", b)
	}
	if b := f.branch(filepath.Join(suite.SandboxPath, "api")); b != "HEAD" {
		t.Errorf("on suite: %q", b)
	}
	// Stopped on ledger, its worktrees stay; on suite, its agents start anyway.
	if err := f.m.Stop(ctx, "atlas@ledger"); err != nil {
		t.Fatal(err)
	}
	if err := f.m.Wake(ctx, "tove@suite"); err != nil {
		t.Fatal(err)
	}
	tove := f.waitStatus("tove@suite", Running)
	if b := f.branch(filepath.Join(tove.SandboxPath, "api")); b != "HEAD" {
		t.Errorf("tove on suite: %q", b)
	}
	// atlas replaced on suite: another team's agents get their own branches there.
	if _, err := f.m.Deploy(ctx, "orion", "suite", false); err != nil {
		t.Errorf("another team: %v", err)
	}
	f.waitStatus("nora@suite", Running)
}

// A project whose code repos all vanish gets a recheck before its deployment
// stops: back in time, nothing happens; still gone, it stops with the reason.
func TestAGoneProjectStopsAfterARecheck(t *testing.T) {
	f := newFixture(t, keepsRunning, 12)
	f.m.Recheck = 100 * time.Millisecond
	if _, err := f.m.Deploy(ctx, "atlas", "ledger", false); err != nil {
		t.Fatal(err)
	}
	f.waitStatus("mara@ledger", Running)
	repo := filepath.Join(f.root, "dev", "ledger-api")
	away := repo + "-away"
	if err := os.Rename(repo, away); err != nil {
		t.Fatal(err)
	}
	f.m.ProjectsChanged(ctx)
	f.m.ProjectsChanged(ctx) // one recheck per project, however often called
	if err := os.Rename(away, repo); err != nil {
		t.Fatal(err)
	}
	time.Sleep(300 * time.Millisecond)
	if d, _ := f.db.Deployments().Get(ctx, "atlas@ledger"); d.Status != store.DeploymentActive {
		t.Fatalf("stopped though the repo came back: %+v", d)
	}
	if err := os.Rename(repo, away); err != nil {
		t.Fatal(err)
	}
	f.m.ProjectsChanged(ctx)
	f.waitStatus("mara@ledger", Stopped)
	// The deployment stops, and its event is sent, after its agents: wait for each.
	d := f.waitDeployment("atlas@ledger", store.DeploymentStopped)
	if d.StopReason != "ledger is gone: all its code repos are missing" {
		t.Errorf("deployment %+v", d)
	}
	if stopped := f.waitEvent("deployment.stopped", "atlas@ledger"); stopped.StopReason != d.StopReason {
		t.Errorf("the event: %+v", stopped)
	}
}

// An agent mid-turn with no hook for stuck_after_minutes gets one card, and
// keeps running; any hook, or its run ending, resolves the card. Idle and
// awaiting agents are never stuck.
func TestAQuietAgentIsFlaggedStuck(t *testing.T) {
	f := newFixture(t, keepsRunning, 12)
	f.deps.Adapters = map[string]harness.Adapter{"claude-code": eventIsActivity{f.deps.Adapters["claude-code"].(fake.Adapter)}}
	f.deps.Requests = f.db.Requests()
	f.m = New(f.deps)
	if _, err := f.m.Deploy(ctx, "atlas", "ledger", false); err != nil {
		t.Fatal(err)
	}
	f.waitStatus("mara@ledger", Running)
	open := func() []store.Request { // the stuck cards (waiting raises its own)
		t.Helper()
		list, err := f.db.Requests().Open(ctx)
		if err != nil {
			t.Fatal(err)
		}
		return slices.DeleteFunc(list, func(r store.Request) bool { return r.Kind != "blocked" })
	}
	after := func(d time.Duration) { f.m.CheckStuck(ctx, time.Now().Add(d)) }

	after(19 * time.Minute)
	if list := open(); len(list) != 0 {
		t.Fatalf("flagged before 20 min: %+v", list)
	}
	after(21 * time.Minute)
	after(40 * time.Minute) // still one card
	list := open()
	if len(list) != 1 {
		t.Fatalf("cards: %+v", list)
	}
	r := list[0]
	if r.Kind != "blocked" || r.Source != "foreman" || r.InstanceID != "mara@ledger" || r.DeploymentID != "atlas@ledger" ||
		r.ProjectID != "ledger" || r.Summary != "Mara may be stuck: no activity for 20 min." || !slices.Equal(r.Options, []string{"Restart"}) {
		t.Errorf("card %+v", r)
	}
	if s := f.status("mara@ledger"); s.Status != Running || !f.m.Live("mara@ledger") {
		t.Errorf("a stuck agent was touched: %s", s.Status)
	}
	// Any hook resolves it, whatever it says.
	f.m.Hook("mara@ledger", "", nil)
	if list := open(); len(list) != 0 {
		t.Errorf("still open after a hook: %+v", list)
	}
	f.mu.Lock()
	var kinds []string
	for _, e := range f.events {
		if strings.HasPrefix(e.Type, "request.") {
			kinds = append(kinds, e.Type)
		}
	}
	f.mu.Unlock()
	if !slices.Equal(kinds, []string{"request.created", "request.resolved"}) {
		t.Errorf("events %v", kinds)
	}

	// Idle or awaiting, silence is fine.
	for _, event := range []string{"idle", "waiting"} {
		f.m.Hook("mara@ledger", event, nil)
		after(3 * time.Hour)
		if list := open(); len(list) != 0 {
			t.Errorf("flagged while %s: %+v", event, list)
		}
	}

	// Its run ending resolves it too.
	f.m.Hook("mara@ledger", "busy", nil)
	after(21 * time.Minute)
	if len(open()) != 1 {
		t.Fatal("not flagged again")
	}
	if err := f.m.Stop(ctx, "atlas@ledger"); err != nil {
		t.Fatal(err)
	}
	f.waitStatus("mara@ledger", Stopped)
	deadline := time.Now().Add(5 * time.Second)
	for len(open()) != 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if list := open(); len(list) != 0 {
		t.Errorf("still open after the stop: %+v", list)
	}
}

// A previous Foreman's stuck cards are resolved on Restore: the watch starts
// over. Other requests stay open.
func TestRestoreResolvesOldStuckCards(t *testing.T) {
	f := newFixture(t, keepsRunning, 12)
	f.deps.Requests = f.db.Requests()
	f.m = New(f.deps)
	stuck, err := f.db.Requests().Save(ctx, store.Request{Kind: "blocked", Source: "foreman", InstanceID: "mara@ledger", Summary: "Mara may be stuck"})
	if err != nil {
		t.Fatal(err)
	}
	other, err := f.db.Requests().Save(ctx, store.Request{Kind: "question", Source: "agent", InstanceID: "mara@ledger", Summary: "Which budget?"})
	if err != nil {
		t.Fatal(err)
	}
	if err := f.m.Restore(ctx); err != nil {
		t.Fatal(err)
	}
	list, err := f.db.Requests().Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].ID != other.ID {
		t.Errorf("open after Restore: %+v (stuck was %s)", list, stuck.ID)
	}
}

// A start that fails stops the instance with the reason, for its pane; a
// start that works clears it.
func TestAFailedStartSaysWhy(t *testing.T) {
	f := newFixture(t, keepsRunning, 12)
	api := filepath.Join(f.root, "dev", "ledger-api")
	gitRun(t, "-C", api, "branch", "-m", "main", "master")
	if _, err := f.m.Deploy(ctx, "atlas", "ledger", false); err != nil {
		t.Fatal(err)
	}
	inst := f.waitStatus("mara@ledger", Stopped)
	if !strings.HasPrefix(inst.StopReason, "couldn't start: ") || !strings.Contains(inst.StopReason, `has no branch "main"`) {
		t.Errorf("stop reason %q", inst.StopReason)
	}
	if _, err := f.hm.SetRepoDefault(ctx, "ledger", "ledger-api", "master"); err != nil {
		t.Fatal(err)
	}
	if err := f.m.Wake(ctx, "mara@ledger"); err != nil {
		t.Fatal(err)
	}
	if s := f.waitStatus("mara@ledger", Running); s.StopReason != "" {
		t.Errorf("after a start: %q", s.StopReason)
	}
}

// crowded is a fixture with one session slot: its lead runs script, then
// `exec cat` (so the shell itself is gone), and is idle; tove, woken, waits
// for the slot. Hooks are read as activities. In script, $HELPER is a
// foreman-agent: sleep under that name.
func crowded(t *testing.T, script string) *fixture {
	t.Helper()
	helper := filepath.Join(t.TempDir(), "foreman-agent")
	sleep, err := exec.LookPath("sleep")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(sleep, helper); err != nil {
		t.Fatal(err)
	}
	f := newFixture(t, strings.ReplaceAll(script, "$HELPER", helper)+"\necho ready\nexec cat", 1)
	f.deps.Adapters = map[string]harness.Adapter{"claude-code": eventIsActivity{f.deps.Adapters["claude-code"].(fake.Adapter)}}
	f.deps.Agent.Path = "/opt/foreman/foreman-agent"
	f.m = New(f.deps)
	if _, err := f.m.Deploy(ctx, "atlas", "ledger", false); err != nil {
		t.Fatal(err)
	}
	f.waitStatus("mara@ledger", Running)
	f.output("mara@ledger", "ready")
	f.m.Hook("mara@ledger", "idle", nil)
	if err := f.m.Wake(ctx, "tove@ledger"); err != nil {
		t.Fatal(err)
	}
	f.waitStatus("tove@ledger", Queued)
	return f
}

// idleSince is when the instance went idle, as MakeRoom counts it.
func (f *fixture) idleSince(id string) time.Time {
	f.m.mu.Lock()
	defer f.m.mu.Unlock()
	if r := f.m.runs[id]; r != nil {
		return r.idleSince
	}
	return time.Time{}
}

// An agent idle for a minute gives its slot to one that waits: it sleeps,
// the other starts, and a wake starts it again (queued in turn). Just idle,
// it keeps the slot; with nobody waiting, it is never touched.
func TestAnIdleAgentMakesRoom(t *testing.T) {
	f := crowded(t, `true`)
	idle := f.idleSince("mara@ledger")
	if idle.IsZero() {
		t.Fatal("no idle clock")
	}
	f.m.MakeRoom(ctx, idle.Add(59*time.Second))
	if s := f.status("mara@ledger"); s.Status != Idle || !f.m.Live("mara@ledger") {
		t.Fatalf("slept before a minute: %s", s.Status)
	}
	f.m.MakeRoom(ctx, idle.Add(time.Minute))
	inst := f.waitStatus("mara@ledger", Asleep)
	if inst.Restarts != 0 || inst.StopReason != "" {
		t.Errorf("asleep: %+v", inst)
	}
	f.waitStatus("tove@ledger", Running)
	f.m.Hook("mara@ledger", "busy", nil) // a late hook from the ended run
	if s := f.status("mara@ledger"); s.Status != Asleep {
		t.Errorf("a late hook made it %s", s.Status)
	}

	// Tove idle, nobody waiting: nothing happens.
	f.m.Hook("tove@ledger", "idle", nil)
	f.m.MakeRoom(ctx, time.Now().Add(3*time.Hour))
	if s := f.status("tove@ledger"); s.Status != Idle {
		t.Fatalf("slept with nobody waiting: %s", s.Status)
	}
	// Mara woken waits, and tove makes room in turn, with its own fresh clock.
	if err := f.m.Wake(ctx, "mara@ledger"); err != nil {
		t.Fatal(err)
	}
	f.waitStatus("mara@ledger", Queued)
	f.m.MakeRoom(ctx, f.idleSince("tove@ledger").Add(time.Minute))
	f.waitStatus("tove@ledger", Asleep)
	f.waitStatus("mara@ledger", Running)
	if !f.idleSince("mara@ledger").IsZero() {
		t.Error("the new run kept the old idle clock")
	}
}

// Only foreman-agent may be left running: anything else keeps the agent
// awake, and a finished process never reaped (a zombie) doesn't.
func TestWhatRunsDecidesWhoMakesRoom(t *testing.T) {
	for _, tc := range []struct {
		name, script string
		sleeps       bool
	}{
		{"a helper", `"$HELPER" 300 &`, true},
		{"a zombie", `true &`, true},
		{"a dev server", `"$HELPER" 300 & sleep 300 &`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := crowded(t, tc.script)
			time.Sleep(200 * time.Millisecond) // the background `true` has ended
			f.m.MakeRoom(ctx, time.Now().Add(time.Hour))
			if tc.sleeps {
				f.waitStatus("mara@ledger", Asleep)
				f.waitStatus("tove@ledger", Running)
				return
			}
			if s := f.status("mara@ledger"); s.Status != Idle || !f.m.Live("mara@ledger") {
				t.Errorf("slept with a process running: %s", s.Status)
			}
			if s := f.status("tove@ledger"); s.Status != Queued {
				t.Errorf("tove is %s", s.Status)
			}
		})
	}
}

// Only a quiet idle agent makes room: running, awaiting, input since it went
// idle, or the CEO's unsent draft keep it awake.
func TestOnlyAQuietIdleAgentMakesRoom(t *testing.T) {
	f := crowded(t, `true`)
	drafting := false
	f.m.Drafting = func(id string) bool { return drafting && id == "mara@ledger" }
	awake := func(why string) {
		t.Helper()
		if s := f.status("mara@ledger"); s.Status == Asleep || !f.m.Live("mara@ledger") {
			t.Fatalf("slept %s: %s", why, s.Status)
		}
	}
	for _, event := range []string{"busy", "waiting"} {
		f.m.Hook("mara@ledger", event, nil)
		f.m.MakeRoom(ctx, time.Now().Add(3*time.Hour))
		awake("while " + event)
	}

	f.m.Hook("mara@ledger", "idle", nil)
	idle := f.idleSince("mara@ledger")
	time.Sleep(50 * time.Millisecond)
	if err := f.m.Type("mara@ledger", []byte("x")); err != nil {
		t.Fatal(err)
	}
	f.m.MakeRoom(ctx, idle.Add(time.Minute)) // a minute since idle, not since the input
	awake("with input since it went idle")

	drafting = true
	f.m.MakeRoom(ctx, time.Now().Add(3*time.Hour))
	awake("on the CEO's draft")
	drafting = false
	f.m.MakeRoom(ctx, time.Now().Add(3*time.Hour))
	f.waitStatus("mara@ledger", Asleep)
	f.waitStatus("tove@ledger", Running)
}

// A sleep whose stop fails leaves the agent as it was, taking input again.
func TestASleepThatCantStopLeavesTheAgentAwake(t *testing.T) {
	f := crowded(t, `true`)
	f.stopAll()
	u := &unstoppable{Runner: f.deps.Runner}
	t.Cleanup(func() {
		for _, p := range u.procs {
			_ = p.Stop(time.Second)
		}
	})
	f.deps.Runner = u
	f.m = New(f.deps)
	if _, err := f.m.Deploy(ctx, "atlas", "ledger", false); err != nil {
		t.Fatal(err)
	}
	f.waitStatus("mara@ledger", Running)
	f.output("mara@ledger", "ready")
	f.m.Hook("mara@ledger", "idle", nil)
	if err := f.m.Wake(ctx, "tove@ledger"); err != nil {
		t.Fatal(err)
	}
	f.waitStatus("tove@ledger", Queued)
	f.m.MakeRoom(ctx, time.Now().Add(time.Hour))
	if s := f.status("mara@ledger"); s.Status != Idle || !f.m.Live("mara@ledger") {
		t.Fatalf("after a failed stop: %s", s.Status)
	}
	if err := f.m.Type("mara@ledger", []byte("still here\r")); err != nil {
		t.Errorf("input after a failed stop: %v", err)
	}
	f.output("mara@ledger", "still here")
}

// A team, project or settings change leaves the live agents' runs behind
// until they start again; asleep ones get it at their wake anyway.
func TestAChangeLeavesLiveAgentsBehindUntilTheyStartAgain(t *testing.T) {
	f := newFixture(t, keepsRunning, 12)
	if _, err := f.m.Deploy(ctx, "atlas", "ledger", false); err != nil {
		t.Fatal(err)
	}
	f.waitStatus("mara@ledger", Running)
	behind := func() map[string]bool {
		t.Helper()
		list, err := f.m.List(ctx)
		if err != nil {
			t.Fatal(err)
		}
		got := map[string]bool{}
		for _, i := range list {
			got[i.ID] = i.Behind
		}
		return got
	}
	if got := behind(); got["mara@ledger"] || got["tove@ledger"] {
		t.Fatalf("behind right after the deploy: %v", got)
	}

	// A soul edit.
	team, err := f.hm.Team("atlas")
	if err != nil {
		t.Fatal(err)
	}
	soul := team.Config.Agents["mara"]
	soul.Personality = "Blunt and fast."
	team.Config.Agents["mara"] = soul
	if _, err := f.hm.SaveTeam(ctx, team.Config, team.Version); err != nil {
		t.Fatal(err)
	}
	if got := behind(); !got["mara@ledger"] || got["tove@ledger"] {
		t.Errorf("after a soul edit: %v (asleep tove isn't behind: it starts on the new one)", got)
	}

	// Restarting it catches up, on the new prompt.
	if err := f.m.Restart(ctx, "mara@ledger"); err != nil {
		t.Fatal(err)
	}
	prompt := f.hm.Path("harness", "ledger", "mara", "fake", "PROMPT.md")
	for deadline := time.Now().Add(5 * time.Second); ; time.Sleep(20 * time.Millisecond) {
		if given, _ := os.ReadFile(prompt); strings.Contains(string(given), "Blunt and fast.") {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("never restarted on the new prompt")
		}
	}
	f.waitStatus("mara@ledger", Running)
	if got := behind(); got["mara@ledger"] {
		t.Errorf("behind after its restart: %v", got)
	}

	// The CEO's instructions are part of the prompt too.
	if _, err := f.hm.SaveInstructions(ctx, "Answer in Catalan.", f.hm.Instructions().Version); err != nil {
		t.Fatal(err)
	}
	if got := behind(); !got["mara@ledger"] {
		t.Errorf("not behind after the instructions changed: %v", got)
	}

	// On the prompt as composed now: caught up. With no hash (a run from
	// before it was kept): behind, so Reload puts it on the current prompt.
	text, err := f.m.Prompt(ctx, "mara@ledger")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		hash   string
		behind bool
	}{{promptHash(text), false}, {"", true}} {
		inst := f.status("mara@ledger")
		inst.PromptHash = tc.hash
		if err := f.db.Instances().Put(ctx, inst); err != nil {
			t.Fatal(err)
		}
		if got := behind(); got["mara@ledger"] != tc.behind {
			t.Errorf("hash %q: behind %v, want %v", tc.hash, got["mara@ledger"], tc.behind)
		}
	}
}

// asksPayload is eventIsActivity whose waiting events ask their payload.
type asksPayload struct{ eventIsActivity }

func (asksPayload) Asks(_ string, payload []byte) string { return string(payload) }

// A harness waiting on the user raises one card, whatever the hooks repeat,
// and its next activity, or its end, resolves it.
func TestAWaitingHarnessRaisesACard(t *testing.T) {
	f := newFixture(t, keepsRunning, 12)
	f.deps.Adapters = map[string]harness.Adapter{"claude-code": asksPayload{eventIsActivity{f.deps.Adapters["claude-code"].(fake.Adapter)}}}
	f.deps.Requests = f.db.Requests()
	f.m = New(f.deps)
	if _, err := f.m.Deploy(ctx, "atlas", "ledger", false); err != nil {
		t.Fatal(err)
	}
	f.waitStatus("mara@ledger", Running)
	open := func() []store.Request {
		t.Helper()
		list, err := f.db.Requests().Open(ctx)
		if err != nil {
			t.Fatal(err)
		}
		return list
	}

	f.m.Hook("mara@ledger", "waiting", []byte("Overwrite migrations? (Yes / No)"))
	f.m.Hook("mara@ledger", "waiting", []byte("Overwrite migrations? (Yes / No)"))
	f.m.Hook("mara@ledger", "", nil) // says nothing: it still waits
	list := open()
	if len(list) != 1 {
		t.Fatalf("cards %+v", list)
	}
	if r := list[0]; r.Kind != "question" || r.Source != "harness" || r.InstanceID != "mara@ledger" || r.DeploymentID != "atlas@ledger" ||
		r.ProjectID != "ledger" || r.Summary != "Overwrite migrations? (Yes / No)" || len(r.Options) != 0 {
		t.Errorf("card %+v", r)
	}
	if s := f.status("mara@ledger"); s.Status != Awaiting {
		t.Errorf("status %s", s.Status)
	}
	// Answered in its terminal: it works again.
	f.m.Hook("mara@ledger", "busy", nil)
	if list := open(); len(list) != 0 {
		t.Errorf("still open once busy: %+v", list)
	}

	// Nothing said of what it asks: who waits; at its prompt again resolves it too.
	f.m.Hook("mara@ledger", "waiting", nil)
	if list := open(); len(list) != 1 || list[0].Summary != "Mara is waiting for your input." {
		t.Fatalf("cards %+v", list)
	}
	f.m.Hook("mara@ledger", "idle", nil)
	if list := open(); len(list) != 0 {
		t.Errorf("still open once idle: %+v", list)
	}

	// Dismissed by the CEO: the same wait raises no second card; its end
	// leaves the dismissal be, and the next wait raises a fresh one.
	f.m.Hook("mara@ledger", "waiting", nil)
	card := open()[0]
	if _, err := f.db.Requests().Settle(ctx, card.ID, store.RequestDismissed, ""); err != nil {
		t.Fatal(err)
	}
	f.m.Hook("mara@ledger", "waiting", nil)
	if list := open(); len(list) != 0 {
		t.Errorf("raised again while the same wait: %+v", list)
	}
	f.m.Hook("mara@ledger", "busy", nil)
	f.m.Hook("mara@ledger", "waiting", nil)
	if list := open(); len(list) != 1 || list[0].ID == card.ID {
		t.Fatalf("the next wait: %+v", list)
	}
	f.m.Hook("mara@ledger", "idle", nil)

	// Its run ending resolves it.
	f.m.Hook("mara@ledger", "waiting", nil)
	if err := f.m.Stop(ctx, "atlas@ledger"); err != nil {
		t.Fatal(err)
	}
	f.waitStatus("mara@ledger", Stopped)
	deadline := time.Now().Add(5 * time.Second)
	for len(open()) != 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if list := open(); len(list) != 0 {
		t.Errorf("still open after the stop: %+v", list)
	}
}

// After a restart, an agent still waiting in its re-attached session keeps
// its card (its next activity resolves it); one whose session is gone loses it.
func TestRestoreKeepsTheCardOfAnAgentStillWaiting(t *testing.T) {
	f := newFixture(t, keepsRunning, 12)
	f.deps.Adapters = map[string]harness.Adapter{"claude-code": asksPayload{eventIsActivity{f.deps.Adapters["claude-code"].(fake.Adapter)}}}
	f.deps.Requests = f.db.Requests()
	dep, err := f.db.Deployments().Deploy(ctx, "atlas", "ledger")
	if err != nil {
		t.Fatal(err)
	}
	cards := map[string]store.Request{}
	for _, agent := range []string{"mara", "tove"} {
		_ = f.db.Instances().Put(ctx, store.Instance{ID: agent + "@ledger", DeploymentID: dep.ID, AgentID: agent, Status: Awaiting,
			Harness: "claude-code", SandboxPath: f.hm.Sandbox("ledger", agent), StartedAt: time.Now().Add(-time.Hour)})
		card, err := f.db.Requests().Save(ctx, store.Request{Kind: "question", Source: "harness", DeploymentID: dep.ID, InstanceID: agent + "@ledger", Summary: "Which?"})
		if err != nil {
			t.Fatal(err)
		}
		cards[agent] = card
	}
	survivor, err := (direct.Runner{}).Start(ctx, runner.Spec{Command: []string{"/bin/sh", "-c", "cat"},
		Labels: map[string]string{"foreman.instance": "mara@ledger", HomeLabel: f.hm.Root()}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = survivor.Stop(time.Second) })
	f.deps.Runner = listing{listed: []runner.Process{survivor}}
	f.m = New(f.deps)
	if err := f.m.Restore(ctx); err != nil {
		t.Fatal(err)
	}
	list, err := f.db.Requests().Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].ID != cards["mara"].ID {
		t.Fatalf("open after Restore: %+v", list)
	}
	f.m.Hook("mara@ledger", "busy", nil)
	if list, _ := f.db.Requests().Open(ctx); len(list) != 0 {
		t.Errorf("the kept card didn't resolve on activity: %+v", list)
	}
}

// A harness's long question is a card's short title, the whole of it its
// body; a short one is the title alone.
func TestHeadline(t *testing.T) {
	long := strings.Repeat("é", teamtools.MaxTitle+5)
	for asks, want := range map[string][2]string{
		"":                         {"", ""},
		"  Overwrite? (Yes / No) ": {"Overwrite? (Yes / No)", ""},
		"Which DB?\nPostgres has…": {"Which DB?", "Which DB?\nPostgres has…"},
		long:                       {strings.Repeat("é", teamtools.MaxTitle-1) + "…", long},
	} {
		if title, body := headline(asks); title != want[0] || body != want[1] {
			t.Errorf("%q: %q, %q", asks, title, body)
		}
	}
	if title, _ := headline(long); utf8.RuneCountInString(title) != teamtools.MaxTitle {
		t.Errorf("cut to %d characters", utf8.RuneCountInString(title))
	}
}
