package instances

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/brickpop/foreman/internal/git"
	"github.com/brickpop/foreman/internal/harness"
	"github.com/brickpop/foreman/internal/home"
	"github.com/brickpop/foreman/internal/prompt"
	"github.com/brickpop/foreman/internal/runner"
	"github.com/brickpop/foreman/internal/store"
)

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
	if project.Archived {
		return team, project, on, fmt.Errorf("%w: project %s is archived: unarchive it first (its Project settings)", ErrInvalid, projectID)
	}
	if len(project.Problems) > 0 {
		return team, project, on, fmt.Errorf("%w: project %s has problems: %v", ErrInvalid, projectID, project.Problems)
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
	// fresh.
	switched := inst.Harness != on.harness
	if switched || inst.Profile != on.profile {
		inst.Harness, inst.Profile = on.harness, on.profile
		err := m.change(ctx, id, func(i *store.Instance) bool {
			i.Harness, i.Profile = on.harness, on.profile
			return true
		})
		if err != nil {
			return nil, err
		}
	}
	lead := inst.AgentID == team.Config.Lead

	if err := os.MkdirAll(inst.SandboxPath, 0o755); err != nil {
		return nil, err
	}
	branch := git.HomeBranch(project.ID, inst.AgentID, team.ID, lead)
	var mounts []runner.Mount
	mounts = append(mounts, runner.Mount{Source: inst.SandboxPath, Target: inst.SandboxPath})
	for _, repo := range project.Repos {
		repoDir, worktree := repo.Path, filepath.Join(inst.SandboxPath, repo.Name)
		if err := m.Git.Prepare(ctx, repoDir, worktree, branch, repo.Default); err != nil {
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
	// The live project repo, writable for the lead only; its .git read-only for
	// all. At its real path, as everything else: a path an agent names is one
	// the CEO can open (the CEO, 2026-11-02).
	// Its old mount point in the sandbox, from before: gone when empty (temporary,
	// wiki/technical-debt.md#the-old-project-mount-point).
	_ = os.Remove(filepath.Join(inst.SandboxPath, "project"))
	mounts = append(mounts, runner.Mount{Source: project.Path, Target: project.Path, ReadOnly: !lead})
	if _, err := os.Stat(filepath.Join(project.Path, ".git")); err == nil {
		mounts = append(mounts, runner.Mount{Source: filepath.Join(project.Path, ".git"), Target: filepath.Join(project.Path, ".git"), ReadOnly: true})
	}

	// The harness's own files, private, outside the sandbox.
	dir := project.HarnessDir(inst.AgentID)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	mounts = append(mounts, runner.Mount{Source: dir, Target: dir})

	text := m.compose(inst, dep, team, project)
	if hash := promptHash(text); hash != inst.PromptHash {
		err := m.change(ctx, id, func(i *store.Instance) bool {
			i.PromptHash = hash
			return true
		})
		if err != nil {
			return nil, err
		}
	}
	session := harness.Session{
		Instance: id, Sandbox: inst.SandboxPath, Dir: dir, Prompt: text, Login: on.login,
		// Every start picks up the agent's last conversation on the project,
		// a redeploy's first too, unless deploy said fresh (Start fresh, another
		// team or harness last) or the harness changed since.
		Continue: !inst.Fresh && !switched,
	}
	if m.Agent.Host != "" {
		mounts = append(mounts, runner.Mount{Source: m.Agent.Host, Target: m.Agent.Path, ReadOnly: true})
		session.Link = harness.Link{Program: m.Agent.Path, Env: []string{
			"FOREMAN_AGENT_URL=" + m.Agent.URL, "FOREMAN_AGENT_TOKEN=" + m.Agent.Token(id),
		}}
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
	proc, err := m.Runner.Start(ctx, runner.Spec{
		Name: m.processName(id), Image: image, Command: l.Command, Dir: inst.SandboxPath, Env: l.Env,
		Mounts: append(mounts, l.Mounts...),
		Labels: map[string]string{"foreman.instance": id, "foreman.deployment": dep.ID, HomeLabel: m.Home},
	})
	// A session continuing its conversation knows already.
	if err == nil && !session.Continue && m.Started != nil {
		m.Started(ctx, inst, lead)
	}
	return proc, err
}

// compose is an instance's session prompt: launch writes it, Prompt shows it.
func (m *Manager) compose(inst store.Instance, dep store.Deployment, team home.Team, project home.Project) string {
	return prompt.Compose(prompt.Input{
		Agent: inst.AgentID, Team: team.Config, Project: project.ID, Sandbox: inst.SandboxPath, ProjectRepo: project.Path, Config: project.Config, Repos: project.Repos,
		Tools:        m.tools(inst.AgentID == team.Config.Lead, len(project.Repos) > 0),
		Instructions: m.Config.Settings().Settings.Instructions,
	})
}

// promptHash tells one composed prompt from another, so a run's can be
// compared with what its instance would get now (behind).
func promptHash(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

// behind marks the live instances whose run started on an older prompt than
// the one composed now: a team, project or settings change since, which
// reaches them at their next start, or at once on the CEO's Reload
// (wiki/product/models/team-definition.md#reload). A run with no hash (from
// before it was kept) counts: nothing says it is on the current prompt.
func (m *Manager) behind(dep store.Deployment, insts []store.Instance) {
	team, err := m.Config.Team(dep.TeamID)
	if err != nil {
		return
	}
	project, err := m.Config.Project(dep.ProjectID)
	if err != nil {
		return
	}
	for i, inst := range insts {
		switch inst.Status {
		case Running, Idle, Awaiting:
			insts[i].Behind = promptHash(m.compose(inst, dep, team, project)) != inst.PromptHash
		}
	}
}

// tools are the team tools an instance gets: none without foreman-agent; the
// git ones only while its project has a code repo.
func (m *Manager) tools(lead, code bool) []string {
	if m.Agent.Host == "" {
		return nil
	}
	return prompt.Offered(lead, code)
}
