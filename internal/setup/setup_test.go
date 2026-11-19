package setup

import (
	"context"
	"errors"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/brickpop/foreman/internal/events"
	"github.com/brickpop/foreman/internal/home"
	"github.com/brickpop/foreman/internal/profiles"
	"github.com/brickpop/foreman/internal/runner"
	"github.com/brickpop/foreman/internal/runner/podman"
)

var ctx = context.Background()

// engine is Podman as the CEO's machine has it: installed or not, running or
// not; a stopped machine starts only when Connect is asked to.
type engine struct {
	mu                 sync.Mutex
	installed, running bool
	machineStopped     bool
	starts             int      // Connects asked to start
	initErr            error    // what CheckInit returns
	initChecks         []string // the images CheckInit ran on
	// gates[i], when there is one, holds the i-th CheckInit until the test
	// sends the error it returns.
	gates []chan error
}

func (e *engine) set(f func(e *engine)) { e.mu.Lock(); f(e); e.mu.Unlock() }

func (e *engine) State(context.Context) podman.State {
	e.mu.Lock()
	defer e.mu.Unlock()
	return podman.State{Installed: e.installed, Running: e.running, Version: map[bool]string{true: "5.7"}[e.running]}
}

func (e *engine) Connect(_ context.Context, start bool) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if start {
		e.starts++
		if e.installed && e.machineStopped {
			e.machineStopped, e.running = false, true
		}
	}
	if !e.running {
		return podman.ErrNotRunning
	}
	return nil
}

func (e *engine) CreateMachine(context.Context, int, int) error { return nil }

func (e *engine) CheckInit(_ context.Context, image string) error {
	e.mu.Lock()
	var gate chan error
	if n := len(e.initChecks); n < len(e.gates) {
		gate = e.gates[n]
	}
	e.initChecks = append(e.initChecks, image)
	err := e.initErr
	e.mu.Unlock()
	if gate != nil {
		err = <-gate
	}
	return err
}

func (e *engine) checks() int { e.mu.Lock(); defer e.mu.Unlock(); return len(e.initChecks) }

type images struct {
	mu     sync.Mutex
	state  string
	builds chan struct{}
}

func (im *images) Ensure(context.Context, string) (string, error) {
	im.mu.Lock()
	im.state = "ready"
	im.mu.Unlock()
	im.builds <- struct{}{}
	return "img", nil
}
func (im *images) Refresh(context.Context) error { return nil }
func (im *images) Status(context.Context) []runner.ImageStatus {
	im.mu.Lock()
	defer im.mu.Unlock()
	ref := map[string]string{"ready": "img"}[im.state]
	return []runner.ImageStatus{{Name: "harness", Ref: ref, State: im.state}}
}

type config struct {
	teams    []home.Team
	projects []home.Project
	pristine bool
}

func (c config) Teams() []home.Team            { return c.teams }
func (c config) Projects() []home.Project      { return c.projects }
func (c config) Pristine(context.Context) bool { return c.pristine }

type rig struct {
	*Setup
	engine          *engine
	images          *images
	ready, connects int
	deliveryBlocked error // what Blocked said when Connected last ran
	readyErr        error
	published       atomic.Int32 // counted from the image build's goroutine too
}

func newRig(t *testing.T, e *engine) *rig {
	t.Helper()
	r := &rig{engine: e, images: &images{state: "missing", builds: make(chan struct{}, 10)}}
	var eng Engine
	if e != nil {
		eng = e
	}
	r.Setup = New(Deps{
		Engine: eng, Images: r.images, Config: config{},
		Profiles: func() []profiles.Profile {
			return []profiles.Profile{{Name: "default", Harness: "claude-code", Connected: true}}
		},
		Ready: func(context.Context) error {
			r.ready++
			return r.readyErr
		},
		Connected: func(context.Context) {
			r.connects++
			r.deliveryBlocked = r.Blocked()
		},
		Publish: func(e events.Event) {
			if e.Type == "setup.changed" {
				r.published.Add(1)
			}
		},
	})
	return r
}

func (r *rig) built(t *testing.T) {
	t.Helper()
	<-r.images.builds
}

// Foreman starts with no Podman; once it is installed and running, the next
// check restores (once), builds the image, and lets agents start.
func TestPodmanAppearsAfterStartup(t *testing.T) {
	r := newRig(t, &engine{})
	r.check(ctx, true)
	if r.ready != 0 || !errors.Is(r.Blocked(), ErrNotReady) {
		t.Fatalf("without Podman: ready %d, blocked %v", r.ready, r.Blocked())
	}
	// Podman answers, the restore hasn't run yet: still working.
	r.engine.set(func(e *engine) { e.installed, e.running = true, true })
	if got := stateOf(r.Steps(ctx), "running"); got != "working" {
		t.Errorf("before the restore: %s", got)
	}
	r.engine.set(func(e *engine) { e.installed, e.running = false, false })
	if got := kinds(r.Steps(ctx)); got[1] != "podman todo" {
		t.Errorf("steps %v", got)
	}

	r.engine.set(func(e *engine) { e.installed, e.running = true, true })
	before := r.published.Load()
	r.check(ctx, false)
	r.built(t)
	if r.ready != 1 || r.Blocked() != nil || r.published.Load() == before {
		t.Fatalf("with Podman: ready %d, blocked %v, published %d", r.ready, r.Blocked(), r.published.Load()-before)
	}
	if got := stateOf(r.Steps(ctx), "running"); got != "done" {
		t.Errorf("restored: %s", got)
	}
	// What waited is delivered once agents may start: a delivery wakes asleep
	// agents, which a not-yet-ready Foreman refuses.
	if r.connects != 1 || r.deliveryBlocked != nil {
		t.Errorf("first delivery: connected %d, blocked %v", r.connects, r.deliveryBlocked)
	}
	r.check(ctx, false)
	if r.ready != 1 || r.connects != 1 {
		t.Errorf("a steady engine: ready %d, connected %d", r.ready, r.connects)
	}
	select {
	case <-r.images.builds:
		t.Error("the image was built again while Podman stayed up")
	default:
	}
}

// Podman going away blocks agents again; its return delivers what waited
// (Connected), never restores twice, and checks the image again.
func TestPodmanStopsAndComesBack(t *testing.T) {
	r := newRig(t, &engine{installed: true, running: true})
	r.check(ctx, true)
	r.built(t)
	r.engine.set(func(e *engine) { e.running = false })
	r.check(ctx, false)
	if r.Blocked() == nil {
		t.Fatal("Podman stopped, yet agents may start")
	}
	r.engine.set(func(e *engine) { e.running = true })
	r.check(ctx, false)
	r.built(t)
	if r.ready != 1 || r.connects != 2 || r.Blocked() != nil || r.deliveryBlocked != nil {
		t.Errorf("back: ready %d, connected %d, blocked %v, delivery blocked %v", r.ready, r.connects, r.Blocked(), r.deliveryBlocked)
	}
}

// A stopped machine is started at Foreman's launch and by the CEO's Start,
// never by the periodic check: a machine the CEO stopped stays stopped.
func TestOnlyLaunchAndStartStartTheMachine(t *testing.T) {
	r := newRig(t, &engine{installed: true, machineStopped: true})
	r.engine.set(func(e *engine) { e.machineStopped = true })
	r.check(ctx, false)
	if r.engine.starts != 0 || r.Blocked() == nil {
		t.Fatalf("a periodic check started the machine")
	}
	r.check(ctx, true)
	r.built(t)
	if r.engine.starts != 1 || r.Blocked() != nil {
		t.Errorf("the launch didn't start it: starts %d, blocked %v", r.engine.starts, r.Blocked())
	}
}

// A failing restore keeps agents blocked and is retried at the next check.
func TestAFailedRestoreIsRetried(t *testing.T) {
	r := newRig(t, &engine{installed: true, running: true})
	r.readyErr = errors.New("database locked")
	r.check(ctx, true)
	r.built(t)
	if r.Blocked() == nil {
		t.Fatal("not restored, yet agents may start")
	}
	r.readyErr = nil
	r.check(ctx, false)
	if r.ready != 2 || r.Blocked() != nil || r.connects != 1 {
		t.Errorf("retry: ready %d, blocked %v, connected %d", r.ready, r.Blocked(), r.connects)
	}
}

// The Direct engine needs no Podman: ready at the first check, no Podman or
// image steps.
func TestDirectEngine(t *testing.T) {
	r := newRig(t, nil)
	r.Images = runnerless{}
	if r.Blocked() == nil {
		t.Fatal("ready before the first check")
	}
	r.check(ctx, true)
	r.check(ctx, false)
	if r.ready != 1 || r.Blocked() != nil {
		t.Fatalf("ready %d, blocked %v", r.ready, r.Blocked())
	}
	if got := kinds(r.Steps(ctx)); !reflect.DeepEqual(got, []string{"hq done", "profile:default done", "team todo", "project todo"}) {
		t.Errorf("steps %v", got)
	}
}

type runnerless struct{}

func (runnerless) Ensure(context.Context, string) (string, error) { return "", nil }
func (runnerless) Refresh(context.Context) error                  { return nil }
func (runnerless) Status(context.Context) []runner.ImageStatus    { return nil }

func TestSteps(t *testing.T) {
	var team home.Team
	for _, c := range []struct {
		name     string
		config   config
		profiles []profiles.Profile
		want     []string
	}{
		{"nothing yet: the default profile, a team, a project", config{},
			[]profiles.Profile{{Name: "default", Harness: "claude-code", Connected: true}},
			[]string{"hq done", "image todo", "profile:default done", "team todo", "project todo"}},
		// The HQ first start made: done all the same, Restore only offered.
		{"a pristine HQ", config{pristine: true},
			[]profiles.Profile{{Name: "default", Harness: "claude-code", Connected: true}},
			[]string{"hq done", "image todo", "profile:default done", "team todo", "project todo"}},
		// The profiles the projects use: one not connected, one that doesn't exist.
		{"a project without a repo is no step: a research team needs none",
			config{teams: []home.Team{team}, projects: []home.Project{
				{ID: "web", Repos: []home.Repo{{ID: "app", Path: "/home/ceo/app"}}}, {ID: "ledger"}}},
			[]profiles.Profile{{Name: "default", Harness: "claude-code"}, {Name: "gone"}},
			[]string{"hq done", "image todo", "profile:default todo", "profile:gone failed", "team done", "project done"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			s := New(Deps{Images: &images{state: "missing"}, Config: c.config,
				Profiles: func() []profiles.Profile { return c.profiles }})
			steps := s.Steps(ctx)
			if got := kinds(steps); !reflect.DeepEqual(got, c.want) {
				t.Errorf("steps %v\nwant  %v", got, c.want)
			}
			if steps[0].Restorable != c.config.pristine {
				t.Errorf("restorable %v", steps[0].Restorable)
			}
			for _, st := range steps {
				if st.Subject == "gone" && !strings.Contains(st.Detail, "No such profile") {
					t.Errorf("gone: %q", st.Detail)
				}
			}
		})
	}
}

func TestPodmanSteps(t *testing.T) {
	bounds := &podman.MachineBounds{MinMemory: 4, MaxMemory: 19, Memory: 16, MaxCPUs: 10, CPUs: 5}
	for _, c := range []struct {
		name  string
		goos  string
		state podman.State
		want  []string
	}{
		{"linux, nothing", "linux", podman.State{}, []string{"podman todo", "running todo"}},
		{"linux, the service failed", "linux", podman.State{Installed: true, Error: "boom"}, []string{"podman done", "running failed"}},
		{"linux, running", "linux", podman.State{Installed: true, Running: true}, []string{"podman done", "running done"}},
		{"mac, nothing", "darwin", podman.State{}, []string{"podman todo", "machine todo", "running todo"}},
		{"mac, no machine", "darwin", podman.State{Installed: true, Machine: "none", Bounds: bounds}, []string{"podman done", "machine todo", "running todo"}},
		{"mac, creating", "darwin", podman.State{Installed: true, Machine: "creating", Log: "pulling"}, []string{"podman done", "machine working", "running todo"}},
		{"mac, stopped", "darwin", podman.State{Installed: true, Machine: "stopped"}, []string{"podman done", "machine done", "running todo"}},
		{"mac, running", "darwin", podman.State{Installed: true, Machine: "running", Running: true}, []string{"podman done", "machine done", "running done"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			steps := podmanSteps(c.state, c.goos)
			if got := kinds(steps); !reflect.DeepEqual(got, c.want) {
				t.Errorf("steps %v\nwant  %v", got, c.want)
			}
			for _, s := range steps {
				if s.Kind == "machine" && c.state.Machine == "none" && (s.Machine == nil || s.Machine.Memory != 16 || s.Machine.MaxCPUs != 10) {
					t.Errorf("the machine row lacks its sizes: %+v", s.Machine)
				}
				if s.Kind == "machine" && c.state.Machine == "creating" && s.Log != "pulling" {
					t.Errorf("the creation's output isn't shown: %q", s.Log)
				}
			}
		})
	}
}

// kinds names each step "kind[:subject] state".
func kinds(steps []Step) []string {
	var out []string
	for _, s := range steps {
		k := s.Kind
		if s.Subject != "" && s.Kind != "image" {
			k += ":" + s.Subject
		}
		out = append(out, k+" "+s.State)
	}
	return out
}

func stateOf(steps []Step, kind string) string {
	for _, s := range steps {
		if s.Kind == kind {
			return s.State
		}
	}
	return "missing"
}

// initStep waits until the init check has settled (it runs in the background
// after the image build) and returns the init row.
func (r *rig) initStep(t *testing.T) Step {
	t.Helper()
	for deadline := time.Now().Add(2 * time.Second); time.Now().Before(deadline); time.Sleep(time.Millisecond) {
		for _, s := range r.Steps(ctx) {
			if s.Kind == "init" && s.State != "todo" && s.State != "working" {
				return s
			}
		}
	}
	t.Fatal("the init check never settled")
	return Step{}
}

// started waits until the init check has started n times.
func (r *rig) started(t *testing.T, n int) {
	t.Helper()
	for deadline := time.Now().Add(2 * time.Second); r.engine.checks() < n; time.Sleep(time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("the init check started %d times, want %d", r.engine.checks(), n)
		}
	}
}

// release lets a held init check return err and waits until checkInit has
// settled its result: its last publish. Call it while nothing else publishes
// (every check the test started has reached CheckInit).
func (r *rig) release(t *testing.T, gate chan<- error, err error) {
	t.Helper()
	before := r.published.Load()
	gate <- err
	for deadline := time.Now().Add(2 * time.Second); r.published.Load() == before; time.Sleep(time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("the init check never settled")
		}
	}
}

// The init check runs once Podman answers and the agent image is built, on
// that image, and its result is cached: neither Steps nor the periodic check
// runs it again.
func TestInitCheckPasses(t *testing.T) {
	r := newRig(t, &engine{installed: true, running: true})
	if s := stepOf(r.Steps(ctx), "init"); s.State != "todo" {
		t.Errorf("before the check: %+v", s)
	}
	r.check(ctx, true)
	r.built(t)
	if s := r.initStep(t); s.State != "done" || s.Detail != "" {
		t.Errorf("passed: %+v", s)
	}
	for range 3 {
		r.Steps(ctx)
		r.check(ctx, false)
	}
	r.engine.mu.Lock()
	defer r.engine.mu.Unlock()
	if !reflect.DeepEqual(r.engine.initChecks, []string{"img"}) {
		t.Errorf("checks %q, want one on the agent image", r.engine.initChecks)
	}
}

// The checklist reads top-down: the row waits on the agent image, so it comes
// right after the last image row.
func TestInitStepFollowsTheImages(t *testing.T) {
	r := newRig(t, &engine{installed: true, running: true})
	r.Images = twoImages{}
	got := kinds(r.Steps(ctx))
	want := []string{"hq done", "podman done"}
	if runtime.GOOS == "darwin" {
		want = append(want, "machine todo")
	}
	want = append(want, "running working", "image todo", "image todo", "init todo")
	if len(got) < len(want) || !slices.Equal(got[:len(want)], want) {
		t.Errorf("steps %v\nwant  %v, then the rest", got, want)
	}
}

// twoImages is two agent images, neither built yet.
type twoImages struct{}

func (twoImages) Ensure(context.Context, string) (string, error) { return "", errors.New("not built") }
func (twoImages) Refresh(context.Context) error                  { return nil }
func (twoImages) Status(context.Context) []runner.ImageStatus {
	return []runner.ImageStatus{{Name: "harness", State: "missing"}, {Name: "other", State: "missing"}}
}

// While the throwaway container runs, the row is working.
func TestInitCheckWorking(t *testing.T) {
	gate := make(chan error)
	r := newRig(t, &engine{installed: true, running: true, gates: []chan error{gate}})
	r.check(ctx, true)
	r.built(t)
	r.started(t, 1)
	if s := stepOf(r.Steps(ctx), "init"); s.State != "working" {
		t.Errorf("while it runs: %+v", s)
	}
	r.release(t, gate, nil)
	if s := stepOf(r.Steps(ctx), "init"); s.State != "done" {
		t.Errorf("after: %+v", s)
	}
}

// A failed check keeps its result, with Podman's own words in the log, until
// Podman comes back: the periodic check never runs it again. Podman's return
// checks again, which is how a fix (catatonit installed) is picked up.
func TestInitCheckFailsUntilPodmanComesBack(t *testing.T) {
	podmanSaid := "crun: container-init binary not found on the host: stat /usr/libexec/podman/catatonit: no such file or directory"
	r := newRig(t, &engine{installed: true, running: true, initErr: errors.New(podmanSaid)})
	r.check(ctx, true)
	r.built(t)
	if s := r.initStep(t); s.State != "failed" || s.Log != podmanSaid || strings.Contains(s.Detail, podmanSaid) {
		t.Fatalf("failed: %+v", s)
	}
	r.engine.set(func(e *engine) { e.initErr = nil })
	r.check(ctx, false)
	if stepOf(r.Steps(ctx), "init").State != "failed" || r.engine.checks() != 1 {
		t.Fatal("the periodic check ran the init check again")
	}
	r.engine.set(func(e *engine) { e.running = false })
	r.check(ctx, false)
	r.engine.set(func(e *engine) { e.running = true })
	r.check(ctx, false)
	r.built(t)
	r.started(t, 2)
	if s := r.initStep(t); s.State != "done" {
		t.Errorf("back: %+v", s)
	}
}

// While Podman is away the row is todo, never a ✓ under a Podman running row
// that isn't done; a check still running then drops its result.
func TestInitCheckResetsWhenPodmanStops(t *testing.T) {
	gate := make(chan error)
	r := newRig(t, &engine{installed: true, running: true, gates: []chan error{nil, gate}})
	r.check(ctx, true)
	r.built(t)
	if s := r.initStep(t); s.State != "done" {
		t.Fatalf("passed: %+v", s)
	}
	r.engine.set(func(e *engine) { e.running = false })
	r.check(ctx, false)
	if s := stepOf(r.Steps(ctx), "init"); s.State != "todo" {
		t.Errorf("Podman stopped: %+v", s)
	}

	r.engine.set(func(e *engine) { e.running = true })
	r.check(ctx, false)
	r.built(t)
	r.started(t, 2)
	r.engine.set(func(e *engine) { e.running = false })
	r.check(ctx, false)
	r.release(t, gate, nil)
	if s := stepOf(r.Steps(ctx), "init"); s.State != "todo" {
		t.Errorf("a check that ended after Podman stopped: %+v", s)
	}
}

// A check that outlived a Podman cycle never overwrites the newer one's
// result, whichever ends first.
func TestAnOutlivedCheckIsDropped(t *testing.T) {
	a, b := make(chan error), make(chan error)
	r := newRig(t, &engine{installed: true, running: true, gates: []chan error{a, b}})
	r.check(ctx, true)
	r.built(t)
	r.started(t, 1)
	r.engine.set(func(e *engine) { e.running = false })
	r.check(ctx, false)
	r.engine.set(func(e *engine) { e.running = true })
	r.check(ctx, false)
	r.built(t)
	r.started(t, 2)
	r.release(t, a, errors.New("podman went away"))
	if s := stepOf(r.Steps(ctx), "init"); s.State != "working" {
		t.Errorf("the outlived check ended: %+v", s)
	}
	r.release(t, b, nil)
	if s := stepOf(r.Steps(ctx), "init"); s.State != "done" {
		t.Errorf("the newer check passed: %+v", s)
	}
}

// What the failed row says: the catatonit fix only when Podman says its init
// is missing, and only on Linux (on macOS the init lives in the Podman
// machine); any other failure says agents can't start, never to install what
// may well be there. Podman's error is in the log, never the detail.
func TestInitStep(t *testing.T) {
	missing := "container-init binary not found on the host"
	fix := "Agents can't start: Podman can't find catatonit, the small program every agent runs under. " +
		"Install it (sudo dnf install catatonit, or your distribution's package), then restart Foreman."
	general := "Podman can't start a test container, so agents can't start either. Its error is below."
	for _, c := range []struct {
		state, podmanErr, goos string
		want                   Step
	}{
		{"", "", "linux", Step{Kind: "init", State: "todo", Detail: "Waits for the steps above"}},
		{"working", "", "linux", Step{Kind: "init", State: "working", Detail: "Starting a test container"}},
		{"done", "", "darwin", Step{Kind: "init", State: "done"}},
		{"failed", missing, "linux", Step{Kind: "init", State: "failed", Detail: fix, Log: missing}},
		{"failed", missing, "darwin", Step{Kind: "init", State: "failed", Detail: general, Log: missing}},
		{"failed", "context deadline exceeded", "linux", Step{Kind: "init", State: "failed", Detail: general, Log: "context deadline exceeded"}},
	} {
		if got := initStep(c.state, c.podmanErr, c.goos); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s %q on %s: %+v\nwant %+v", c.state, c.podmanErr, c.goos, got, c.want)
		}
	}
}

func TestMissingInit(t *testing.T) {
	for podmanSaid, want := range map[string]bool{
		"container-init binary not found on the host: stat /usr/libexec/podman/catatonit: no such file or directory": true,
		"container init binary not found":     true,
		"context deadline exceeded":           false,
		"the check's container exited with 1": false,
		"podman isn't running":                false,
		"":                                    false,
	} {
		if got := missingInit(podmanSaid); got != want {
			t.Errorf("missingInit(%q) = %v, want %v", podmanSaid, got, want)
		}
	}
}

// No agent image yet (its build failed): nothing to check on.
func TestNoInitCheckWithoutTheImage(t *testing.T) {
	r := newRig(t, &engine{installed: true, running: true})
	failed := failedImages{ensured: make(chan struct{}, 1)}
	r.Images = failed
	r.check(ctx, true)
	<-failed.ensured
	r.check(ctx, false)
	if s := stepOf(r.Steps(ctx), "init"); s.State != "todo" || r.engine.checks() != 0 {
		t.Errorf("without an image: %+v, %d checks", s, r.engine.checks())
	}
}

// failedImages is an agent image whose build fails.
type failedImages struct{ ensured chan struct{} }

func (im failedImages) Ensure(context.Context, string) (string, error) {
	im.ensured <- struct{}{}
	return "", errors.New("build failed")
}
func (failedImages) Refresh(context.Context) error { return nil }
func (failedImages) Status(context.Context) []runner.ImageStatus {
	return []runner.ImageStatus{{Name: "harness", State: "failed"}}
}

func stepOf(steps []Step, kind string) Step {
	for _, s := range steps {
		if s.Kind == kind {
			return s
		}
	}
	return Step{Kind: kind, State: "missing"}
}
