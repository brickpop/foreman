// Package setup is the setup checklist and keeps the engine connected: Foreman
// starts whatever is missing, reaches Podman once it is there, and says what
// is left to do (wiki/product/features/first-run.md).
package setup

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/brickpop/foreman/internal/events"
	"github.com/brickpop/foreman/internal/home"
	"github.com/brickpop/foreman/internal/profiles"
	"github.com/brickpop/foreman/internal/runner"
	"github.com/brickpop/foreman/internal/runner/podman"
)

// Engine is the Podman engine as setup needs it.
type Engine interface {
	State(ctx context.Context) podman.State
	// Connect reaches Podman; start also starts a stopped machine (macOS).
	Connect(ctx context.Context, start bool) error
	CreateMachine(ctx context.Context, cpus, memory int) error
	// CheckInit runs a throwaway container on image with an init, as agents
	// run, and returns Podman's error when it can't.
	CheckInit(ctx context.Context, image string) error
}

// Config is the home's teams and projects.
type Config interface {
	Teams() []home.Team
	Projects() []home.Project
	// Pristine says Foreman's HQ is as its first start made it, with no
	// origin: it may still be restored from another machine's.
	Pristine(ctx context.Context) bool
}

// Deps is what the checklist reads and the watcher drives.
type Deps struct {
	Engine Engine // nil for the Direct engine: no Podman or image steps
	Images runner.Images
	// Profiles are the profiles the projects use (the default while there
	// is no project), with their state; one that doesn't exist has no harness.
	Profiles func() []profiles.Profile
	Config   Config
	// Ready runs once, the first time the engine answers: restoring the
	// instances. Retried while it fails.
	Ready func(ctx context.Context) error
	// Connected runs each time agents may start again: right after Ready
	// succeeds, then whenever the engine answers after being away. It
	// delivers what waited, so it runs after Ready, never inside it: a
	// delivery wakes asleep agents, which is refused until Ready is done.
	Connected func(ctx context.Context)
	Publish   func(events.Event)
	Every     time.Duration // how often the watcher checks Podman (default 3 s)
}

// ErrNotReady means agents can't start yet: Podman isn't there or isn't
// running, or the instances aren't restored yet.
var ErrNotReady = errors.New("podman isn't running")

// Setup is the checklist and the engine's watcher.
type Setup struct {
	Deps
	poke chan struct{}

	mu    sync.Mutex
	ready bool         // Ready ran
	last  podman.State // what the last check saw, to publish changes
	// The init check's cached result: "" (not run yet), "working", "done"
	// or "failed", with Podman's error. initGen counts the checks started and
	// Podman's departures: a check stores its result only while it is still
	// the latest, so one that outlived Podman going away is dropped.
	initState, initErr string
	initGen            int
}

// New returns the setup block; Run starts watching.
func New(d Deps) *Setup {
	if d.Every == 0 {
		d.Every = 3 * time.Second
	}
	return &Setup{Deps: d, poke: make(chan struct{}, 1)}
}

// Run keeps the engine connected until ctx ends: at once (starting a stopped
// macOS machine, as Foreman's launch always has), then every Every, or sooner
// when poked. The first time the engine answers, it runs Ready; each time it
// answers again, it builds the agent image and checks Podman can start agents
// on it, in the background.
func (s *Setup) Run(ctx context.Context) {
	start := true
	for {
		s.check(ctx, start)
		start = false
		select {
		case <-ctx.Done():
			return
		case <-s.poke:
		case <-time.After(s.Every):
		}
	}
}

func (s *Setup) check(ctx context.Context, start bool) {
	if s.Engine == nil {
		s.becomeReady(ctx, false)
		return
	}
	err := s.Engine.Connect(ctx, start)
	state := s.Engine.State(ctx)
	s.mu.Lock()
	changed := state.Installed != s.last.Installed || state.Machine != s.last.Machine ||
		state.Running != s.last.Running || state.Error != s.last.Error
	connected := err == nil && !s.last.Running
	if s.last.Running && !state.Running {
		// Podman went away: what the init check found no longer holds, and
		// a check still running then drops its result.
		s.initState, s.initErr = "", ""
		s.initGen++
	}
	s.last = state
	s.mu.Unlock()
	if changed {
		s.changed()
	}
	if err != nil {
		return
	}
	if connected {
		go s.build(context.WithoutCancel(ctx))
	}
	s.becomeReady(ctx, connected)
}

// becomeReady runs Ready until it succeeds once, then Connected; after that,
// Connected runs whenever the engine answers again (again).
func (s *Setup) becomeReady(ctx context.Context, again bool) {
	s.mu.Lock()
	ready := s.ready
	s.mu.Unlock()
	if ready {
		if again {
			s.connected(ctx)
		}
		return
	}
	if err := s.Ready(ctx); err != nil {
		slog.Error("restoring after the engine came up", "err", err)
		return
	}
	s.mu.Lock()
	s.ready = true
	s.mu.Unlock()
	s.changed()
	s.connected(ctx)
}

func (s *Setup) connected(ctx context.Context) {
	if s.Connected != nil {
		s.Connected(ctx)
	}
}

// build makes sure the agent image exists, so the first deploy doesn't wait,
// then checks that Podman can start an agent on it.
func (s *Setup) build(ctx context.Context) {
	s.changed()
	ref, err := s.Images.Ensure(ctx, "")
	s.changed()
	if err != nil {
		slog.Error("building the agent image", "err", err)
		return
	}
	s.checkInit(ctx, ref)
}

// initTimeout bounds the init check: a throwaway container that runs `true`.
const initTimeout = time.Minute

// checkInit runs a throwaway container with an init, as agents run, and
// caches the result for the init step. While one runs, no other starts,
// unless Podman went away and came back meanwhile: the new one is then the
// latest, and only its result is kept.
func (s *Setup) checkInit(ctx context.Context, image string) {
	s.mu.Lock()
	if s.initState == "working" {
		s.mu.Unlock()
		return
	}
	s.initState, s.initErr = "working", ""
	s.initGen++
	gen := s.initGen
	s.mu.Unlock()
	s.changed()
	ctx, cancel := context.WithTimeout(ctx, initTimeout)
	err := s.Engine.CheckInit(ctx, image)
	cancel()
	s.mu.Lock()
	if s.initGen == gen { // else Podman went away, or a newer check started
		s.initState = "done"
		if err != nil {
			s.initState, s.initErr = "failed", err.Error()
		}
	}
	s.mu.Unlock()
	if err != nil {
		slog.Error("podman can't start a container with an init", "err", err)
	}
	s.changed()
}

func (s *Setup) changed() {
	s.Publish(events.Event{Type: "setup.changed", Data: struct{}{}})
}

// Poke makes the watcher check now.
func (s *Setup) Poke() {
	select {
	case s.poke <- struct{}{}:
	default:
	}
}

// Blocked is nil when agents can start, or why they can't: deploying and
// waking an agent wait for it.
func (s *Setup) Blocked() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch {
	case s.Deps.Engine != nil && !s.last.Running:
		return fmt.Errorf("%w: see the setup checklist on the Dashboard", ErrNotReady)
	case !s.ready:
		return errors.New("foreman is still restoring the agents")
	}
	return nil
}

// StartPodman starts the stopped macOS machine (the CEO's Start), in the
// background: the checklist follows through setup.changed.
func (s *Setup) StartPodman(ctx context.Context) {
	if s.Deps.Engine == nil {
		return
	}
	go func() {
		if err := s.Deps.Engine.Connect(context.WithoutCancel(ctx), true); err != nil {
			slog.Error("starting Podman", "err", err)
		}
		s.Poke()
	}()
}

// CreateMachine creates the macOS Podman machine with cpus and memory (GiB).
func (s *Setup) CreateMachine(ctx context.Context, cpus, memory int) error {
	if s.Deps.Engine == nil {
		return errors.New("setup: the Direct engine has no machine")
	}
	if err := s.Deps.Engine.CreateMachine(ctx, cpus, memory); err != nil {
		return err
	}
	s.Poke()
	return nil
}

// Steps is the checklist now, in order.
func (s *Setup) Steps(ctx context.Context) []Step {
	// Always done: the first start made it. Restore is an offer, not a step.
	steps := []Step{{Kind: "hq", State: "done", Restorable: s.Config.Pristine(ctx)}}
	if s.Deps.Engine != nil {
		s.mu.Lock()
		ready := s.ready
		s.mu.Unlock()
		for _, step := range podmanSteps(s.Deps.Engine.State(ctx), runtime.GOOS) {
			if step.Kind == "running" && step.State == "done" && !ready {
				step.State, step.Detail = "working", "Restoring the agents"
			}
			steps = append(steps, step)
		}
	}
	for _, im := range s.Images.Status(ctx) {
		steps = append(steps, imageStep(im))
	}
	if s.Deps.Engine != nil { // the init check runs on the image: after its rows
		s.mu.Lock()
		state, podmanErr := s.initState, s.initErr
		s.mu.Unlock()
		steps = append(steps, initStep(state, podmanErr, runtime.GOOS))
	}
	teams, projects := s.Config.Teams(), s.Config.Projects()
	for _, p := range s.Profiles() {
		step := Step{Kind: "profile", Subject: p.Name, State: "done"}
		switch {
		case p.Harness == "":
			step.State, step.Detail = "failed", "No such profile: create it under Settings → Agents → Profiles, or pick another in the project's settings"
		case !p.Connected:
			step.State, step.Detail = "todo", "Not connected yet: its agents can't start"
		}
		steps = append(steps, step)
	}
	steps = append(steps, presence("team", len(teams), "Create a team"))
	steps = append(steps, presence("project", len(projects), "Add a project"))
	return steps
}

func presence(kind string, n int, todo string) Step {
	if n > 0 {
		return Step{Kind: kind, State: "done"}
	}
	return Step{Kind: kind, State: "todo", Detail: todo}
}

func podmanSteps(st podman.State, goos string) []Step {
	install := "Install Podman: sudo dnf install podman (or your distribution's package)"
	if goos == "darwin" {
		install = "Install Podman: brew install podman"
	}
	steps := []Step{{Kind: "podman", State: "done", Detail: st.Version}}
	if !st.Installed {
		steps[0].State, steps[0].Detail = "todo", install
	}
	if goos == "darwin" {
		machine := Step{Kind: "machine", State: "done", Log: st.Log}
		switch st.Machine {
		case "":
			machine.State, machine.Detail = "todo", "Needs Podman installed first"
		case "none":
			machine.State, machine.Detail = "todo", "Create the Podman machine, where agents run"
			if b := st.Bounds; b != nil {
				machine.Machine = &MachineSize{MinMemory: b.MinMemory, MaxMemory: b.MaxMemory,
					Memory: b.Memory, MaxCPUs: b.MaxCPUs, CPUs: b.CPUs}
			}
		case "creating":
			machine.State, machine.Detail = "working", "Creating the Podman machine"
		}
		steps = append(steps, machine)
	}
	running := Step{Kind: "running", State: "done", Detail: st.Version}
	switch {
	case st.Running:
	case !st.Installed || st.Machine == "none" || st.Machine == "creating":
		running.State, running.Detail = "todo", "Waits for the steps above"
	case st.Machine == "stopped":
		running.State, running.Detail = "todo", "The Podman machine is stopped (Start)"
	default:
		running.State, running.Detail = "failed", st.Error
	}
	return append(steps, running)
}

func imageStep(im runner.ImageStatus) Step {
	step := Step{Kind: "image", Subject: im.Name, Log: im.Log}
	switch im.State {
	case "ready":
		step.State = "done"
	case "building":
		step.State, step.Detail = "working", "Building the agent image (a few minutes, once)"
	case "failed":
		step.State, step.Detail = "failed", "The agent image didn't build (Settings → System → Maintenance: Rebuild)"
	default:
		step.State, step.Detail = "todo", "Builds once Podman runs"
	}
	return step
}

// initStep is the row for the init check's cached result (Steps never runs
// it). The catatonit fix is for Linux only: on macOS the init lives in the
// Podman machine, where dnf advice from the host would be wrong.
func initStep(state, podmanErr, goos string) Step {
	step := Step{Kind: "init", State: "done"}
	switch state {
	case "":
		step.State, step.Detail = "todo", "Waits for the steps above"
	case "working":
		step.State, step.Detail = "working", "Starting a test container"
	case "failed":
		step.State, step.Log = "failed", podmanErr
		step.Detail = "Podman can't start a test container, so agents can't start either. Its error is below."
		if goos == "linux" && missingInit(podmanErr) {
			step.Detail = "Agents can't start: Podman can't find catatonit, the small program every agent runs under. " +
				"Install it (sudo dnf install catatonit, or your distribution's package), then restart Foreman."
		}
	}
	return step
}

// missingInit reports whether Podman's error says its init binary (catatonit)
// isn't on the host. Podman words it "container-init binary not found on the
// host"; matching from "init" also takes the spelling without the hyphen.
func missingInit(podmanErr string) bool {
	return strings.Contains(podmanErr, "init binary not found")
}
