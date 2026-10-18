package setup

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/brickpop/foreman/internal/events"
	"github.com/brickpop/foreman/internal/profiles"
	"github.com/brickpop/foreman/internal/runner"
	"github.com/brickpop/foreman/internal/runner/podman"
	"github.com/brickpop/foreman/internal/workspace"
)

var ctx = context.Background()

// engine is Podman as the CEO's machine has it: installed or not, running or
// not; a stopped machine starts only when Connect is asked to.
type engine struct {
	mu                 sync.Mutex
	installed, running bool
	machineStopped     bool
	starts             int // Connects asked to start
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
	return []runner.ImageStatus{{Name: "harness", State: im.state}}
}

type config struct {
	teams    []workspace.Team
	projects []workspace.Project
}

func (c config) Teams() []workspace.Team       { return c.teams }
func (c config) Projects() []workspace.Project { return c.projects }

type rig struct {
	*Setup
	engine           *engine
	images           *images
	ready, reconnect int
	readyErr         error
	published        int
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
		Reconnected: func(context.Context) { r.reconnect++ },
		Publish: func(e events.Event) {
			if e.Type == "setup.changed" {
				r.published++
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
	if got := kinds(r.Steps(ctx)); got[0] != "podman todo" {
		t.Errorf("steps %v", got)
	}

	r.engine.set(func(e *engine) { e.installed, e.running = true, true })
	before := r.published
	r.check(ctx, false)
	r.built(t)
	if r.ready != 1 || r.Blocked() != nil || r.published == before {
		t.Fatalf("with Podman: ready %d, blocked %v, published %d", r.ready, r.Blocked(), r.published-before)
	}
	if got := stateOf(r.Steps(ctx), "running"); got != "done" {
		t.Errorf("restored: %s", got)
	}
	r.check(ctx, false)
	if r.ready != 1 || r.reconnect != 0 {
		t.Errorf("a steady engine: ready %d, reconnected %d", r.ready, r.reconnect)
	}
	select {
	case <-r.images.builds:
		t.Error("the image was built again while Podman stayed up")
	default:
	}
}

// Podman going away blocks agents again; its return delivers what waited
// (Reconnected), never restores twice, and checks the image again.
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
	if r.ready != 1 || r.reconnect != 1 || r.Blocked() != nil {
		t.Errorf("back: ready %d, reconnected %d, blocked %v", r.ready, r.reconnect, r.Blocked())
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
	if r.ready != 2 || r.Blocked() != nil || r.reconnect != 0 {
		t.Errorf("retry: ready %d, blocked %v, reconnected %d", r.ready, r.Blocked(), r.reconnect)
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
	if got := kinds(r.Steps(ctx)); !reflect.DeepEqual(got, []string{"profile:default done", "team todo", "project todo"}) {
		t.Errorf("steps %v", got)
	}
}

type runnerless struct{}

func (runnerless) Ensure(context.Context, string) (string, error) { return "", nil }
func (runnerless) Refresh(context.Context) error                  { return nil }
func (runnerless) Status(context.Context) []runner.ImageStatus    { return nil }

func TestSteps(t *testing.T) {
	var team workspace.Team
	for _, c := range []struct {
		name     string
		config   config
		profiles []profiles.Profile
		want     []string
	}{
		{"nothing yet: the default profile, a team, a project", config{},
			[]profiles.Profile{{Name: "default", Harness: "claude-code", Connected: true}},
			[]string{"image todo", "profile:default done", "team todo", "project todo"}},
		// The profiles the projects use: one not connected, one that doesn't exist.
		{"a project without a repo, one with problems, one with a repo",
			config{teams: []workspace.Team{team}, projects: []workspace.Project{
				{ID: "web", Repos: []string{"app"}}, {ID: "ledger"}, {ID: "broken", Problems: []string{"bad toml"}}}},
			[]profiles.Profile{{Name: "default", Harness: "claude-code"}, {Name: "gone"}},
			[]string{"image todo", "profile:default todo", "profile:gone failed", "team done", "project done", "repo:ledger todo"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			s := New(Deps{Images: &images{state: "missing"}, Config: c.config,
				Profiles: func() []profiles.Profile { return c.profiles }})
			steps := s.Steps(ctx)
			if got := kinds(steps); !reflect.DeepEqual(got, c.want) {
				t.Errorf("steps %v\nwant  %v", got, c.want)
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
