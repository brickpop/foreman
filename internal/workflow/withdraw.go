package workflow

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strings"

	"github.com/brickpop/foreman/internal/agentlink"
	"github.com/brickpop/foreman/internal/events"

	"github.com/brickpop/foreman/internal/store"
	"github.com/brickpop/foreman/internal/teamtools"
)

// Withdrawal: a review taken back before the CEO decides it, by its lead
// (withdraw_merge), or by Foreman when the CEO merges another of the lead's
// reviews that holds all of it (wiki/product/features/reviews.md#withdrawn).

// withdrawMerge withdraws the caller's open review of branch, for reason;
// with by, it is superseded by the open review of that lead branch, which
// must hold all of it.
func (t tool) withdrawMerge(ctx context.Context, branch, reason, by string) agentlink.Result {
	if !t.c.Lead {
		return teamtools.Refusal("not_lead", "only the lead withdraws its requests to merge")
	}
	prefix := "lead/" + t.c.Team.ID + "/"
	for _, b := range []string{branch, by} {
		if b != "" && !strings.HasPrefix(b, prefix) {
			return teamtools.Refusal("branch_not_allowed", "you withdraw, and supersede with, your own lead branches only, %s<topic>", prefix)
		}
	}
	switch {
	case reason == "":
		return teamtools.Refusal("no_reason", "say why, for the CEO")
	case teamtools.TooLong(reason, teamtools.MaxBody):
		return teamtools.Refusal("too_long", "the reason is at most %d characters", teamtools.MaxBody)
	case by == branch:
		return teamtools.Refusal("superseded_by_itself", "superseded_by names another branch")
	}
	v, ok := t.ownReview(ctx, branch)
	if !ok {
		return teamtools.Refusal("not_open", "%s has no review open: nothing to withdraw", branch)
	}
	var superseding string
	if by != "" {
		sup, ok := t.ownReview(ctx, by)
		if !ok {
			return teamtools.Refusal("superseding_not_open", "%s has no review open: request_merge it first, or leave superseded_by out", by)
		}
		for _, mr := range v.MergeRequests {
			if mr.State != store.MergeOpen {
				continue
			}
			_, dir, err := t.w.repoOf(mr)
			if err != nil {
				return gitError(err)
			}
			if in, err := t.w.Git.Contains(ctx, dir, by, branch); err != nil || !in {
				return teamtools.Refusal("not_superseded", "%s doesn't hold all of %s in %s: merge it in first, or leave superseded_by out", by, branch, mr.Repo)
			}
		}
		superseding = sup.ID
	}
	repos := t.w.withdraw(ctx, v, reason, superseding, "lead", nil)
	text := fmt.Sprintf("Withdrew review %s of %s (%s): it left the CEO's queue. The branch stays.", v.ID, branch, strings.Join(repos, ", "))
	if superseding != "" {
		text += fmt.Sprintf(" The CEO sees it superseded by %s (%s).", superseding, by)
	}
	return agentlink.Result{Text: text + " Its hand-offs went to your open reviews that hold them; any other waits for the next request_merge of a branch that does."}
}

// ownReview is the caller's open review of branch.
func (t tool) ownReview(ctx context.Context, branch string) (Review, bool) {
	id, err := t.w.MergeRequests.OpenReview(ctx, t.project.ID, branch)
	if err != nil || id == "" {
		return Review{}, false
	}
	v, err := t.w.openReview(ctx, id)
	return v, err == nil && v.DeploymentID == t.c.Deployment.ID
}

// withdraw ends an open review as withdrawn, by the lead or Foreman, for
// why: its open merge requests end, the CEO's draft comments on it go, and
// its request resolves. The work items they carried go to the superseding
// review's merge request in their repo (to, by repo: merged already), or to
// the lead's open merge requests that hold them, else wait free for the
// next request_merge: their commits are still on the lead branch. It
// returns the repos withdrawn.
func (w *Workflow) withdraw(ctx context.Context, v Review, why, superseding, by string, to map[string]string) []string {
	ended := map[string]string{} // merge request id → its repo
	var repos []string
	for _, mr := range v.MergeRequests {
		if mr.State != store.MergeOpen {
			continue
		}
		w.settle(ctx, &mr, "")
		mr.State, mr.Note = store.MergeWithdrawn, why
		w.saveMerge(ctx, mr)
		ended[mr.ID], repos = mr.Repo, append(repos, mr.Repo)
	}
	r := v.Review
	r.WithdrawnAt, r.WithdrawnReason, r.SupersededBy = w.Now().UTC(), why, superseding
	if _, err := w.Reviews.Save(ctx, r); err != nil {
		slog.Warn("keeping a review", "review", r.ID, "err", err)
	}
	lost := w.dropDrafts(ctx, r.ID)
	w.resolveReview(ctx, r.ID, "withdrawn")
	items, err := w.WorkItems.ForDeployment(ctx, v.DeploymentID)
	if err != nil {
		slog.Warn("work items", "deployment", v.DeploymentID, "err", err)
	}
	for _, item := range items {
		if repo, ok := ended[item.MergeRequestID]; ok {
			item.MergeRequestID = to[repo]
			w.save(ctx, item)
		}
	}
	if open, err := w.MergeRequests.Open(ctx); err == nil {
		for _, mr := range open {
			if mr.DeploymentID != v.DeploymentID || !slices.Contains(repos, mr.Repo) {
				continue
			}
			if _, dir, err := w.repoOf(mr); err == nil {
				w.carry(ctx, dir, mr)
			}
		}
	}
	if w.Publish != nil {
		w.Publish(events.Event{Type: "review.withdrawn", Data: Withdrawn{Review: r, DeploymentID: v.DeploymentID, Lead: v.Lead, By: by, LostDrafts: lost}})
	}
	return repos
}

// dropDrafts deletes a review's draft comments: they can't be sent once it
// is withdrawn (the CEO, 2026-10-22: warn that they're lost). It returns how
// many went.
func (w *Workflow) dropDrafts(ctx context.Context, reviewID string) int {
	comments, err := w.ReviewComments.ForReview(ctx, reviewID)
	if err != nil {
		slog.Warn("a withdrawn review's comments", "review", reviewID, "err", err)
		return 0
	}
	lost := 0
	for _, c := range comments {
		if c.State != store.CommentDraft {
			continue
		}
		if err := w.ReviewComments.Delete(ctx, c.ID); err != nil {
			slog.Warn("dropping a draft comment", "comment", c.ID, "err", err)
			continue
		}
		lost++
	}
	return lost
}

// supersede withdraws the other open reviews of a merged merge request's
// lead whose every open merge request main now holds (git.Merged): what the
// CEO merged holds all of them (the CEO, 2026-10-22: merge one, the others
// it contains go). One main lacks something of stays open.
func (w *Workflow) supersede(ctx context.Context, merged store.MergeRequest) {
	open, err := w.MergeRequests.Open(ctx)
	if err != nil {
		return
	}
	var ids []string
	for _, mr := range open {
		if mr.DeploymentID == merged.DeploymentID && mr.Repo == merged.Repo && mr.ReviewID != merged.ReviewID && !slices.Contains(ids, mr.ReviewID) {
			ids = append(ids, mr.ReviewID)
		}
	}
	if len(ids) == 0 {
		return
	}
	to := map[string]string{}
	if mrs, err := w.MergeRequests.ForReview(ctx, merged.ReviewID); err == nil {
		for _, mr := range mrs {
			if mr.State == store.MergeMerged {
				to[mr.Repo] = mr.ID
			}
		}
	}
	for _, id := range ids {
		v, err := w.openReview(ctx, id)
		if err != nil || !w.heldByMain(ctx, v) {
			continue
		}
		lead := v.MergeRequests[len(v.MergeRequests)-1]
		why := fmt.Sprintf("%s, merged, holds all of it", merged.Branch)
		repos := w.withdraw(ctx, v, why, merged.ReviewID, "foreman", to)
		w.tell(ctx, lead, "foreman",
			fmt.Sprintf("From Foreman: the CEO merged %s, which holds all of %s, so Foreman withdrew the review of %s (%s, %s). Don't request it again.", merged.Branch, v.Branch, v.Branch, v.ID, strings.Join(repos, ", ")))
	}
}

// heldByMain says whether main holds every open merge request of a review:
// merging any of them would change nothing.
func (w *Workflow) heldByMain(ctx context.Context, v Review) bool {
	for _, mr := range v.MergeRequests {
		if mr.State != store.MergeOpen {
			continue
		}
		main, dir, err := w.repoOf(mr)
		if err != nil {
			return false
		}
		if held, err := w.Git.Merged(ctx, dir, main, mr.Branch); err != nil || !held {
			return false
		}
	}
	return true
}
