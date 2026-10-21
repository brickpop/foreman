package workflow

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/brickpop/foreman/internal/agentlink"
	"github.com/brickpop/foreman/internal/events"
	"github.com/brickpop/foreman/internal/git"
	"github.com/brickpop/foreman/internal/home"
	"github.com/brickpop/foreman/internal/store"
	"github.com/brickpop/foreman/internal/teamtools"
)

// Merge requests: the lead's request_merge, one per repo, grouped in the
// branch's review, where the CEO decides (reviews.go;
// wiki/product/features/pipeline-and-merges.md#review-and-merge,
// wiki/product/models/merge-request.md).

var (
	// ErrNotOpen means the review has no open merge request left (they were
	// merged or closed already), or none in the repos named.
	ErrNotOpen = errors.New("workflow: nothing open to decide")
	// ErrConflict means the branch doesn't merge cleanly into main: the
	// merge request is marked so and the lead told to merge main in.
	ErrConflict = git.ErrConflict
	// ErrLocalChanges means the CEO's checkout has main checked out with
	// uncommitted changes the merge would overwrite.
	ErrLocalChanges = git.ErrLocalChanges
	// ErrBranchMoved means the branch isn't at the commit the CEO read: the
	// lead committed since, and the CEO hasn't seen it.
	ErrBranchMoved = git.ErrMoved
)

func (t tool) requestMerge(ctx context.Context, branch, entry string) agentlink.Result {
	if !t.c.Lead {
		return teamtools.Refusal("not_lead", "only the lead requests merges to main: hand your work to the lead")
	}
	prefix := "lead/" + t.c.Team.ID + "/"
	if !strings.HasPrefix(branch, prefix) || !git.ValidBranch(ctx, branch) {
		return teamtools.Refusal("branch_not_allowed", "you request merges of your own lead branches only, %s<topic>", prefix)
	}
	if entry == "" {
		return teamtools.Refusal("entry_not_found", "name the wiki entry that explains the work, usually its task card")
	}
	if _, err := t.w.Wiki.Entry(t.project.ID, entry); err != nil {
		return teamtools.Refusal("entry_not_found", "no wiki entry %q: give its path in the wiki, e.g. backlog/export-csv.md", entry)
	}
	if res, ok := t.unsettled(ctx, t.worktrees(), "a merge request is your committed work"); !ok {
		return res
	}
	t.integrate(ctx) // hand-offs merged since the last check go with it
	// The branch's open review takes every repo's merge request; with none
	// open, the first one made starts a new review.
	reviewID, err := t.w.MergeRequests.OpenReview(ctx, t.project.ID, branch)
	if err != nil {
		return teamtools.Refusal("internal", "%v", err)
	}
	var made []string
	found := false
	for _, r := range t.worktrees() {
		if !t.w.Git.HasBranch(ctx, r.dir, branch) {
			continue
		}
		found = true
		if merged, err := t.w.Git.Merged(ctx, r.dir, t.project.Main(r.name), branch); err != nil {
			return gitError(err)
		} else if merged {
			continue
		}
		if reviewID == "" {
			review, err := t.w.Reviews.Save(ctx, store.Review{ProjectID: t.project.ID, Branch: branch})
			if err != nil {
				return teamtools.Refusal("internal", "%v", err)
			}
			reviewID = review.ID
		}
		mr, err := t.w.MergeRequests.OpenFor(ctx, t.project.ID, r.name, branch)
		refreshed := err == nil
		if err != nil && !errors.Is(err, store.ErrNotFound) {
			return teamtools.Refusal("internal", "%v", err)
		}
		if !refreshed {
			mr = store.MergeRequest{ProjectID: t.project.ID, Repo: r.name, Branch: branch}
		}
		mr.DeploymentID, mr.AuthorInstance, mr.Entry, mr.ReviewID = t.c.Deployment.ID, t.c.Instance.ID, entry, reviewID
		mr.Conflicting, mr.Note = false, ""
		if mr, err = t.w.MergeRequests.Save(ctx, mr); err != nil {
			return teamtools.Refusal("internal", "%v", err)
		}
		t.w.announce(mr)
		t.carry(ctx, r.dir, mr)
		t.act.Repos = append(t.act.Repos, store.ActionRepo{Repo: r.name, Branch: branch, MergeRequest: mr.ID})
		what := r.name + " (" + mr.ID + ")"
		if refreshed {
			what += ", refreshed"
		}
		made = append(made, what)
	}
	switch {
	case !found:
		return teamtools.Refusal("branch_not_found", "no branch %q in your repos", branch)
	case len(made) == 0:
		return teamtools.Refusal("nothing_to_merge", "%s changes nothing in its repo's main branch, in any repo", branch)
	}
	// Asked again after changes were requested: the review waits on the CEO again.
	if review, err := t.w.Reviews.Get(ctx, reviewID); err == nil && !review.ChangesRequestedAt.IsZero() {
		review.ChangesRequestedAt = time.Time{}
		if _, err := t.w.Reviews.Save(ctx, review); err != nil {
			slog.Warn("keeping a review", "review", reviewID, "err", err)
		}
	}
	t.w.review(ctx, reviewID)
	return agentlink.Result{Text: fmt.Sprintf("Review %s, %s: waiting for the CEO.", reviewID, strings.Join(made, ", "))}
}

// carry gives the merge request the integrated work items its branch
// contains that have none yet.
func (t tool) carry(ctx context.Context, dir string, mr store.MergeRequest) {
	items, err := t.w.WorkItems.ForDeployment(ctx, t.c.Deployment.ID)
	if err != nil {
		slog.Warn("work items", "deployment", t.c.Deployment.ID, "err", err)
		return
	}
	for _, item := range items {
		if item.Stage != store.StageIntegrated || item.MergeRequestID != "" || !t.w.Git.HasBranch(ctx, dir, item.Branch) {
			continue
		}
		if in, err := t.w.Git.Contains(ctx, dir, mr.Branch, item.Branch); err == nil && in {
			item.MergeRequestID = mr.ID
			t.w.save(ctx, item)
		}
	}
}

// mergeToMain squashes one merge request's branch, at head (the commit the
// CEO read), into its repo's main: one commit "<entry title>", in the CEO's
// checkout when it has main checked out, without a working tree otherwise
// (wiki/product/features/pipeline-and-merges.md#merge-to-main).
func (w *Workflow) mergeToMain(ctx context.Context, mr store.MergeRequest, head string) error {
	main, dir, err := w.repoOf(mr)
	if err != nil {
		return err
	}
	title := mr.Entry
	if e, err := w.Wiki.Entry(mr.ProjectID, mr.Entry); err == nil {
		title = e.Title
	}
	switch err := w.Git.Squash(ctx, dir, main, mr.Branch, head, title); {
	case errors.Is(err, git.ErrConflict):
		mr.Conflicting, mr.Note = true, "conflicts with "+main
		w.saveMerge(ctx, mr)
		w.tell(ctx, mr, "foreman", fmt.Sprintf("From Foreman: %s conflicts with %s in %s, so the CEO couldn't merge it. Merge %s in with merge(%q), resolve the conflicts and commit(), then call request_merge again.", mr.Branch, main, mr.Repo, main, main))
		return ErrConflict
	case err != nil:
		return err
	}
	w.merged(ctx, mr, main, dir, head)
	return nil
}

// reject ends one merge request unmerged: its work items go back to "in
// worktree", and the branch stays for the lead to rework.
func (w *Workflow) reject(ctx context.Context, mr store.MergeRequest, reason string) {
	mr.State, mr.RejectedAt, mr.Note = store.MergeRejected, time.Now().UTC(), reason
	w.saveMerge(ctx, mr)
	items, _ := w.WorkItems.ForDeployment(ctx, mr.DeploymentID)
	for _, item := range items {
		if item.MergeRequestID == mr.ID {
			item.MergeRequestID, item.Stage, item.Note = "", store.StageInWorktree, "rejected: "+reason
			if reason == "" {
				item.Note = "rejected"
			}
			w.save(ctx, item)
		}
	}
}

// Reconcile notices open merge requests whose branch changes nothing in main,
// whoever merged it (the CEO's own git, a forge), and gives a review whose
// request couldn't be saved (a database error) its request.
func (w *Workflow) Reconcile(ctx context.Context) {
	open, err := w.MergeRequests.Open(ctx)
	if err != nil {
		slog.Warn("open merge requests", "err", err)
		return
	}
	for _, mr := range open {
		main, dir, err := w.repoOf(mr)
		if err != nil {
			continue
		}
		head, err := w.Git.Resolve(ctx, dir, "refs/heads/"+mr.Branch)
		if merged, merr := w.Git.Merged(ctx, dir, main, mr.Branch); err == nil && merr == nil && merged {
			w.merged(ctx, mr, main, dir, head)
		} else {
			w.review(ctx, mr.ReviewID)
		}
	}
}

// merged ends a merge request as merged, tells its lead, and tells the
// leads of the other open ones on the same repo that main moved. Its review
// request is resolved once nothing in the review is left open. The branch,
// still at head (the commit merged), moves to the new main: a squash left
// its commits out of main, and a next review on it would show them again.
func (w *Workflow) merged(ctx context.Context, mr store.MergeRequest, main, dir, head string) {
	mr.State, mr.MergedAt, mr.Conflicting, mr.Note = store.MergeMerged, time.Now().UTC(), false, ""
	w.saveMerge(ctx, mr)
	if !w.anyOpen(ctx, mr.ReviewID) {
		w.resolveReview(ctx, mr.ReviewID, "merged")
	}
	next := fmt.Sprintf("%s now starts from %s.", mr.Branch, main)
	if err := w.Git.MoveBranch(ctx, dir, mr.Branch, head, main); err != nil {
		slog.Info("keeping a merged branch", "merge request", mr.ID, "err", err)
		next = fmt.Sprintf("%s has commits since, or changes in the way, so it still holds what was merged: merge %s into it with merge(%q) before you go on.", mr.Branch, main, main)
	}
	w.tell(ctx, mr, "foreman", fmt.Sprintf("From Foreman: %s was merged into %s in %s, as one commit. %s", mr.Branch, main, mr.Repo, next))
	open, err := w.MergeRequests.Open(ctx)
	if err != nil {
		return
	}
	for _, other := range open {
		if other.ProjectID == mr.ProjectID && other.Repo == mr.Repo {
			w.tell(ctx, other, "foreman", fmt.Sprintf("From Foreman: %s moved in %s (%s was merged). Merge it into %s with merge(%q), resolve any conflicts and commit(), then call request_merge again.", main, mr.Repo, mr.Branch, other.Branch, main))
		}
	}
}

// anyOpen reports whether a review still has an open merge request.
func (w *Workflow) anyOpen(ctx context.Context, reviewID string) bool {
	if reviewID == "" {
		return false
	}
	mrs, err := w.MergeRequests.ForReview(ctx, reviewID)
	if err != nil {
		return true // unknown: keep its request
	}
	for _, mr := range mrs {
		if mr.State == store.MergeOpen {
			return true
		}
	}
	return false
}

// repoOf is a merge request's repo: its main branch and its own checkout
// (the CEO's).
func (w *Workflow) repoOf(mr store.MergeRequest) (main, dir string, err error) {
	project, err := w.Config.Project(mr.ProjectID)
	if err != nil {
		return "", "", err
	}
	dir, ok := project.RepoDir(mr.Repo)
	if !ok {
		return "", "", fmt.Errorf("%w: %s has no repo %s agents can use", home.ErrNotFound, project.ID, mr.Repo)
	}
	return project.Main(mr.Repo), dir, nil
}

// tell delivers a notice to the merge request's lead, from Foreman or the CEO.
func (w *Workflow) tell(ctx context.Context, mr store.MergeRequest, from, text string) {
	if _, err := w.Messages.Send(ctx, store.TeamMessage{DeploymentID: mr.DeploymentID, FromInstance: from,
		ToInstance: mr.AuthorInstance, Kind: "notice", Body: text}); err != nil {
		slog.Warn("telling the lead", "merge request", mr.ID, "err", err)
	}
}

func (w *Workflow) saveMerge(ctx context.Context, mr store.MergeRequest) {
	saved, err := w.MergeRequests.Save(ctx, mr)
	if err != nil {
		slog.Warn("keeping a merge request", "id", mr.ID, "err", err)
		return
	}
	w.announce(saved)
}

func (w *Workflow) announce(mr store.MergeRequest) {
	if w.Publish != nil {
		w.Publish(events.Event{Type: "pipeline.changed", Data: mr})
	}
}

// review makes sure a review waiting on the CEO has its open review request:
// the CEO's "Needs you" on the Dashboard and the project's Overview
// (wiki/product/models/request.md). None while changes are requested (it
// waits on the lead) or once nothing in it is open.
func (w *Workflow) review(ctx context.Context, reviewID string) {
	if reviewID == "" {
		return
	}
	if _, err := w.Requests.ForReview(ctx, reviewID); err == nil || !errors.Is(err, store.ErrNotFound) {
		return
	}
	r, err := w.Reviews.Get(ctx, reviewID)
	if err != nil || !r.ChangesRequestedAt.IsZero() {
		return
	}
	mrs, err := w.MergeRequests.ForReview(ctx, reviewID)
	if err != nil {
		return
	}
	var open []store.MergeRequest
	for _, mr := range mrs {
		if mr.State == store.MergeOpen {
			open = append(open, mr)
		}
	}
	if len(open) == 0 {
		return
	}
	latest := open[len(open)-1]
	req, err := w.Requests.Save(ctx, store.Request{Kind: "review", ProjectID: r.ProjectID, DeploymentID: latest.DeploymentID,
		InstanceID: latest.AuthorInstance, Source: "agent", ReviewID: reviewID,
		Summary: fmt.Sprintf("Merge %s: %s.", r.Branch, w.titleOf(latest))})
	if err != nil {
		slog.Warn("raising a review", "review", reviewID, "err", err)
		return
	}
	w.publishRequest("request.created", req)
}

// titleOf is the title of the entry a merge request names, else its path.
func (w *Workflow) titleOf(mr store.MergeRequest) string {
	if e, err := w.Wiki.Entry(mr.ProjectID, mr.Entry); err == nil && e.Title != "" {
		return e.Title
	}
	return mr.Entry
}

// resolveReview closes a review's open request.
func (w *Workflow) resolveReview(ctx context.Context, reviewID, answer string) {
	if reviewID == "" {
		return
	}
	r, err := w.Requests.ForReview(ctx, reviewID)
	if err != nil {
		return
	}
	r.Status, r.Answer, r.ResolvedAt = store.RequestResolved, answer, time.Now().UTC()
	if r, err = w.Requests.Save(ctx, r); err != nil {
		slog.Warn("resolving a review", "review", reviewID, "err", err)
		return
	}
	w.publishRequest("request.resolved", r)
}

func (w *Workflow) publishRequest(kind string, r store.Request) {
	if w.Publish != nil {
		w.Publish(events.Event{Type: kind, Data: r})
	}
}

// OpenRequests lists what is open for the CEO.
func (w *Workflow) OpenRequests(ctx context.Context) ([]store.Request, error) {
	return w.Requests.Open(ctx)
}
