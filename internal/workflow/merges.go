package workflow

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
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
	// merged, dismissed or withdrawn already), or none in the repos named.
	ErrNotOpen = errors.New("workflow: nothing open to decide")
	// ErrConflict means the branch doesn't merge cleanly into its target: the
	// merge request is marked so and the lead told to merge the target in.
	ErrConflict = git.ErrConflict
	// ErrLocalChanges means the CEO's checkout has the target checked out with
	// uncommitted changes the merge would overwrite.
	ErrLocalChanges = git.ErrLocalChanges
	// ErrBranchMoved means the branch isn't at the commit the CEO read: the
	// lead committed since, and the CEO hasn't seen it.
	ErrBranchMoved = git.ErrMoved
)

func (t tool) requestMerge(ctx context.Context, branch, title, body, entry, into, land string) agentlink.Result {
	if !t.c.Lead {
		return teamtools.Refusal("not_lead", "only the lead requests merges: hand your work to the lead")
	}
	prefix := git.BranchPrefix(t.project.ID, "", t.c.Team.ID, true)
	if !strings.HasPrefix(branch, prefix) || !git.ValidBranch(ctx, branch) {
		return teamtools.Refusal("branch_not_allowed", "you request merges of your own lead branches only, %s<topic>", prefix)
	}
	switch {
	case title == "":
		return teamtools.Refusal("empty", "give the merge a title: what it brings, in one short line")
	case teamtools.BadTitle(title):
		return teamtools.Refusal("too_long", "a title is one line of at most %d characters: put the rest in body", teamtools.MaxTitle)
	case teamtools.TooLong(body, teamtools.MaxBody):
		return teamtools.Refusal("too_long", "a body is at most %d characters: put the long parts in the wiki entry", teamtools.MaxBody)
	}
	if entry == "" {
		return teamtools.Refusal("entry_not_found", "name the wiki entry that explains the work, usually its task card")
	}
	if _, err := t.w.Wiki.Title(t.project.ID, entry); err != nil {
		return teamtools.Refusal("entry_not_found", "no wiki entry %q: give its path in the wiki, e.g. backlog/export-csv.md", entry)
	}
	if git.AgentBranch(into) {
		return teamtools.Refusal("agent_branch", "%s is an agent's branch: a merge goes into one of the CEO's branches (the default when left out)", into)
	}
	// A landing is only proposed: the CEO makes it on Merge, under the
	// project's prefixes (the CEO, 2026-11-11).
	if land != "" {
		prefixes := t.project.Config.LandingPrefixes
		switch {
		case len(prefixes) == 0:
			return teamtools.Refusal("landing_not_allowed", "this project takes no landing names (project.toml's landing_prefixes): leave out land, and it lands on into")
		case !git.ValidBranch(ctx, land) || !slices.ContainsFunc(prefixes, func(p string) bool { return strings.HasPrefix(land, p) && len(land) > len(p) }):
			return teamtools.Refusal("landing_not_allowed", "%q isn't a landing name you may propose: one under %s", land, strings.Join(prefixes, ", "))
		}
	}
	if res, ok := t.unsettled(ctx, t.worktrees(), "a merge request is your committed work"); !ok {
		return res
	}
	t.integrate(ctx) // hand-offs merged since the last check go with it
	// Every repo is checked before any merge request is made, so a refusal
	// leaves none half made.
	type request struct {
		r      repo
		mr     store.MergeRequest
		exists bool
		held   bool // moved to a branch that holds all of it already
	}
	var requests []request
	var lacking, taken []string
	found := false
	for _, r := range t.worktrees() {
		if !t.w.Git.HasBranch(ctx, r.dir, branch) {
			continue
		}
		found = true
		mr, err := t.w.MergeRequests.OpenFor(ctx, t.project.ID, r.name, branch)
		exists, was := err == nil, mr.Target
		if err != nil && !errors.Is(err, store.ErrNotFound) {
			return teamtools.Refusal("internal", "%v", err)
		}
		// Into the branch named, else the one it goes to already, else the
		// repo's default; landing on land when named, else where it did.
		switch {
		case into != "":
			mr.Target = into
		case !exists:
			mr.Target = t.project.Default(r.name)
		}
		if land != "" {
			mr.Landing = land
		}
		if !t.w.Git.HasBranch(ctx, r.dir, mr.Target) {
			// A repo the review has open goes with it, so the review never
			// splits; any other needs it only with new commits, reckoned from
			// its default.
			if merged, err := t.w.Git.Merged(ctx, r.dir, t.project.Default(r.name), branch); err != nil {
				return gitError(err)
			} else if exists || !merged {
				lacking = append(lacking, fmt.Sprintf("%s (no %s)", r.name, mr.Target))
			}
			continue
		}
		merged, err := t.w.Git.Merged(ctx, r.dir, mr.Target, branch)
		if err != nil {
			return gitError(err)
		} else if merged && !(exists && mr.Target != was) {
			continue
		}
		if mr.Landing != "" && !t.w.landable(ctx, r.dir, mr.Target, mr.Landing) {
			taken = append(taken, fmt.Sprintf("%s (%s)", r.name, mr.Landing))
			continue
		}
		requests = append(requests, request{r, mr, exists, merged})
	}
	switch {
	case !found:
		return teamtools.Refusal("branch_not_found", "no branch %q in your repos", branch)
	case len(lacking) > 0:
		return teamtools.Refusal("no_such_branch", "%s is open or has new commits in %s, which lacks the branch it would merge into: ask the CEO to make it there, or split the work", branch, strings.Join(lacking, ", "))
	case len(taken) > 0:
		return teamtools.Refusal("landing_taken", "the landing is a branch already in %s, at another commit than the branch it merges into: name another with land", strings.Join(taken, ", "))
	case len(requests) == 0:
		return teamtools.Refusal("nothing_to_merge", "%s changes nothing in the branch it merges into, in any repo", branch)
	}
	// The branch's open review takes every repo's merge request; with none
	// open, a new review. Either way it takes this call's title and body.
	reviewID, err := t.w.MergeRequests.OpenReview(ctx, t.project.ID, branch)
	if err != nil {
		return teamtools.Refusal("internal", "%v", err)
	}
	review := store.Review{ProjectID: t.project.ID, Branch: branch}
	if reviewID != "" {
		if review, err = t.w.Reviews.Get(ctx, reviewID); err != nil {
			return teamtools.Refusal("internal", "%v", err)
		}
	}
	review.Title, review.Body = title, body
	if review, err = t.w.Reviews.Save(ctx, review); err != nil {
		return teamtools.Refusal("internal", "%v", err)
	}
	reviewID = review.ID
	var made []string
	waiting := false
	for _, q := range requests {
		mr := q.mr
		if !q.exists {
			mr.ProjectID, mr.Repo, mr.Branch = t.project.ID, q.r.name, branch
		}
		mr.DeploymentID, mr.AuthorInstance, mr.Entry, mr.ReviewID = t.c.Deployment.ID, t.c.Instance.ID, entry, reviewID
		mr.Conflicting, mr.Note = false, ""
		if mr, err = t.w.MergeRequests.Save(ctx, mr); err != nil {
			return teamtools.Refusal("internal", "%v", err)
		}
		t.w.announce(mr)
		t.w.carry(ctx, q.r.dir, mr)
		t.act.Repos = append(t.act.Repos, store.ActionRepo{Repo: q.r.name, Branch: branch, MergeRequest: mr.ID})
		what := fmt.Sprintf("%s (%s) into %s", q.r.name, mr.ID, mr.Target)
		if mr.Landing != "" {
			what += ", landing on " + mr.Landing
		}
		switch {
		case q.held:
			// Settled now, not at the next Reconcile: the review never shows it
			// open for the CEO to merge what is there already.
			head, _ := t.w.Git.Resolve(ctx, q.r.dir, "refs/heads/"+branch)
			t.w.merged(ctx, mr, mr.Target, q.r.dir, head)
			what += ", which holds all of it already: counted as merged"
		case q.exists:
			what += ", refreshed"
		}
		waiting = waiting || !q.held
		made = append(made, what)
	}
	// Asked again after changes were requested: the review waits on the CEO again.
	if review, err := t.w.Reviews.Get(ctx, reviewID); err == nil && !review.ChangesRequestedAt.IsZero() {
		review.ChangesRequestedAt = time.Time{}
		if _, err := t.w.Reviews.Save(ctx, review); err != nil {
			slog.Warn("keeping a review", "review", reviewID, "err", err)
		}
	}
	t.w.review(ctx, reviewID)
	next := "waiting for the CEO"
	if !waiting {
		next = "nothing left for the CEO"
	}
	return agentlink.Result{Text: fmt.Sprintf("Review %s, %s: %s.", reviewID, strings.Join(made, ", "), next)}
}

// carry gives the merge request the integrated work items of its deployment
// its branch contains that have none yet, or only a dismissed one (the lead
// asked again on a branch the CEO dismissed but kept).
func (w *Workflow) carry(ctx context.Context, dir string, mr store.MergeRequest) {
	items, err := w.WorkItems.ForDeployment(ctx, mr.DeploymentID)
	if err != nil {
		slog.Warn("work items", "deployment", mr.DeploymentID, "err", err)
		return
	}
	for _, item := range items {
		if item.Stage != store.StageIntegrated || !w.free(ctx, item) || !w.Git.HasBranch(ctx, dir, item.Branch) {
			continue
		}
		if in, err := w.Git.Contains(ctx, dir, mr.Branch, item.Branch); err == nil && in {
			item.MergeRequestID = mr.ID
			w.save(ctx, item)
		}
	}
}

// mergeToTarget squashes one merge request's branch, at head (the commit
// the CEO read), against its target onto where it lands (mr.Landing: made at
// the target's head, or the target itself): one commit "<entry title>", in
// the CEO's checkout when it has that branch checked out, without a working
// tree otherwise (wiki/product/features/pipeline-and-merges.md#merge-to-a-target).
func (w *Workflow) mergeToTarget(ctx context.Context, mr store.MergeRequest, head string) error {
	target, dir, err := w.repoOf(mr)
	if err != nil {
		return err
	}
	title := mr.Entry
	if t, err := w.Wiki.Title(mr.ProjectID, mr.Entry); err == nil {
		title = t
	}
	onto := cmp.Or(mr.Landing, target)
	switch err := w.Git.Squash(ctx, dir, target, onto, mr.Branch, head, title); {
	case errors.Is(err, git.ErrBranchExists):
		return fmt.Errorf("%w: %s is in %s already, at another commit than %s: land it under another name", ErrInvalid, onto, mr.Repo, target)
	case errors.Is(err, git.ErrConflict):
		mr.Conflicting, mr.Note = true, "conflicts with "+target
		w.saveMerge(ctx, mr)
		w.tell(ctx, mr, "foreman", fmt.Sprintf("From Foreman: %s conflicts with %s in %s, so the CEO couldn't merge it. Merge %s in with merge(%q), resolve the conflicts and commit(), then call request_merge again.", mr.Branch, target, mr.Repo, target, target))
		return ErrConflict
	case err != nil:
		return err
	}
	w.merged(ctx, mr, onto, dir, head)
	return nil
}

// landable says whether landing can take a merge into target in dir: not a
// branch yet, or one at target's head (the CEO, 2026-11-11).
func (w *Workflow) landable(ctx context.Context, dir, target, landing string) bool {
	if !w.Git.HasBranch(ctx, dir, landing) {
		return true
	}
	at, err := w.Git.Resolve(ctx, dir, "refs/heads/"+landing)
	was, err2 := w.Git.Resolve(ctx, dir, "refs/heads/"+target)
	return err == nil && err2 == nil && at == was
}

// free says whether a work item is carried by no merge request, or only by
// a dismissed or withdrawn one.
func (w *Workflow) free(ctx context.Context, item store.WorkItem) bool {
	if item.MergeRequestID == "" {
		return true
	}
	mr, err := w.MergeRequests.Get(ctx, item.MergeRequestID)
	return err == nil && (mr.State == store.MergeRejected || mr.State == store.MergeWithdrawn)
}

// reject ends one merge request unmerged. Its work items stay carried by
// it, so they leave In flight with it: the work isn't wanted. A new
// hand-off of one of their branches frees it (handed).
func (w *Workflow) reject(ctx context.Context, mr store.MergeRequest, reason string) {
	w.settle(ctx, &mr, "")
	mr.State, mr.RejectedAt, mr.Note = store.MergeRejected, time.Now().UTC(), reason
	w.saveMerge(ctx, mr)
}

// Reconcile notices open merge requests whose branch changes nothing in their
// target,
// whoever merged it (the CEO's own git, a forge), and gives a review whose
// request couldn't be saved (a database error) its request.
func (w *Workflow) Reconcile(ctx context.Context) {
	open, err := w.MergeRequests.Open(ctx)
	if err != nil {
		slog.Warn("open merge requests", "err", err)
		return
	}
	for _, mr := range open {
		// An earlier one's merge may have withdrawn it since the list was read.
		if now, err := w.MergeRequests.Get(ctx, mr.ID); err != nil || now.State != store.MergeOpen {
			continue
		}
		target, dir, err := w.repoOf(mr)
		if err != nil {
			continue
		}
		head, err := w.Git.Resolve(ctx, dir, "refs/heads/"+mr.Branch)
		if merged, merr := w.Git.Merged(ctx, dir, target, mr.Branch); err == nil && merr == nil && merged {
			w.merged(ctx, mr, target, dir, head)
		} else {
			w.review(ctx, mr.ReviewID)
		}
	}
}

// merged ends a merge request as merged, tells its lead, withdraws the
// lead's other reviews their targets now hold all of (supersede), and tells
// the leads of the other open ones into the same target of the same repo that
// it moved. Its review request is resolved once nothing in the review is left
// open. The branch, still at head (the commit merged), moves to the target
// as it is now: a squash left its commits out of it, and a next review on it
// would show them again. Idle desks follow the default branch only.
func (w *Workflow) merged(ctx context.Context, mr store.MergeRequest, target, dir, head string) {
	w.settle(ctx, &mr, head)
	mr.State, mr.MergedAt, mr.Conflicting, mr.Note = store.MergeMerged, time.Now().UTC(), false, ""
	w.saveMerge(ctx, mr)
	if !w.anyOpen(ctx, mr.ReviewID) {
		w.resolveReview(ctx, mr.ReviewID, "merged")
	}
	next := fmt.Sprintf("%s now starts from %s.", mr.Branch, target)
	if err := w.Git.MoveBranch(ctx, dir, mr.Branch, head, target); err != nil {
		slog.Info("keeping a merged branch", "merge request", mr.ID, "err", err)
		next = fmt.Sprintf("%s has commits since, or changes in the way, so it still holds what was merged: merge %s into it with merge(%q) before you go on.", mr.Branch, target, target)
	}
	w.tell(ctx, mr, "foreman", fmt.Sprintf("From Foreman: %s was merged into %s in %s, as one commit. %s", mr.Branch, target, mr.Repo, next))
	if p, err := w.Config.Project(mr.ProjectID); err == nil && p.Default(mr.Repo) == target {
		w.freshenDesks(ctx, mr.ProjectID, dir, target)
	}
	w.supersede(ctx, mr)
	open, err := w.MergeRequests.Open(ctx)
	if err != nil {
		return
	}
	for _, other := range open {
		if other.ProjectID == mr.ProjectID && other.Repo == mr.Repo && other.Target == target {
			w.tell(ctx, other, "foreman", fmt.Sprintf("From Foreman: %s moved in %s (%s was merged). Merge it into %s with merge(%q), resolve any conflicts and commit(), then call request_merge again.", target, mr.Repo, mr.Branch, other.Branch, target))
		}
	}
}

// freshenDesks brings the project's idle desks in the repo up to its new
// default branch, def: every agent's home branch there (another project
// sharing the repo has its own default) holding nothing def lacks, with nothing
// uncommitted where it is checked out (git.Freshen). A desk with work stays
// as it is.
func (w *Workflow) freshenDesks(ctx context.Context, project, dir, def string) {
	for _, prefix := range []string{"wt/" + project + "/", "lead/" + project + "/"} {
		branches, err := w.Git.Branches(ctx, dir, prefix)
		if err != nil {
			slog.Info("listing the idle desks", "repo", dir, "err", err)
			return
		}
		for _, b := range branches {
			if !git.IsHomeBranch(b) {
				continue
			}
			if _, err := w.Git.Freshen(ctx, dir, b, def); err != nil {
				slog.Info("bringing a desk up to the default branch", "repo", dir, "branch", b, "err", err)
			}
		}
	}
}

// backupsKept is how long a backup under refs/foreman/ is kept.
const backupsKept = 30 * 24 * time.Hour

// PruneBackups deletes the backups older than backupsKept in every project's
// code repos.
func (w *Workflow) PruneBackups(ctx context.Context) {
	for _, p := range w.Config.Projects() {
		for _, r := range p.Repos {
			if dir, ok := p.RepoDir(r.Name); ok {
				w.pruneBackups(ctx, dir)
			}
		}
	}
}

func (w *Workflow) pruneBackups(ctx context.Context, dir string) {
	if err := w.Git.PruneBackups(ctx, dir, w.Now().Add(-backupsKept)); err != nil {
		slog.Info("deleting old backups", "repo", dir, "err", err)
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

// repoOf is a merge request's target (its repo's default branch when it
// names none) and its repo's own checkout (the CEO's).
func (w *Workflow) repoOf(mr store.MergeRequest) (target, dir string, err error) {
	project, err := w.Config.Project(mr.ProjectID)
	if err != nil {
		return "", "", err
	}
	dir, ok := project.RepoDir(mr.Repo)
	if !ok {
		return "", "", fmt.Errorf("%w: %s has no repo %s agents can use", home.ErrNotFound, project.ID, mr.Repo)
	}
	if mr.Target == "" {
		return project.Default(mr.Repo), dir, nil
	}
	return mr.Target, dir, nil
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
		Summary: fmt.Sprintf("Merge %s: %s", r.Branch, r.Title)})
	if err != nil {
		slog.Warn("raising a review", "review", reviewID, "err", err)
		return
	}
	w.publishRequest("request.created", req)
}

// titleOf is the title of the entry a merge request names, else its path.
func (w *Workflow) titleOf(mr store.MergeRequest) string {
	if t, err := w.Wiki.Title(mr.ProjectID, mr.Entry); err == nil && t != "" {
		return t
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
	if r, err = w.Requests.Settle(ctx, r.ID, store.RequestResolved, answer); err != nil {
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

// settle records where a merge request's branch stands as it leaves open:
// its head (the one merged, else the branch's), kept under
// refs/foreman/decided/<id> so the commits outlive the branch, and where it
// parted from its target, so its review still shows the change
// (wiki/product/features/reviews.md#decided). A failure leaves them unset:
// the review then says it can't show the change.
func (w *Workflow) settle(ctx context.Context, mr *store.MergeRequest, head string) {
	target, dir, err := w.repoOf(*mr)
	if err == nil && head == "" {
		head, err = w.Git.Resolve(ctx, dir, "refs/heads/"+mr.Branch)
	}
	if err == nil {
		err = w.Git.Keep(ctx, dir, "refs/foreman/decided/"+mr.ID, head)
	}
	if err != nil {
		slog.Info("recording a decided merge request", "id", mr.ID, "err", err)
		return
	}
	mr.Head = head
	if mr.Base, err = w.Git.PartedFrom(ctx, dir, target, head); err != nil {
		slog.Info("where a decided merge request parted from its target", "id", mr.ID, "err", err)
	}
}
