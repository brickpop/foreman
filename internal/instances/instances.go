// Package instances decides which instances run: one per member of each
// active deployment (wiki/product/architecture/building-blocks.md#5-instance-manager-instances).
// It prepares each one's sandbox, launches its harness in the engine, keeps
// its terminal, restarts it once after a crash, queues beyond the concurrency
// limit, puts idle agents to sleep to make room, and re-attaches after a
// Foreman restart (wiki/product/architecture/agent-runtime.md).
//
// Only the lead starts on deploy; the other members sleep until they are
// woken (wiki/product/architecture/agent-runtime.md#lazy-starts).
//
// The files, by concern:
//
//	instances.go  the manager, its dependencies and what it answers
//	lifecycle.go  the CEO's actions: deploy, wake, restart, stop; Foreman exiting
//	run.go        one run of an instance: its start, its end, re-attaching one
//	restore.go    picking up after a Foreman restart
//	launch.go     what a run is started with: worktrees, mounts, prompt, harness
//	status.go     the one way an instance is changed in the store
//	queue.go      the session slots and their queue
//	room.go       idle agents giving their slot up to one that waits
//	hooks.go      what the harness says it is doing
//	stuck.go      agents quiet mid-turn
//	gone.go       deployments whose project is gone
package instances

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"sync"
	"time"

	"github.com/brickpop/foreman/internal/events"
	"github.com/brickpop/foreman/internal/harness"
	"github.com/brickpop/foreman/internal/home"
	"github.com/brickpop/foreman/internal/profiles"
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
	// Publish hears every event. A status change is published while the
	// change still holds its lock, so they arrive in order: it must not call
	// the manager back.
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
	// Started is told of each session that starts fresh (not continuing its
	// conversation), once its process runs: Workflow tells the agent what its
	// worktrees hold from before. nil: nobody.
	Started func(ctx context.Context, inst store.Instance, lead bool)
	// Drafting reports whether the CEO has typed into an instance's prompt
	// without sending it: it keeps the instance awake (MakeRoom). nil: never.
	Drafting func(id string) bool
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
	work     sync.WaitGroup // starts and room-making under way
	changing sync.Mutex     // one instance change at a time (change)

	mu         sync.Mutex
	runs       map[string]*run // instances with a process or a start under way
	queue      []string        // instances waiting for a slot, in creation order
	closing    bool            // Foreman is exiting: nothing starts any more
	makingRoom bool            // a MakeRoom is under way
	gone       map[string]bool // projects found gone, waiting for their recheck
}

// New returns a manager.
func New(d Deps) *Manager {
	return &Manager{Deps: d, runs: map[string]*run{}, gone: map[string]bool{}}
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

func (m *Manager) publish(typ string, data any) {
	if m.Publish != nil {
		m.Publish(events.Event{Type: typ, Data: data})
	}
}

// Live reports whether the instance has a process or a start under way: the
// agent link accepts its token only then.
func (m *Manager) Live(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.runs[id] != nil
}

// Terminal returns a running instance's terminal.
func (m *Manager) Terminal(id string) (*terminal.Terminal, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if r := m.runs[id]; r != nil && r.term != nil {
		return r.term, nil
	}
	return nil, ErrNotRunning
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

// List returns the instances of every active deployment, by deployment, each
// with what its run is doing.
func (m *Manager) List(ctx context.Context) ([]store.Instance, error) {
	deps, err := m.DeploymentsIn(ctx, store.DeploymentActive)
	if err != nil {
		return nil, err
	}
	list := []store.Instance{}
	for _, dep := range deps {
		insts, err := m.Instances.List(ctx, dep.ID)
		if err != nil {
			return nil, err
		}
		list = append(list, insts...)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range list {
		if r := m.runs[list[i].ID]; r != nil && r.doing != nil {
			doing := *r.doing
			list[i].Doing = &doing
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

// Active reports whether a deployment still runs (it isn't stopped).
func (m *Manager) Active(ctx context.Context, deploymentID string) (bool, error) {
	dep, err := m.Deployments.Get(ctx, deploymentID)
	if errors.Is(err, store.ErrNotFound) {
		return false, nil
	}
	return dep.Status == store.DeploymentActive, err
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

// Wait waits for the starts and room-making under way to finish (at
// shutdown, and in tests: a start may be writing worktrees).
func (m *Manager) Wait() { m.work.Wait() }
