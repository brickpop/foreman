package workflow

import (
	"context"
	"errors"
	"maps"
	"slices"
	"strings"

	"github.com/brickpop/foreman/internal/git"
	"github.com/brickpop/foreman/internal/home"
)

// Cleanup is what CleanUp did on a project: what went, and what stayed with
// why, so the CEO sees what is left to do by hand.
type Cleanup struct {
	Removed []Cleaned `json:"removed"`
	Kept    []Cleaned `json:"kept"`
}

// Cleaned is one worktree or branch of an agent, in one code repo.
type Cleaned struct {
	Agent  string `json:"agent"`
	Repo   string `json:"repo"`
	What   string `json:"what"`   // "worktree" or "branch"
	Branch string `json:"branch"` // the branch, or the worktree's
	Why    string `json:"why,omitempty"`
}

// CleanUp removes, on a project, what the agents of earlier teams left
// (wiki/product/features/project-team.md#earlier-teams): only what is
// safe. A worktree goes when it holds nothing not committed (git refuses
// otherwise) and no merge is under way; a wt/<project>/<agent>/* branch goes
// when all of it has landed (landed) and nothing has it checked out,
// its head kept under refs/foreman/cleaned/ for a while. An agent running on
// this project is left alone; one running on another is not, as its branches
// there carry that project. Everything else is kept, and listed with why.
func (w *Workflow) CleanUp(ctx context.Context, projectID string) (Cleanup, error) {
	p, err := w.Config.Project(projectID)
	if err != nil {
		return Cleanup{}, err
	}
	running := map[string]bool{}
	if w.Teams != nil {
		list, err := w.Teams.List(ctx)
		if err != nil {
			return Cleanup{}, err
		}
		for _, i := range list {
			if strings.HasSuffix(i.DeploymentID, "@"+p.ID) {
				running[i.AgentID] = true
			}
		}
	}
	var out Cleanup
	keep := func(c Cleaned, why string) { c.Why = why; out.Kept = append(out.Kept, c) }
	for _, r := range p.Repos {
		trees, err := w.Git.Worktrees(ctx, r.Path)
		if err != nil {
			return out, err
		}
		// Those on a branch, by branch, and the idle ones, on none ("").
		type tree struct{ branch, dir string }
		var all []tree
		for branch, dir := range trees {
			all = append(all, tree{branch, dir})
		}
		idle, err := w.Git.Detached(ctx, r.Path)
		if err != nil {
			return out, err
		}
		for _, dir := range idle {
			all = append(all, tree{"", dir})
		}
		agents := map[string]bool{}
		for _, t := range all {
			branch, dir := t.branch, t.dir
			agent := home.WorktreeAgent(dir, func(agent string) string { return w.Config.Sandbox(p.ID, agent) })
			if agent == "" || running[agent] {
				continue
			}
			agents[agent] = true
			c := Cleaned{Agent: agent, Repo: r.Name, What: "worktree", Branch: branch}
			if merging, err := w.Git.Merging(ctx, dir); err != nil {
				keep(c, err.Error())
				continue
			} else if merging {
				keep(c, "a merge is under way in it")
				continue
			}
			switch err := w.Git.RemoveWorktree(ctx, r.Path, dir); {
			case errors.Is(err, git.ErrUncommitted):
				keep(c, "it holds changes not committed")
			case err != nil:
				keep(c, err.Error())
			default:
				out.Removed = append(out.Removed, c)
				delete(trees, branch)
			}
		}
		for _, agent := range slices.Sorted(maps.Keys(agents)) {
			branches, err := w.Git.Branches(ctx, r.Path, git.BranchPrefix(p.ID, agent, "", false))
			if err != nil {
				return out, err
			}
			for _, b := range branches {
				c := Cleaned{Agent: agent, Repo: r.Name, What: "branch", Branch: b}
				if _, checkedOut := trees[b]; checkedOut {
					keep(c, "it is checked out")
					continue
				}
				if !w.landed(ctx, p, r.Name, r.Path, b) {
					keep(c, "not all of it is merged")
					continue
				}
				if err := w.Git.DeleteBranch(ctx, r.Path, b, git.BackupRef("cleaned", w.Now(), b)); errors.Is(err, git.ErrCheckedOut) {
					keep(c, "it is checked out")
					continue
				} else if err != nil {
					keep(c, err.Error())
					continue
				}
				out.Removed = append(out.Removed, c)
			}
		}
		w.pruneBackups(ctx, r.Path)
	}
	// In a steady order: the worktrees come from a map.
	for _, list := range [][]Cleaned{out.Removed, out.Kept} {
		slices.SortFunc(list, func(a, b Cleaned) int {
			return strings.Compare(a.Agent+" "+a.Repo+" "+a.What+" "+a.Branch, b.Agent+" "+b.Repo+" "+b.What+" "+b.Branch)
		})
	}
	return out, nil
}
