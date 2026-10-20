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

// Merge requests: the lead's request_merge, then the CEO's Merge to main or
// Reject (wiki/product/features/pipeline-and-merges.md#review-and-merge,
// wiki/product/models/merge-request.md).

var (
	// ErrNotOpen means the merge request was already merged or rejected.
	ErrNotOpen = errors.New("workflow: the merge request isn't open")
	// ErrConflict means the branch doesn't merge cleanly into main: the
	// merge request is marked so and the lead told to merge main in.
	ErrConflict = git.ErrConflict
	// ErrLocalChanges means the CEO's checkout has main checked out with
	// uncommitted changes the merge would overwrite.
	ErrLocalChanges = git.ErrLocalChanges
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
	main := t.project.Config.Main
	var made []string
	found := false
	for _, r := range t.worktrees() {
		if !t.w.Git.HasBranch(ctx, r.dir, branch) {
			continue
		}
		found = true
		if n, err := t.w.Git.Ahead(ctx, r.dir, branch, main); err != nil {
			return gitError(err)
		} else if n == 0 {
			continue
		}
		mr, err := t.w.MergeRequests.OpenFor(ctx, t.project.ID, r.name, branch)
		refreshed := err == nil
		if err != nil && !errors.Is(err, store.ErrNotFound) {
			return teamtools.Refusal("internal", "%v", err)
		}
		if !refreshed {
			mr = store.MergeRequest{ProjectID: t.project.ID, Repo: r.name, Branch: branch}
		}
		mr.DeploymentID, mr.AuthorInstance, mr.Entry = t.c.Deployment.ID, t.c.Instance.ID, entry
		mr.Conflicting, mr.Note = false, ""
		if mr, err = t.w.MergeRequests.Save(ctx, mr); err != nil {
			return teamtools.Refusal("internal", "%v", err)
		}
		t.w.announce(mr)
		t.w.review(ctx, mr)
		t.carry(ctx, r.dir, mr)
		t.act.Repos = append(t.act.Repos, store.ActionRepo{Repo: r.name, Branch: branch, MergeRequest: mr.ID})
		what := mr.ID + " (" + r.name + ")"
		if refreshed {
			what += ", refreshed"
		}
		made = append(made, what)
	}
	switch {
	case !found:
		return teamtools.Refusal("branch_not_found", "no branch %q in your repos", branch)
	case len(made) == 0:
		return teamtools.Refusal("nothing_to_merge", "%s has no commits beyond %s in any repo", branch, main)
	}
	return agentlink.Result{Text: fmt.Sprintf("Merge request %s: waiting for the CEO.", strings.Join(made, ", "))}
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

// MergeRequest returns one merge request.
func (w *Workflow) MergeRequest(ctx context.Context, id string) (store.MergeRequest, error) {
	return w.MergeRequests.Get(ctx, id)
}

// Diff is what a merge request's branch changes against main.
func (w *Workflow) Diff(ctx context.Context, id string) (string, error) {
	mr, main, dir, err := w.target(ctx, id)
	if err != nil {
		return "", err
	}
	return w.Git.Diff(ctx, dir, main, mr.Branch)
}

// MergeToMain is the CEO's Merge to main: a merge commit "Merge <entry
// title>", in the CEO's checkout when it has main checked out, without a
// working tree otherwise (wiki/product/features/pipeline-and-merges.md#merge-to-main).
func (w *Workflow) MergeToMain(ctx context.Context, id string) error {
	mr, main, dir, err := w.target(ctx, id)
	if err != nil {
		return err
	}
	if mr.State != store.MergeOpen {
		return ErrNotOpen
	}
	title := mr.Entry
	if e, err := w.Wiki.Entry(mr.ProjectID, mr.Entry); err == nil {
		title = e.Title
	}
	switch err := w.Git.MergeInto(ctx, dir, main, mr.Branch, "Merge "+title); {
	case errors.Is(err, git.ErrConflict):
		mr.Conflicting, mr.Note = true, "conflicts with "+main
		w.saveMerge(ctx, mr)
		w.tell(ctx, mr, "foreman", fmt.Sprintf("From Foreman: %s conflicts with %s in %s, so the CEO couldn't merge it. Merge %s in with merge(%q), resolve the conflicts and commit(), then call request_merge again.", mr.Branch, main, mr.Repo, main, main))
		return ErrConflict
	case err != nil:
		return err
	}
	w.merged(ctx, mr, main)
	return nil
}

// Reject is the CEO's Reject: the reason goes to the lead, its work items
// back to "in worktree", and the branch stays for the lead to rework.
func (w *Workflow) Reject(ctx context.Context, id, reason string) error {
	mr, err := w.MergeRequests.Get(ctx, id)
	if err != nil {
		return err
	}
	if mr.State != store.MergeOpen {
		return ErrNotOpen
	}
	mr.State, mr.RejectedAt, mr.Note = store.MergeRejected, time.Now().UTC(), reason
	w.saveMerge(ctx, mr)
	w.resolveReview(ctx, mr, "rejected")
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
	why := ""
	if reason != "" {
		why = ": " + reason
	}
	w.tell(ctx, mr, "ceo", fmt.Sprintf("From the CEO: the merge of %s in %s was rejected%s. Its work items are back in your hands; switch back to the branch to rework it, then call request_merge again.", mr.Branch, mr.Repo, why))
	return nil
}

// Reconcile notices open merge requests whose branch is in main already,
// whoever merged it (the CEO's own git, a forge), and gives one whose
// review couldn't be saved (a database error) its review.
func (w *Workflow) Reconcile(ctx context.Context) {
	open, err := w.MergeRequests.Open(ctx)
	if err != nil {
		slog.Warn("open merge requests", "err", err)
		return
	}
	for _, mr := range open {
		_, main, dir, err := w.target(ctx, mr.ID)
		if err != nil {
			continue
		}
		if in, err := w.Git.Contains(ctx, dir, main, mr.Branch); err == nil && in {
			w.merged(ctx, mr, main)
		} else {
			w.review(ctx, mr) // one whose review couldn't be saved gets it now
		}
	}
}

// merged ends a merge request as merged, tells its lead, and tells the
// leads of the other open ones on the same repo that main moved.
func (w *Workflow) merged(ctx context.Context, mr store.MergeRequest, main string) {
	mr.State, mr.MergedAt, mr.Conflicting, mr.Note = store.MergeMerged, time.Now().UTC(), false, ""
	w.saveMerge(ctx, mr)
	w.resolveReview(ctx, mr, "merged")
	w.tell(ctx, mr, "foreman", fmt.Sprintf("From Foreman: %s was merged into %s in %s.", mr.Branch, main, mr.Repo))
	open, err := w.MergeRequests.Open(ctx)
	if err != nil {
		return
	}
	for _, other := range open {
		if other.ProjectID == mr.ProjectID && other.Repo == mr.Repo {
			w.tell(ctx, other, "foreman", fmt.Sprintf("From Foreman: %s moved (%s was merged). Merge it into %s with merge(%q), resolve any conflicts and commit(), then call request_merge again.", main, mr.Branch, other.Branch, main))
		}
	}
}

// target is a merge request with its project's protected branch and the
// repo's own checkout (the CEO's).
func (w *Workflow) target(ctx context.Context, id string) (store.MergeRequest, string, string, error) {
	mr, err := w.MergeRequests.Get(ctx, id)
	if err != nil {
		return mr, "", "", err
	}
	project, err := w.Config.Project(mr.ProjectID)
	if err != nil {
		return mr, "", "", err
	}
	dir, ok := project.RepoDir(mr.Repo)
	if !ok {
		return mr, "", "", fmt.Errorf("%w: %s has no repo %s agents can use", home.ErrNotFound, project.ID, mr.Repo)
	}
	return mr, project.Config.Main, dir, nil
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

// review makes sure an open merge request has its open review request: the
// CEO's "Needs you" on the Dashboard and in Changes (wiki/product/models/request.md).
func (w *Workflow) review(ctx context.Context, mr store.MergeRequest) {
	if _, err := w.Requests.ForMergeRequest(ctx, mr.ID); err == nil || !errors.Is(err, store.ErrNotFound) {
		return
	}
	main, title := "main", mr.Entry
	if project, err := w.Config.Project(mr.ProjectID); err == nil {
		main = project.Config.Main
	}
	if e, err := w.Wiki.Entry(mr.ProjectID, mr.Entry); err == nil {
		title = e.Title
	}
	r, err := w.Requests.Save(ctx, store.Request{Kind: "review", ProjectID: mr.ProjectID, DeploymentID: mr.DeploymentID,
		InstanceID: mr.AuthorInstance, Source: "agent", MergeRequestID: mr.ID,
		Summary: fmt.Sprintf("Merge %s into %s in %s: %s.", mr.Branch, main, mr.Repo, title)})
	if err != nil {
		slog.Warn("raising a review", "merge request", mr.ID, "err", err)
		return
	}
	w.publishRequest("request.created", r)
}

// resolveReview closes a merge request's review once it left open.
func (w *Workflow) resolveReview(ctx context.Context, mr store.MergeRequest, answer string) {
	r, err := w.Requests.ForMergeRequest(ctx, mr.ID)
	if err != nil {
		return
	}
	r.Status, r.Answer, r.ResolvedAt = store.RequestResolved, answer, time.Now().UTC()
	if r, err = w.Requests.Save(ctx, r); err != nil {
		slog.Warn("resolving a review", "merge request", mr.ID, "err", err)
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
