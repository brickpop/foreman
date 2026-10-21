// Package instances decides which instances run: one per member of each
// active deployment (wiki/product/architecture/building-blocks.md#5-instance-manager-instances).
// It prepares each one's sandbox, launches its harness in the engine, keeps
// its terminal, restarts it once after a crash, queues beyond the concurrency
// limit, and re-attaches after a Foreman restart
// (wiki/product/architecture/agent-runtime.md).
//
// Only the lead starts on deploy; the other members sleep until they are
// woken (wiki/product/architecture/agent-runtime.md#lazy-starts).
//
// Epic 1: no agent link yet, so statuses are the lifecycle's own (asleep,
// queued, starting, running, stopped); idle, awaiting and done come with the hooks.
package instances

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/brickpop/foreman/internal/events"
	"github.com/brickpop/foreman/internal/harness"
	"github.com/brickpop/foreman/internal/home"
	"github.com/brickpop/foreman/internal/profiles"
	"github.com/brickpop/foreman/internal/prompt"
	"github.com/brickpop/foreman/internal/runner"
	"github.com/brickpop/foreman/internal/store"
	"github.com/brickpop/foreman/internal/terminal"
)

// Statuses (wiki/product/models/instance.md#statuses).
const (
	Asleep   = "asleep"
	Queued   = "queued"
	Starting = "starting"
	Running  = "running"
	Idle     = "idle"
	Awaiting = "awaiting"
	Stopped  = "stopped"
)

// Git is what instances need of the Git block: a worktree per code repo,
// with its submodules.
type Git interface {
	Prepare(ctx context.Context, repo, worktree, branch, base string) error
	SyncSubmodules(ctx context.Context, dir string, recursive bool) error
}

// Config is the home's config, as loaded.
type Config interface {
	Project(id string) (home.Project, error)
	// Checked is the project with its repos checked now: one may have moved
	// since the last reload, which the home doesn't watch.
	Checked(id string) (home.Project, error)
	// Sandbox is the folder of an agent's worktrees on a project.
	Sandbox(project, agent string) string
	Team(id string) (home.Team, error)
	Settings() home.SettingsFile
}

// Deps are the blocks the manager is assembled from.
type Deps struct {
	Home        string // Foreman's home: its processes are labelled with it
	Runner      runner.Runner
	Images      runner.Images
	Git         Git
	Config      Config
	Deployments store.Deployments
	Instances   store.Instances
	Adapters    map[string]harness.Adapter // by harness name (claude-code, …)
	// Profile is the profile a project's agents run on (a harness and its
	// credentials) and its folder, as it is when asked: a change reaches
	// each agent at its next start.
	Profile func(project string) (profiles.Profile, string, error)
	OpenLog func(instance string) (terminal.Log, error)
	Publish func(events.Event)
	Agent   AgentLink
	// Blocked is nil when agents can start, or why not (Podman isn't running);
	// deploying and waking refuse meanwhile. nil: always ready.
	Blocked func() error
	// Recheck is how long a project found gone is given to come back before
	// its deployments stop (ProjectsChanged): a minute when zero.
	Recheck time.Duration
	// Requests is where a stuck agent's card goes (CheckStuck); nil: none.
	Requests store.Requests
}

// AgentLink is foreman-agent as the instances get it: mounted into each
// container, reaching Foreman's agent port with the instance's token
// (wiki/product/architecture/team-tools.md#transport).
type AgentLink struct {
	Host  string // the binary on the host; "" when it isn't built: agents start without team tools
	Path  string // where it is in the container (the Direct engine: Host)
	URL   string // the agent port, as an instance reaches it
	Token func(instance string) string
}

// Manager runs the instances of every active deployment.
type Manager struct {
	Deps
	mu      sync.Mutex
	starts  sync.WaitGroup   // starts under way
	live    map[string]*live // instances with a process or a start under way
	queue   []string         // queued instances, in creation order
	closing bool             // Foreman is exiting: nothing starts any more
	// doing is each instance's activity line in its current run (Hook).
	doing map[string]store.Doing
	// gone are the projects found gone, waiting for their recheck.
	gone map[string]bool
	// heard is when each live instance last showed activity: a hook, or its run
	// starting (CheckStuck).
	heard map[string]time.Time
	// stuck are the open cards of agents found stuck, by instance.
	stuck map[string]store.Request
}

type live struct {
	term     *terminal.Terminal // nil while starting
	proc     runner.Process
	stopping bool // a stop was asked: its end is not a crash
}

// New returns a manager.
func New(d Deps) *Manager {
	return &Manager{Deps: d, live: map[string]*live{}, doing: map[string]store.Doing{}, gone: map[string]bool{},
		heard: map[string]time.Time{}, stuck: map[string]store.Request{}}
}

var (
	// ErrNotRunning means the instance has no terminal now.
	ErrNotRunning = errors.New("instances: not running")
	// ErrInvalid means the team or project can't be deployed as it is.
	ErrInvalid = errors.New("instances: can't deploy")
	// ErrNotReady means agents can't start yet (Deps.Blocked).
	ErrNotReady = errors.New("agents can't start yet")
)

// stopGrace is how long a harness gets to end before it is killed.
const stopGrace = 10 * time.Second

// Deploy deploys team on project with goal and starts its lead; the other
// members sleep until woken.
func (m *Manager) Deploy(ctx context.Context, teamID, projectID, goal string) (store.Deployment, error) {
	if err := m.blocked(); err != nil {
		return store.Deployment{}, err
	}
	team, project, on, err := m.deployable(ctx, teamID, projectID)
	if err != nil {
		return store.Deployment{}, err
	}
	dep, err := m.Deployments.Deploy(ctx, teamID, projectID, goal)
	if err != nil {
		return store.Deployment{}, err
	}
	m.publish("deployment.created", dep)
	for _, agent := range team.Config.Roster() {
		inst := store.Instance{
			ID: agent + "@" + projectID, DeploymentID: dep.ID, AgentID: agent, Status: Asleep,
			SandboxPath: m.Config.Sandbox(project.ID, agent), Harness: on.harness, Profile: on.profile,
		}
		if err := m.Instances.Put(ctx, inst); err != nil {
			return dep, err
		}
		if agent == team.Config.Lead {
			m.enqueue(inst.ID)
		}
	}
	m.startQueued()
	return dep, nil
}

// Wake starts an asleep instance (the CEO's Start, or work arriving for it),
// or, on the CEO's Start again, a stopped one of an active deployment: both
// pick up their conversation. An instance that is up or on its way is left as
// it is.
func (m *Manager) Wake(ctx context.Context, id string) error {
	inst, err := m.Instances.Get(ctx, id)
	if err != nil {
		return err
	}
	if inst.Status != Asleep && inst.Status != Stopped {
		return nil
	}
	if dep, err := m.Deployments.Get(ctx, inst.DeploymentID); err != nil || dep.Status != store.DeploymentActive {
		return nil // a stopped deployment's instances start only with a redeploy
	}
	if err := m.blocked(); err != nil {
		return err
	}
	if inst.Restarts != 0 {
		inst.Restarts = 0 // the CEO's start: a later crash gets its one restart again
		if err := m.Instances.Put(ctx, inst); err != nil {
			return err
		}
	}
	m.enqueue(id)
	m.startQueued()
	return nil
}

// Restart stops a running instance and starts it again at once, on its
// project's profile as it is now (Project settings → Profile), continuing
// its conversation unless the harness changed. One that isn't up is left as
// it is: it starts on that profile anyway.
func (m *Manager) Restart(ctx context.Context, id string) error {
	inst, err := m.Instances.Get(ctx, id)
	if err != nil {
		return err
	}
	switch inst.Status {
	case Starting, Running, Idle, Awaiting:
	default:
		return nil
	}
	if err := m.blocked(); err != nil {
		return err
	}
	m.mu.Lock()
	l := m.live[id]
	if l != nil {
		l.stopping = true // its end is not a crash
	}
	m.mu.Unlock()
	if l != nil && l.proc != nil {
		if err := l.proc.Stop(stopGrace); err != nil {
			return err
		}
	}
	// As a stop: the slot is free at once, and a run still starting ends
	// itself without touching the new one.
	if l != nil {
		m.release(id, l)
	}
	// Read again: hooks may have changed it while it stopped.
	if inst, err = m.Instances.Get(ctx, id); err != nil {
		return err
	}
	if inst.Restarts != 0 {
		inst.Restarts = 0 // the CEO's restart: a later crash gets its one restart again
		if err := m.Instances.Put(ctx, inst); err != nil {
			return err
		}
	}
	m.enqueue(id)
	m.startQueued()
	return nil
}

// blocked is why agents can't start now, wrapping ErrNotReady, or nil.
func (m *Manager) blocked() error {
	m.mu.Lock()
	closing := m.closing
	m.mu.Unlock()
	if closing {
		return fmt.Errorf("%w: foreman is stopping", ErrNotReady)
	}
	if m.Blocked == nil {
		return nil
	}
	if err := m.Blocked(); err != nil {
		return fmt.Errorf("%w: %v", ErrNotReady, err)
	}
	return nil
}

// runsOn is what a project's agents run on: its profile's harness, its
// adapter, and the profile's folder (the harness's login).
type runsOn struct {
	profile string
	harness string
	adapter harness.Adapter
	login   string
}

// deployable checks what deploying needs: a valid team and project, and the
// project's profile, with an adapter for its harness and its credentials.
func (m *Manager) deployable(ctx context.Context, teamID, projectID string) (home.Team, home.Project, runsOn, error) {
	var on runsOn
	team, err := m.Config.Team(teamID)
	if err != nil {
		return team, home.Project{}, on, fmt.Errorf("%w: team %s: %v", ErrInvalid, teamID, err)
	}
	project, err := m.Config.Checked(projectID)
	if err != nil {
		return team, project, on, fmt.Errorf("%w: project %s: %v", ErrInvalid, projectID, err)
	}
	if len(team.Problems) > 0 {
		return team, project, on, fmt.Errorf("%w: team %s has problems: %v", ErrInvalid, teamID, team.Problems)
	}
	if len(project.Problems) > 0 {
		return team, project, on, fmt.Errorf("%w: project %s has problems: %v", ErrInvalid, projectID, project.Problems)
	}
	if len(project.Repos) == 0 {
		return team, project, on, fmt.Errorf("%w: project %s has no code repo for agents to work in: add one to it first", ErrInvalid, projectID)
	}
	if err := m.sharing(ctx, teamID, project); err != nil {
		return team, project, on, err
	}
	p, login, err := m.Profile(projectID)
	if err != nil {
		return team, project, on, fmt.Errorf("%w: project %s's profile: %v (Settings → Agents → Profiles, and its Project settings)", ErrInvalid, projectID, err)
	}
	on = runsOn{profile: p.Name, harness: p.Harness, adapter: m.Adapters[p.Harness], login: login}
	if on.adapter == nil {
		return team, project, on, fmt.Errorf("%w: profile %s's harness %q isn't supported yet", ErrInvalid, p.Name, p.Harness)
	}
	// Refused here, not when the agent starts, where it would only reach the log.
	if !on.adapter.Connected(login) {
		return team, project, on, fmt.Errorf("%w: profile %s isn't connected yet: Connect it under Settings → Agents → Profiles", ErrInvalid, p.Name)
	}
	return team, project, on, nil
}

// Active reports whether a deployment still runs (it isn't stopped).
func (m *Manager) Active(ctx context.Context, deploymentID string) (bool, error) {
	dep, err := m.Deployments.Get(ctx, deploymentID)
	if errors.Is(err, store.ErrNotFound) {
		return false, nil
	}
	return dep.Status == store.DeploymentActive, err
}

// Stop stops a deployment's instances and marks it stopped (the CEO's Stop).
// Their worktrees stay on disk.
func (m *Manager) Stop(ctx context.Context, deploymentID string) error {
	return m.stop(ctx, deploymentID, "")
}

// stop is Stop, with why when Foreman stops it on its own.
func (m *Manager) stop(ctx context.Context, deploymentID, reason string) error {
	dep, err := m.Deployments.Get(ctx, deploymentID)
	if err != nil {
		return err
	}
	insts, err := m.Instances.List(ctx, deploymentID)
	if err != nil {
		return err
	}
	for _, inst := range insts {
		m.mu.Lock()
		m.queue = remove(m.queue, inst.ID)
		l := m.live[inst.ID]
		if l != nil {
			l.stopping = true
		}
		m.mu.Unlock()
		if l != nil && l.proc != nil {
			if err := l.proc.Stop(stopGrace); err != nil {
				slog.Warn("stopping an instance", "instance", inst.ID, "err", err)
			}
		}
		// Its slot is free at once, also mid-start, so a redeploy right after
		// starts a new run; the old run's late end then changes nothing.
		if l != nil {
			m.release(inst.ID, l)
		}
		m.setStatus(ctx, inst.ID, Stopped)
	}
	if err := m.Deployments.Stop(ctx, deploymentID, reason); err != nil {
		return err
	}
	if dep.Status == store.DeploymentActive {
		dep.StopReason = reason
	}
	dep.Status = store.DeploymentStopped
	m.publish("deployment.stopped", dep)
	m.startQueued() // its slots go to whoever waits
	return nil
}

// Shutdown stops every instance's process as Foreman exits, all at once, each
// with the usual grace. Each goes to sleep, not to stopped: work or the CEO's
// Start wakes it on a later run, continuing its conversation. One whose stop
// fails (Podman not answering) keeps its status, so the next start re-attaches
// it as after a crash. Nothing starts after it
// (wiki/product/architecture/agent-runtime.md#recovery).
func (m *Manager) Shutdown(ctx context.Context) {
	m.mu.Lock()
	m.closing = true
	ids := m.queue
	m.queue = nil
	running := map[string]*live{}
	for id, l := range m.live {
		// A start under way sees it and stops what it launched.
		l.stopping = true
		ids = append(ids, id)
		if l.proc != nil {
			running[id] = l
		}
	}
	m.mu.Unlock()
	var stops sync.WaitGroup
	var failedMu sync.Mutex
	failed := map[string]bool{}
	for id, l := range running {
		stops.Go(func() {
			if err := l.proc.Stop(stopGrace); err != nil {
				slog.Warn("stopping an instance; the next start re-attaches it", "instance", id, "err", err)
				failedMu.Lock()
				failed[id] = true
				failedMu.Unlock()
				return
			}
			// Its terminal has read the end (and closed its log), and the slot
			// is free, before Shutdown returns: its watcher would do both later.
			<-l.term.Done()
			m.release(id, l)
		})
	}
	stops.Wait()
	m.Wait()
	for _, id := range ids {
		if !failed[id] {
			m.setStatus(ctx, id, Asleep)
		}
	}
}

// Live reports whether the instance has a process or a start under way: the
// agent link accepts its token only then.
func (m *Manager) Live(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.live[id] != nil
}

// Hook takes one of an instance's harness hook events: what it says the
// harness is doing becomes the instance's status (running, idle, awaiting)
// and its activity line. Before the instance runs, or once it stops, the
// event changes nothing.
func (m *Manager) Hook(id, event string, payload []byte) {
	ctx := context.Background()
	inst, err := m.Instances.Get(ctx, id)
	if err != nil {
		return
	}
	adapter := m.Adapters[inst.Harness]
	if adapter == nil || (inst.Status != Running && inst.Status != Idle && inst.Status != Awaiting) {
		return
	}
	m.heardFrom(ctx, id)
	if status, ok := map[harness.Activity]string{harness.Busy: Running, harness.Idle: Idle, harness.Waiting: Awaiting}[adapter.Interpret(event, payload)]; ok {
		m.setStatus(ctx, id, status)
	}
	if line := oneLine(adapter.Describe(event, payload)); line != "" {
		a := store.Doing{Line: line, At: time.Now().UTC()}
		m.mu.Lock()
		m.doing[id] = a
		m.mu.Unlock()
		m.publish("instance.doing", DoingChange{ID: id, Doing: a})
	}
}

// DoingChange is the instance.doing event's data.
type DoingChange struct {
	ID    string      `json:"id"`
	Doing store.Doing `json:"doing"`
}

// doingMax is the most of a line kept, in characters: a row shows one line.
const doingMax = 200

// oneLine is a description as a row shows it: its first line, without
// control characters (a payload's text is the harness's, so anything), cut
// at doingMax.
func oneLine(s string) string {
	s, _, _ = strings.Cut(strings.TrimSpace(s), "\n")
	s = strings.TrimSpace(strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, s))
	if r := []rune(s); len(r) > doingMax {
		s = string(r[:doingMax-1]) + "…"
	}
	return s
}

// Type writes p into the instance's terminal, as if typed (a delivery).
func (m *Manager) Type(id string, p []byte) error {
	term, err := m.Terminal(id)
	if err != nil {
		return err
	}
	return term.Input(p)
}

// Newline is the keystroke that starts a new line in the instance's harness
// without sending; Enter when the harness isn't known.
func (m *Manager) Newline(id string) string {
	inst, err := m.Instances.Get(context.Background(), id)
	if err != nil || m.Adapters[inst.Harness] == nil {
		return "\r"
	}
	return m.Adapters[inst.Harness].Newline()
}

// Terminal returns a running instance's terminal.
func (m *Manager) Terminal(id string) (*terminal.Terminal, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if l := m.live[id]; l != nil && l.term != nil {
		return l.term, nil
	}
	return nil, ErrNotRunning
}

// Restore picks up after a Foreman restart: this home's running containers
// are re-attached, live instances whose process is gone restart (continuing
// their conversation), and this home's processes that belong to nothing are
// stopped. Stuck cards from before are resolved: the watch starts over, and
// raises them again if the agent is still quiet. Another home's (a test's, just dev's) are left alone.
func (m *Manager) Restore(ctx context.Context) error {
	m.resolveOldStuck(ctx)
	procs, err := m.Runner.List(ctx)
	if err != nil {
		return err
	}
	byID := map[string]runner.Process{} // this home's
	for _, p := range procs {
		if p.Labels()[HomeLabel] == m.Home {
			byID[p.Labels()["foreman.instance"]] = p
		}
	}
	deps, err := m.Deployments.List(ctx)
	if err != nil {
		return err
	}
	for _, dep := range deps {
		if dep.Status != store.DeploymentActive {
			continue
		}
		insts, err := m.Instances.List(ctx, dep.ID)
		if err != nil {
			return err
		}
		for _, inst := range insts {
			if inst.Status == Stopped || inst.Status == Asleep {
				continue
			}
			if p, ok := byID[inst.ID]; ok {
				delete(byID, inst.ID)
				if err := m.adopt(ctx, inst.ID, p); err != nil {
					slog.Warn("re-attaching", "instance", inst.ID, "err", err)
				}
				continue
			}
			m.enqueue(inst.ID)
		}
	}
	for id, p := range byID {
		slog.Warn("stopping an orphan process: no live instance", "instance", id)
		go func() { _ = p.Stop(stopGrace) }()
	}
	m.startQueued()
	return nil
}

// adopt takes over a process found running.
func (m *Manager) adopt(ctx context.Context, id string, p runner.Process) error {
	log, err := m.OpenLog(id)
	if err != nil {
		_ = p.Stop(stopGrace)
		return err
	}
	l := &live{proc: p, term: terminal.New(id, p, log)}
	m.mu.Lock()
	if m.closing {
		// Exiting: left running, for the next start to re-attach.
		m.mu.Unlock()
		_ = log.Close()
		return nil
	}
	m.live[id] = l
	m.mu.Unlock()
	// Its process lived on: what its hooks last said still holds (idle stays
	// idle, so a message queued for it is delivered).
	if inst, err := m.Instances.Get(ctx, id); err != nil || (inst.Status != Idle && inst.Status != Awaiting) {
		m.setStatus(ctx, id, Running)
	}
	m.heardFrom(ctx, id)
	go m.watch(id, l)
	return nil
}

func (m *Manager) enqueue(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.live[id] == nil && !contains(m.queue, id) {
		m.queue = append(m.queue, id)
	}
}

// startQueued starts queued instances while there are free slots
// (max_sessions; a slot is held by every instance with a process or a start).
// Those left waiting are marked queued: only they, so an instance that starts
// at once never shows queued (nor warns the CEO about slots).
func (m *Manager) startQueued() {
	max := m.Config.Settings().Settings.MaxSessions
	for {
		m.mu.Lock()
		if len(m.queue) == 0 || len(m.live) >= max || m.closing {
			waiting := slices.Clone(m.queue)
			closing := m.closing
			m.mu.Unlock()
			if !closing {
				for _, id := range waiting {
					m.markQueued(id)
				}
			}
			return
		}
		id := m.queue[0]
		m.queue = m.queue[1:]
		l := &live{}
		m.live[id] = l
		m.starts.Add(1)
		m.mu.Unlock()
		go func() {
			defer m.starts.Done()
			m.start(id, l)
		}()
	}
}

// LimitChanged starts the queued instances that fit under a new session limit
// (settings.toml changed). A lower limit stops nothing: running ones finish.
func (m *Manager) LimitChanged() { m.startQueued() }

// markQueued marks an instance queued if it still waits: the check and the
// write hold the lock a start takes to leave the queue, so one that started
// meanwhile is never marked.
func (m *Manager) markQueued(id string) {
	ctx := context.Background()
	m.mu.Lock()
	marked := false
	if contains(m.queue, id) {
		if inst, err := m.Instances.Get(ctx, id); err == nil && inst.Status != Queued {
			inst.Status = Queued
			marked = m.Instances.Put(ctx, inst) == nil
		}
	}
	m.mu.Unlock()
	if marked {
		m.publish("instance.status", StatusChange{ID: id, Status: Queued})
	}
}

// start prepares the instance's sandbox, launches its harness and keeps its
// terminal. A failure stops the instance (with its reason in the log).
func (m *Manager) start(id string, l *live) {
	ctx := context.Background()
	// A new run: what the last one did is no longer what it is doing.
	m.mu.Lock()
	delete(m.doing, id)
	m.mu.Unlock()
	m.setStatus(ctx, id, Starting)
	proc, err := m.launch(ctx, id)
	var log terminal.Log
	if err == nil {
		if log, err = m.OpenLog(id); err != nil {
			_ = proc.Stop(stopGrace)
			err = fmt.Errorf("session log: %w", err)
		}
	}
	if err == nil {
		// Checked and handed over in one step: a Stop either sees the process
		// (and stops it) or is seen here.
		m.mu.Lock()
		stopping := l.stopping
		if !stopping {
			l.proc, l.term = proc, terminal.New(id, proc, log)
		}
		m.mu.Unlock()
		if stopping {
			_ = proc.Stop(stopGrace)
			_ = log.Close()
			err = errors.New("stopped while starting")
		}
	}
	if err != nil {
		slog.Warn("starting an instance", "instance", id, "err", err)
		// A stop already freed the slot and set the status; a run may have
		// replaced this one since.
		if m.release(id, l) {
			m.stopped(ctx, id, "couldn't start: "+err.Error())
		}
		m.startQueued()
		return
	}
	inst, _ := m.Instances.Get(ctx, id)
	inst.Status, inst.StartedAt, inst.StopReason = Running, time.Now(), ""
	_ = m.Instances.Put(ctx, inst)
	m.publish("instance.status", StatusChange{ID: id, Status: Running})
	m.heardFrom(ctx, id)
	go m.watch(id, l)
}

// launch does the work of a start: image, worktrees, prompt, harness files,
// process.
func (m *Manager) launch(ctx context.Context, id string) (runner.Process, error) {
	inst, err := m.Instances.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	dep, err := m.Deployments.Get(ctx, inst.DeploymentID)
	if err != nil {
		return nil, err
	}
	team, project, on, err := m.deployable(ctx, dep.TeamID, dep.ProjectID)
	if err != nil {
		return nil, err
	}
	adapter := on.adapter
	// The project's profile as it is now: a change reaches the agent here.
	// Another harness can't continue this one's conversation: the run starts
	// fresh, and a lead gets its goal again.
	switched := inst.Harness != on.harness
	if switched || inst.Profile != on.profile {
		inst.Harness, inst.Profile = on.harness, on.profile
		if err := m.Instances.Put(ctx, inst); err != nil {
			return nil, err
		}
	}
	lead := inst.AgentID == team.Config.Lead

	if err := os.MkdirAll(inst.SandboxPath, 0o755); err != nil {
		return nil, err
	}
	branch := "wt/" + inst.AgentID + "/main"
	if lead {
		branch = "lead/" + team.ID + "/integration"
	}
	var mounts []runner.Mount
	mounts = append(mounts, runner.Mount{Source: inst.SandboxPath, Target: inst.SandboxPath})
	for _, repo := range project.Repos {
		repoDir, worktree := repo.Path, filepath.Join(inst.SandboxPath, repo.Name)
		if err := m.Git.Prepare(ctx, repoDir, worktree, branch, repo.Main); err != nil {
			return nil, err
		}
		// An agent starts without a submodule that can't be fetched (offline,
		// a private repo) rather than not at all: its next switch or merge
		// tries again, and says so.
		if err := m.Git.SyncSubmodules(ctx, worktree, project.Config.RecursiveSubmodules); err != nil {
			slog.Warn("checking out the submodules", "instance", inst.ID, "repo", repo.Name, "err", err)
		}
		// Agents read git; only Foreman writes it. At its real path, which the
		// worktree's link names.
		real, err := filepath.EvalSymlinks(repoDir)
		if err != nil {
			return nil, err
		}
		mounts = append(mounts, runner.Mount{Source: filepath.Join(real, ".git"), Target: filepath.Join(real, ".git"), ReadOnly: true})
	}
	// The live project repo, writable for the lead only; its .git read-only for all.
	projectRepo, inSandbox := project.Path, filepath.Join(inst.SandboxPath, "project")
	if err := os.MkdirAll(inSandbox, 0o755); err != nil {
		return nil, err
	}
	mounts = append(mounts, runner.Mount{Source: projectRepo, Target: inSandbox, ReadOnly: !lead})
	if _, err := os.Stat(filepath.Join(projectRepo, ".git")); err == nil {
		mounts = append(mounts, runner.Mount{Source: filepath.Join(projectRepo, ".git"), Target: filepath.Join(inSandbox, ".git"), ReadOnly: true})
	}

	text := m.compose(inst, dep, team, project)
	first := inst.StartedAt.IsZero()
	session := harness.Session{
		Instance: id, Sandbox: inst.SandboxPath, Prompt: text, Login: on.login,
		// A restart (after a crash, or a Foreman restart) picks up its conversation;
		// a new deployment, or another harness, starts fresh.
		Continue: !first && !switched,
	}
	if m.Agent.Host != "" {
		mounts = append(mounts, runner.Mount{Source: m.Agent.Host, Target: m.Agent.Path, ReadOnly: true})
		session.Link = harness.Link{Program: m.Agent.Path, Env: []string{
			"FOREMAN_AGENT_URL=" + m.Agent.URL, "FOREMAN_AGENT_TOKEN=" + m.Agent.Token(id),
		}}
	}
	if lead && dep.Goal != "" {
		// The kickoff (journey G1): the lead starts on the goal at once. The
		// adapter gives it only to a session that starts fresh, so a restart
		// that continues the conversation doesn't get the goal twice, and one
		// with nothing to continue still gets it.
		session.Opener = fmt.Sprintf("Your goal on %s: %s", project.ID, dep.Goal)
	}
	l, err := adapter.Launch(session)
	if err != nil {
		return nil, err
	}
	if err := harness.WriteFiles(l.Files); err != nil {
		return nil, err
	}
	image, err := m.Images.Ensure(ctx, project.ID)
	if err != nil {
		return nil, err
	}
	return m.Runner.Start(ctx, runner.Spec{
		Name: m.processName(id), Image: image, Command: l.Command, Dir: inst.SandboxPath, Env: l.Env,
		Mounts: append(mounts, l.Mounts...),
		Labels: map[string]string{"foreman.instance": id, "foreman.deployment": dep.ID, HomeLabel: m.Home},
	})
}

// HomeLabel names the Foreman home a process belongs to. Every Foreman on a
// machine (one per home: yours, just dev's, a test's) shares one Podman, so
// each touches only its own: two may well have an agent mara on a project
// ledger.
const HomeLabel = "foreman.home"

// processName is the instance's process name, unique on the machine.
func (m *Manager) processName(id string) string { return ProcessName(m.Home, id) }

// ProcessName is a home's process name for id, unique on the machine: the
// home's short hash, then the id.
func ProcessName(home, id string) string {
	sum := sha256.Sum256([]byte(home))
	return hex.EncodeToString(sum[:3]) + "-" + id
}

// watch waits for the instance's process to end: a stop asked for, or a crash
// (restarted once, continuing its conversation; a second crash stops it).
func (m *Manager) watch(id string, l *live) {
	<-l.term.Done()
	ctx := context.Background()
	m.unstuck(ctx, id, "its run ended")
	m.mu.Lock()
	delete(m.heard, id)
	m.mu.Unlock()
	m.mu.Lock()
	stopping := l.stopping
	m.mu.Unlock()
	m.release(id, l)
	if stopping {
		m.startQueued()
		return
	}
	inst, err := m.Instances.Get(ctx, id)
	code, _ := l.term.ExitCode()
	switch {
	case err != nil:
	case code == 0:
		// A clean exit (the CEO's /exit, say) is done for now, not a crash: it
		// sleeps, holding no slot, until work or the CEO's Start wakes it, and
		// then picks up its conversation.
		inst.Status, inst.Restarts = Asleep, 0
		_ = m.Instances.Put(ctx, inst)
		m.publish("instance.status", StatusChange{ID: id, Status: Asleep})
	case inst.Restarts == 0:
		slog.Warn("instance exited; restarting it once", "instance", id, "code", code)
		inst.Restarts++
		_ = m.Instances.Put(ctx, inst)
		m.enqueue(id)
	default:
		slog.Warn("instance exited again; stopped", "instance", id, "code", code)
		m.stopped(ctx, id, fmt.Sprintf("it exited twice in a row (code %d)", code))
	}
	m.startQueued()
}

// release frees the slot of one run of the instance: only its own, so the late
// end of a stopped run never frees the run that replaced it (a redeploy). It
// says whether the slot was still that run's.
func (m *Manager) release(id string, l *live) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.live[id] != l {
		return false
	}
	delete(m.live, id)
	return true
}

// StatusChange is the instance.status event's data.
type StatusChange struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

func (m *Manager) setStatus(ctx context.Context, id, status string) {
	inst, err := m.Instances.Get(ctx, id)
	if err != nil || inst.Status == status {
		return
	}
	// A reason is only Foreman's own stop's (stopped).
	inst.Status, inst.StopReason = status, ""
	if err := m.Instances.Put(ctx, inst); err != nil {
		slog.Warn("saving an instance", "instance", id, "err", err)
	}
	m.publish("instance.status", StatusChange{ID: id, Status: status})
}

// stopped stops an instance on Foreman's own decision, saying why: its pane
// shows it until the next start.
func (m *Manager) stopped(ctx context.Context, id, reason string) {
	inst, err := m.Instances.Get(ctx, id)
	if err != nil {
		return
	}
	inst.Status, inst.StopReason = Stopped, reason
	if err := m.Instances.Put(ctx, inst); err != nil {
		slog.Warn("saving an instance", "instance", id, "err", err)
	}
	m.publish("instance.status", StatusChange{ID: id, Status: Stopped})
}

func (m *Manager) publish(typ string, data any) {
	if m.Publish != nil {
		m.Publish(events.Event{Type: typ, Data: data})
	}
}

func contains(list []string, v string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
}

func remove(list []string, v string) []string {
	out := list[:0]
	for _, s := range list {
		if s != v {
			out = append(out, s)
		}
	}
	return out
}

// List returns the instances of every active deployment, by deployment.
func (m *Manager) List(ctx context.Context) ([]store.Instance, error) {
	deps, err := m.Deployments.List(ctx)
	if err != nil {
		return nil, err
	}
	list := []store.Instance{}
	for _, dep := range deps {
		if dep.Status != store.DeploymentActive {
			continue
		}
		insts, err := m.Instances.List(ctx, dep.ID)
		if err != nil {
			return nil, err
		}
		list = append(list, insts...)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range list {
		if a, ok := m.doing[list[i].ID]; ok {
			list[i].Doing = &a
		}
	}
	return list, nil
}

// DeploymentsIn returns the deployments with status (active or stopped).
func (m *Manager) DeploymentsIn(ctx context.Context, status string) ([]store.Deployment, error) {
	all, err := m.Deployments.List(ctx)
	if err != nil {
		return nil, err
	}
	list := []store.Deployment{}
	for _, d := range all {
		if d.Status == status {
			list = append(list, d)
		}
	}
	return list, nil
}

// compose is an instance's session prompt: launch writes it, Prompt shows it.
func (m *Manager) compose(inst store.Instance, dep store.Deployment, team home.Team, project home.Project) string {
	return prompt.Compose(prompt.Input{
		Agent: inst.AgentID, Team: team.Config, Project: project.ID, Sandbox: inst.SandboxPath, Config: project.Config, Repos: project.Repos, Goal: dep.Goal,
		Tools: m.tools(inst.AgentID == team.Config.Lead),
	})
}

// tools are the team tools an instance gets: none without foreman-agent.
func (m *Manager) tools(lead bool) []string {
	if m.Agent.Host == "" {
		return nil
	}
	return prompt.Offered(lead)
}

// Prompt is the session prompt an instance runs with, exactly as composed for
// its harness (the Agent page shows it).
func (m *Manager) Prompt(ctx context.Context, id string) (string, error) {
	inst, err := m.Instances.Get(ctx, id)
	if err != nil {
		return "", err
	}
	dep, err := m.Deployments.Get(ctx, inst.DeploymentID)
	if err != nil {
		return "", err
	}
	team, err := m.Config.Team(dep.TeamID)
	if err != nil {
		return "", err
	}
	project, err := m.Config.Project(dep.ProjectID)
	if err != nil {
		return "", err
	}
	return m.compose(inst, dep, team, project), nil
}

// Wait waits for the starts under way to finish (at shutdown, and in tests:
// a start may be writing worktrees).
func (m *Manager) Wait() { m.starts.Wait() }

// sharing refuses a team on a project that shares a repo with another
// project the team works on, stopped or not: its agents' branches
// (wt/<agent>/*, lead/<team>/*) and their worktrees, which a stopped
// deployment keeps, would collide in that repo.
func (m *Manager) sharing(ctx context.Context, teamID string, project home.Project) error {
	deps, err := m.Deployments.List(ctx)
	if err != nil {
		return err
	}
	for _, d := range deps {
		if d.TeamID != teamID || d.ProjectID == project.ID {
			continue
		}
		other, err := m.Config.Project(d.ProjectID)
		if err != nil {
			continue // gone: its worktrees with it
		}
		for _, r := range project.Repos {
			for _, o := range other.Repos {
				if samePath(r.Path, o.Path) {
					return fmt.Errorf("%w: team %s also works on %s, which shares the repo %s (%s): its branches can't be in two projects of one repo. Deploy another team here", ErrInvalid, teamID, other.ID, r.Name, r.Path)
				}
			}
		}
	}
	return nil
}

// samePath says whether two paths are one folder, symlinks resolved.
func samePath(a, b string) bool {
	ra, errA := filepath.EvalSymlinks(a)
	rb, errB := filepath.EvalSymlinks(b)
	if errA != nil || errB != nil {
		return filepath.Clean(a) == filepath.Clean(b)
	}
	return ra == rb
}

// ProjectsChanged looks for active deployments whose project is gone
// (home.Project.Gone: its project repo deleted, or every code repo missing)
// and gives each such project Recheck to come back (a move or a git
// operation in progress); still gone then, its deployments stop, with the
// reason (wiki/product/models/deployment.md#lifecycle). Called when the
// projects change, and once the instances are restored.
func (m *Manager) ProjectsChanged(ctx context.Context) {
	deps, err := m.DeploymentsIn(ctx, store.DeploymentActive)
	if err != nil {
		slog.Warn("looking for gone projects", "err", err)
		return
	}
	recheck := m.Recheck
	if recheck == 0 {
		recheck = time.Minute
	}
	for _, d := range deps {
		p, err := m.Config.Project(d.ProjectID)
		if err != nil || p.Gone() == "" {
			continue
		}
		m.mu.Lock()
		pending := m.gone[p.ID]
		m.gone[p.ID] = true
		m.mu.Unlock()
		if !pending {
			time.AfterFunc(recheck, func() { m.stopIfGone(p.ID) })
		}
	}
}

// stopIfGone stops the project's active deployments if it is still gone.
func (m *Manager) stopIfGone(project string) {
	ctx := context.Background()
	m.mu.Lock()
	delete(m.gone, project)
	closing := m.closing
	m.mu.Unlock()
	if closing {
		return
	}
	p, err := m.Config.Checked(project)
	if err != nil {
		return
	}
	reason := p.Gone()
	if reason == "" {
		return
	}
	deps, err := m.DeploymentsIn(ctx, store.DeploymentActive)
	if err != nil {
		slog.Warn("stopping a gone project's deployments", "project", project, "err", err)
		return
	}
	for _, d := range deps {
		if d.ProjectID != project {
			continue
		}
		if err := m.stop(ctx, d.ID, project+" is gone: "+reason); err != nil {
			slog.Warn("stopping a gone project's deployment", "deployment", d.ID, "err", err)
		}
	}
}

// heardFrom notes activity from an instance: its stuck card, if any, resolves.
func (m *Manager) heardFrom(ctx context.Context, id string) {
	m.mu.Lock()
	m.heard[id] = time.Now()
	m.mu.Unlock()
	m.unstuck(ctx, id, "active again")
}

// unstuck resolves the instance's stuck card, if it has one, with why.
func (m *Manager) unstuck(ctx context.Context, id, why string) {
	m.mu.Lock()
	r, ok := m.stuck[id]
	delete(m.stuck, id)
	m.mu.Unlock()
	if !ok {
		return
	}
	r.Status, r.Answer, r.ResolvedAt = store.RequestResolved, why, time.Now().UTC()
	if _, err := m.Requests.Save(ctx, r); err != nil {
		slog.Warn("resolving a stuck card", "instance", id, "err", err)
		return
	}
	m.publish("request.resolved", r)
}

// CheckStuck raises a card for each agent mid-turn (running) from which no
// hook has come for the stuck_after_minutes setting: hooks fire on every
// prompt, tool call and turn end, so a working agent is never that quiet. The
// agent keeps running, and the CEO decides (wiki/product/architecture/agent-runtime.md#stuck-agents).
func (m *Manager) CheckStuck(ctx context.Context, now time.Time) {
	if m.Requests == nil {
		return
	}
	limit := time.Duration(m.Config.Settings().Settings.StuckAfterMinutes) * time.Minute
	m.mu.Lock()
	var quiet []string
	for id, l := range m.live {
		if l.term == nil || m.stuck[id].ID != "" {
			continue
		}
		heard, ok := m.heard[id]
		if !ok {
			m.heard[id] = now
			continue
		}
		if now.Sub(heard) >= limit {
			quiet = append(quiet, id)
		}
	}
	m.mu.Unlock()
	for _, id := range quiet {
		inst, err := m.Instances.Get(ctx, id)
		if err != nil || inst.Status != Running {
			continue
		}
		dep, err := m.Deployments.Get(ctx, inst.DeploymentID)
		if err != nil {
			continue
		}
		r, err := m.Requests.Save(ctx, store.Request{Kind: "blocked", Source: "foreman",
			ProjectID: dep.ProjectID, DeploymentID: dep.ID, InstanceID: id, Options: []string{"Restart"},
			Summary: fmt.Sprintf("%s may be stuck: no activity for %d min.", m.agentName(dep, inst.AgentID), int(limit.Minutes()))})
		if err != nil {
			slog.Warn("raising a stuck card", "instance", id, "err", err)
			continue
		}
		m.mu.Lock()
		m.stuck[id] = r
		m.mu.Unlock()
		m.publish("request.created", r)
	}
}

// agentName is the instance's agent as its team names it, else its id.
func (m *Manager) agentName(dep store.Deployment, agent string) string {
	if team, err := m.Config.Team(dep.TeamID); err == nil {
		if a, ok := team.Config.Agents[agent]; ok && a.Name != "" {
			return a.Name
		}
	}
	return agent
}

// resolveOldStuck resolves the stuck cards a previous Foreman left open:
// Foreman's blocked cards about an agent.
func (m *Manager) resolveOldStuck(ctx context.Context) {
	if m.Requests == nil {
		return
	}
	open, err := m.Requests.Open(ctx)
	if err != nil {
		slog.Warn("listing stuck cards", "err", err)
		return
	}
	for _, r := range open {
		if r.Kind == "blocked" && r.Source == "foreman" && r.InstanceID != "" {
			m.mu.Lock()
			m.stuck[r.InstanceID] = r
			m.mu.Unlock()
			m.unstuck(ctx, r.InstanceID, "Foreman restarted")
		}
	}
}
