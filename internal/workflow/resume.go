package workflow

import (
	"context"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"strings"

	"github.com/brickpop/foreman/internal/git"
	"github.com/brickpop/foreman/internal/home"
	"github.com/brickpop/foreman/internal/store"
)

// A deployment resumes: its worktrees keep what earlier sessions left
// (wiki/product/architecture/sandboxes-and-git.md#branches). An agent starting
// fresh is told what that is, once, so it resumes it or moves on.

// Resumed tells an agent whose session starts fresh what its worktrees hold
// from before: its branches with work that hasn't landed (landed), the changes it hasn't
// committed, and for the lead the reviews still open. Nothing when there is
// nothing.
func (w *Workflow) Resumed(ctx context.Context, inst store.Instance, lead bool) {
	team, projectID, _ := strings.Cut(inst.DeploymentID, "@")
	project, err := w.Config.Project(projectID)
	if err != nil {
		return
	}
	prefix := git.BranchPrefix(projectID, inst.AgentID, team, lead)
	unmerged := map[string][]string{} // branch → its repos
	var uncommitted []string
	for _, r := range worktreesIn(project, inst.SandboxPath) {
		branches, err := w.Git.Branches(ctx, r.dir, prefix)
		if err != nil {
			slog.Info("what a worktree holds", "instance", inst.ID, "repo", r.name, "err", err)
			continue
		}
		for _, b := range branches {
			if !w.landed(ctx, project, r.name, r.dir, b) {
				unmerged[b] = append(unmerged[b], r.name)
			}
		}
		if clean, err := w.Git.Clean(ctx, r.dir); err == nil && !clean {
			on, _ := w.Git.Branch(ctx, r.dir)
			uncommitted = append(uncommitted, fmt.Sprintf("%s (on %s)", r.name, on))
		}
	}
	var open []string
	if lead {
		reviews, _ := w.projectReviews(ctx, projectID)
		for _, v := range reviews {
			switch {
			case v.DeploymentID != inst.DeploymentID:
			case v.Status == ReviewNeedsYou:
				open = append(open, v.Branch+" (waiting for the CEO)")
			case v.Status == ReviewChangesRequested:
				open = append(open, v.Branch+" (changes requested: fix them, then request_merge again)")
			}
		}
	}
	if len(unmerged) == 0 && len(uncommitted) == 0 && len(open) == 0 {
		return
	}
	text := "From Foreman: your worktrees hold earlier work."
	if len(unmerged) > 0 {
		var each []string
		for _, b := range slices.Sorted(maps.Keys(unmerged)) {
			each = append(each, fmt.Sprintf("%s (%s)", b, strings.Join(unmerged[b], ", ")))
		}
		text += " Branches with work not merged yet: " + strings.Join(each, ", ") + "."
	}
	if len(uncommitted) > 0 {
		text += " Changes not committed in " + strings.Join(uncommitted, ", ") + "."
	}
	if len(open) > 0 {
		text += " Your reviews still open: " + strings.Join(open, ", ") + "."
	}
	text += " Resume what is still wanted; new work goes on a new topic, made from the default branch with switch(<topic>), or from another branch with switch(<topic>, from)."
	if len(uncommitted) > 0 {
		text += " Changes you don't want: discard()."
	}
	if _, err := w.Messages.Send(ctx, store.TeamMessage{DeploymentID: inst.DeploymentID, FromInstance: "foreman",
		ToInstance: inst.ID, Kind: "notice", Body: text}); err != nil {
		slog.Warn("telling an agent what it left", "instance", inst.ID, "err", err)
	}
}

// landed says whether all of branch is in a branch of the CEO's or upstream
// (Merged: wiki/product/architecture/sandboxes-and-git.md#cleaning-up-branches).
// Unknown (a git error) counts as not.
func (w *Workflow) landed(ctx context.Context, project home.Project, name, dir, branch string) bool {
	state, _ := w.standing(ctx, project, name, dir, branch)
	return state == Merged
}
