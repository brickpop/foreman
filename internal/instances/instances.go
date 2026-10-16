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
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/brickpop/foreman/internal/events"
	"github.com/brickpop/foreman/internal/harness"
	"github.com/brickpop/foreman/internal/prompt"
	"github.com/brickpop/foreman/internal/runner"
	"github.com/brickpop/foreman/internal/store"
	"github.com/brickpop/foreman/internal/terminal"
	"github.com/brickpop/foreman/internal/workspace"
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

// Git is what instances need of the Git block: a worktree per code repo.
type Git interface {
	Prepare(ctx context.Context, repo, worktree, branch, base string) error
	Exclude(ctx context.Context, path string) error
}

// Config is the workspace's config, as loaded.
type Config interface {
	Project(id string) (workspace.Project, error)
	Team(id string) (workspace.Team, error)
	Settings() workspace.SettingsFile
}

// Deps are the blocks the manager is assembled from.
type Deps struct {
	Root        string // the workspace folder: projects are its subfolders
	Runner      runner.Runner
	Images      runner.Images
	Git         Git
	Config      Config
	Deployments store.Deployments
	Instances   store.Instances
	Adapters    map[string]harness.Adapter // by harness name (claude-code, …)
	OpenLog     func(instance string) (terminal.Log, error)
	Publish     func(events.Event)
	Agent       AgentLink
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
	mu     sync.Mutex
	starts sync.WaitGroup   // starts under way
	live   map[string]*live // instances with a process or a start under way
	queue  []string         // queued instances, in creation order
}

type live struct {
	term     *terminal.Terminal // nil while starting
	proc     runner.Process
	stopping bool // a stop was asked: its end is not a crash
}

// New returns a manager.
func New(d Deps) *Manager { return &Manager{Deps: d, live: map[string]*live{}} }

var (
	// ErrNotRunning means the instance has no terminal now.
	ErrNotRunning = errors.New("instances: not running")
	// ErrInvalid means the team or project can't be deployed as it is.
	ErrInvalid = errors.New("instances: can't deploy")
)

// stopGrace is how long a harness gets to end before it is killed.
const stopGrace = 10 * time.Second

// Deploy deploys team on project with goal and starts its lead; the other
// members sleep until woken.
func (m *Manager) Deploy(ctx context.Context, teamID, projectID, goal string) (store.Deployment, error) {
	team, project, err := m.deployable(teamID, projectID)
	if err != nil {
		return store.Deployment{}, err
	}
	dep, err := m.Deployments.Deploy(ctx, teamID, projectID, goal)
	if err != nil {
		return store.Deployment{}, err
	}
	m.publish("deployment.created", dep)
	for _, agent := range team.Config.Roster() {
		soul := team.Config.Agents[agent]
		inst := store.Instance{
			ID: agent + "@" + projectID, DeploymentID: dep.ID, AgentID: agent, Status: Asleep,
			SandboxPath: filepath.Join(m.Root, project.ID, ".sandboxes", agent), Harness: soul.Harness,
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

// Wake starts an asleep instance: the CEO's Start (later, work arriving for
// it). An instance that is up or on its way is left as it is.
func (m *Manager) Wake(ctx context.Context, id string) error {
	inst, err := m.Instances.Get(ctx, id)
	if err != nil {
		return err
	}
	if inst.Status != Asleep {
		return nil
	}
	m.enqueue(id)
	m.startQueued()
	return nil
}

// deployable checks what deploying needs: a valid team and project, and an
// adapter for every member's harness.
func (m *Manager) deployable(teamID, projectID string) (workspace.Team, workspace.Project, error) {
	team, err := m.Config.Team(teamID)
	if err != nil {
		return team, workspace.Project{}, fmt.Errorf("%w: team %s: %v", ErrInvalid, teamID, err)
	}
	project, err := m.Config.Project(projectID)
	if err != nil {
		return team, project, fmt.Errorf("%w: project %s: %v", ErrInvalid, projectID, err)
	}
	if len(team.Problems) > 0 {
		return team, project, fmt.Errorf("%w: team %s has problems: %v", ErrInvalid, teamID, team.Problems)
	}
	if len(project.Problems) > 0 {
		return team, project, fmt.Errorf("%w: project %s has problems: %v", ErrInvalid, projectID, project.Problems)
	}
	if len(project.Repos) == 0 {
		return team, project, fmt.Errorf("%w: project %s has no code repo for agents to work in: create one in its Settings (New repo), or clone one into its folder", ErrInvalid, projectID)
	}
	for _, agent := range team.Config.Roster() {
		h := team.Config.Agents[agent].Harness
		adapter := m.Adapters[h]
		if adapter == nil {
			return team, project, fmt.Errorf("%w: %s's harness %q isn't supported yet", ErrInvalid, agent, h)
		}
		// Refused here, not when the agent starts, where it would only reach the log.
		if !adapter.Connected() {
			return team, project, fmt.Errorf("%w: %s isn't connected yet: log in under Settings → Harnesses → Connect", ErrInvalid, h)
		}
	}
	return team, project, nil
}

// Stop stops a deployment's instances and marks it stopped. Their worktrees
// stay on disk.
func (m *Manager) Stop(ctx context.Context, deploymentID string) error {
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
	if err := m.Deployments.Stop(ctx, deploymentID); err != nil {
		return err
	}
	dep.Status = store.DeploymentStopped
	m.publish("deployment.stopped", dep)
	m.startQueued() // its slots go to whoever waits
	return nil
}

// Live reports whether the instance has a process or a start under way: the
// agent link accepts its token only then.
func (m *Manager) Live(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.live[id] != nil
}

// Hook takes one of an instance's harness hook events: what it says the
// harness is doing becomes the instance's status (running, idle, awaiting).
// Before the instance runs, or once it stops, the event changes nothing.
func (m *Manager) Hook(id, event string, payload []byte) {
	ctx := context.Background()
	inst, err := m.Instances.Get(ctx, id)
	if err != nil {
		return
	}
	adapter := m.Adapters[inst.Harness]
	if adapter == nil {
		return
	}
	status, ok := map[harness.Activity]string{harness.Busy: Running, harness.Idle: Idle, harness.Waiting: Awaiting}[adapter.Interpret(event, payload)]
	if !ok || (inst.Status != Running && inst.Status != Idle && inst.Status != Awaiting) {
		return
	}
	m.setStatus(ctx, id, status)
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

// Restore picks up after a Foreman restart: running containers are
// re-attached, live instances whose process is gone restart (continuing their
// conversation), and processes that belong to nothing are stopped.
func (m *Manager) Restore(ctx context.Context) error {
	procs, err := m.Runner.List(ctx)
	if err != nil {
		return err
	}
	byID := map[string]runner.Process{}
	for _, p := range procs {
		byID[p.Labels()["foreman.instance"]] = p
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
	m.live[id] = l
	m.mu.Unlock()
	// Its process lived on: what its hooks last said still holds (idle stays
	// idle, so a message queued for it is delivered).
	if inst, err := m.Instances.Get(ctx, id); err != nil || (inst.Status != Idle && inst.Status != Awaiting) {
		m.setStatus(ctx, id, Running)
	}
	go m.watch(id, l)
	return nil
}

func (m *Manager) enqueue(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.live[id] == nil && !contains(m.queue, id) {
		m.queue = append(m.queue, id)
	}
	ctx := context.Background()
	if inst, err := m.Instances.Get(ctx, id); err == nil && inst.Status != Queued {
		inst.Status = Queued
		_ = m.Instances.Put(ctx, inst)
		m.publish("instance.status", StatusChange{ID: id, Status: Queued})
	}
}

// startQueued starts queued instances while there are free slots
// (max_sessions; a slot is held by every instance with a process or a start).
func (m *Manager) startQueued() {
	max := m.Config.Settings().Settings.MaxSessions
	for {
		m.mu.Lock()
		if len(m.queue) == 0 || len(m.live) >= max {
			m.mu.Unlock()
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

// start prepares the instance's sandbox, launches its harness and keeps its
// terminal. A failure stops the instance (with its reason in the log).
func (m *Manager) start(id string, l *live) {
	ctx := context.Background()
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
			m.setStatus(ctx, id, Stopped)
		}
		m.startQueued()
		return
	}
	inst, _ := m.Instances.Get(ctx, id)
	inst.Status, inst.StartedAt = Running, time.Now()
	_ = m.Instances.Put(ctx, inst)
	m.publish("instance.status", StatusChange{ID: id, Status: Running})
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
	team, project, err := m.deployable(dep.TeamID, dep.ProjectID)
	if err != nil {
		return nil, err
	}
	adapter := m.Adapters[team.Config.Agents[inst.AgentID].Harness]
	lead := inst.AgentID == team.Config.Lead

	projectDir := filepath.Join(m.Root, project.ID)
	if err := os.MkdirAll(inst.SandboxPath, 0o755); err != nil {
		return nil, err
	}
	if err := m.Git.Exclude(ctx, filepath.Join(projectDir, ".sandboxes")); err != nil {
		return nil, err
	}
	branch := "wt/" + inst.AgentID + "/main"
	if lead {
		branch = "lead/" + team.ID + "/integration"
	}
	var mounts []runner.Mount
	mounts = append(mounts, runner.Mount{Source: inst.SandboxPath, Target: inst.SandboxPath})
	for _, repo := range project.Repos {
		repoDir := filepath.Join(projectDir, repo)
		if err := m.Git.Prepare(ctx, repoDir, filepath.Join(inst.SandboxPath, repo), branch, project.Config.Main); err != nil {
			return nil, err
		}
		// Agents read git; only Foreman writes it.
		mounts = append(mounts, runner.Mount{Source: filepath.Join(repoDir, ".git"), Target: filepath.Join(repoDir, ".git"), ReadOnly: true})
	}
	// The live project repo, writable for the lead only; its .git read-only for all.
	projectRepo, inSandbox := filepath.Join(projectDir, "project"), filepath.Join(inst.SandboxPath, "project")
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
		Instance: id, Sandbox: inst.SandboxPath, Prompt: text,
		// A restart (after a crash, or a Foreman restart) picks up its conversation;
		// a new deployment starts fresh.
		Continue: !first,
	}
	if m.Agent.Host != "" {
		mounts = append(mounts, runner.Mount{Source: m.Agent.Host, Target: m.Agent.Path, ReadOnly: true})
		session.Link = harness.Link{Program: m.Agent.Path, Env: []string{
			"FOREMAN_AGENT_URL=" + m.Agent.URL, "FOREMAN_AGENT_TOKEN=" + m.Agent.Token(id),
		}}
	}
	if lead && first && dep.Goal != "" {
		// The kickoff (journey G1): the lead starts on the goal at once. Only on
		// its first start, so a restart doesn't give the goal twice.
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
		Name: id, Image: image, Command: l.Command, Dir: inst.SandboxPath, Env: l.Env,
		Mounts: append(mounts, l.Mounts...),
		Labels: map[string]string{"foreman.instance": id, "foreman.deployment": dep.ID},
	})
}

// watch waits for the instance's process to end: a stop asked for, or a crash
// (restarted once, continuing its conversation; a second crash stops it).
func (m *Manager) watch(id string, l *live) {
	<-l.term.Done()
	ctx := context.Background()
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
	case inst.Restarts == 0:
		slog.Warn("instance exited; restarting it once", "instance", id, "code", code)
		inst.Restarts++
		_ = m.Instances.Put(ctx, inst)
		m.enqueue(id)
	default:
		slog.Warn("instance exited again; stopped", "instance", id, "code", code)
		m.setStatus(ctx, id, Stopped)
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
	inst.Status = status
	if err := m.Instances.Put(ctx, inst); err != nil {
		slog.Warn("saving an instance", "instance", id, "err", err)
	}
	m.publish("instance.status", StatusChange{ID: id, Status: status})
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
	return list, nil
}

// ActiveDeployments returns the active deployments.
func (m *Manager) ActiveDeployments(ctx context.Context) ([]store.Deployment, error) {
	all, err := m.Deployments.List(ctx)
	if err != nil {
		return nil, err
	}
	active := []store.Deployment{}
	for _, d := range all {
		if d.Status == store.DeploymentActive {
			active = append(active, d)
		}
	}
	return active, nil
}

// compose is an instance's session prompt: launch writes it, Prompt shows it.
func (m *Manager) compose(inst store.Instance, dep store.Deployment, team workspace.Team, project workspace.Project) string {
	return prompt.Compose(prompt.Input{
		Agent: inst.AgentID, Team: team.Config, Project: project.ID, Path: filepath.Join(m.Root, project.ID),
		Config: project.Config, Goal: dep.Goal,
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
