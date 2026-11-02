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
// (wiki/product/features/project-deployment.md#earlier-teams): only what is
// safe. A worktree goes when it holds nothing not committed (git refuses
// otherwise) and no merge is under way; a wt/<agent>/* branch goes when a
// target of its repo holds all of it and nothing has it checked out, its head
// kept under refs/foreman/cleaned/ for a while. An agent running on any
// project is left alone, as a repo may be in several. Everything else is
// kept, and listed with why.
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
			running[i.AgentID] = true
		}
	}
	var out Cleanup
	keep := func(c Cleaned, why string) { c.Why = why; out.Kept = append(out.Kept, c) }
	for _, r := range p.Repos {
		trees, err := w.Git.Worktrees(ctx, r.Path)
		if err != nil {
			return out, err
		}
		agents := map[string]bool{}
		for branch, dir := range trees {
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
			branches, err := w.Git.Branches(ctx, r.Path, "wt/"+agent+"/")
			if err != nil {
				return out, err
			}
			for _, b := range branches {
				c := Cleaned{Agent: agent, Repo: r.Name, What: "branch", Branch: b}
				if _, checkedOut := trees[b]; checkedOut {
					keep(c, "it is checked out")
					continue
				}
				if !w.heldByATarget(ctx, p, r, b) {
					keep(c, "no target holds all of it")
					continue
				}
				if err := w.Git.DropBranch(ctx, r.Path, b, git.BackupRef("cleaned", w.Now(), b), r.Default); err != nil {
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

func (w *Workflow) heldByATarget(ctx context.Context, p home.Project, r home.Repo, branch string) bool {
	for _, t := range p.Targets(r.Name) {
		if held, err := w.Git.Merged(ctx, r.Path, t, branch); err == nil && held {
			return true
		}
	}
	return false
}
