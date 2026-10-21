package workflow

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/brickpop/foreman/internal/git"
	"github.com/brickpop/foreman/internal/store"
	"github.com/brickpop/foreman/internal/teamtools"
)

// Reviews: the merge requests of one lead branch, read and decided together
// by the CEO, with comments that go back to the lead
// (wiki/product/features/reviews.md, wiki/product/models/review.md).

// A review's status, derived from its merge requests.
const (
	ReviewNeedsYou         = "needs-review"
	ReviewChangesRequested = "changes-requested" // waiting on the lead
	ReviewMerged           = "merged"            // nothing open, something merged
	ReviewDismissed        = "dismissed"         // nothing open, nothing merged
	ReviewWithdrawn        = "withdrawn"         // the lead, or Foreman, took it back
)

var (
	// ErrInvalid is a request about a review that can't be done as asked: the
	// error says why, for the CEO.
	ErrInvalid = errors.New("workflow: invalid")
	// ErrSent means the comment was sent already: it stays as the lead got it.
	ErrSent = errors.New("workflow: the comment was sent already")
)

// Review is one review as the CEO reads it.
type Review struct {
	store.Review  `tstype:",extends"`
	Status        string               `json:"status"`
	Entry         string               `json:"entry"` // the wiki entry its latest request named
	Title         string               `json:"title"` // that entry's title
	DeploymentID  string               `json:"deploymentId"`
	Lead          string               `json:"lead"` // the instance that asked
	UpdatedAt     time.Time            `json:"updatedAt"`
	MergeRequests []store.MergeRequest `json:"mergeRequests"`
	Comments      []Comment            `json:"comments"` // only when read on its own
}

// Comment is a comment where it is now: a line comment follows its line as
// the lead's commits move it, and is outdated once that line changed.
type Comment struct {
	store.ReviewComment `tstype:",extends"`
	Now                 int  `json:"now"` // its line on the branch's head; Line while it can't move
	Outdated            bool `json:"outdated"`
}

// Dismiss is the CEO's Dismiss of a review: what becomes of its branches and team.
type Dismiss struct {
	Note         string `json:"note"` // to the lead, when its team keeps working
	DropBranches bool   `json:"dropBranches"`
	StopTeam     bool   `json:"stopTeam"`
}

// Dismissed is what a dismissal did not do as asked: the branches it couldn't drop.
type Dismissed struct {
	Kept []KeptBranch `json:"kept"`
}

// KeptBranch is a branch a dismissal kept, and why, for the CEO.
type KeptBranch struct {
	Repo   string `json:"repo"`
	Branch string `json:"branch"`
	Why    string `json:"why"`
}

// Withdrawn is a review the lead withdrew, or Foreman did (By "foreman")
// when the CEO merged another holding all of it: the review.withdrawn event.
type Withdrawn struct {
	Review       store.Review `json:"review"`
	DeploymentID string       `json:"deploymentId"`
	Lead         string       `json:"lead"`       // the instance whose review it was
	By           string       `json:"by"`         // lead or foreman
	LostDrafts   int          `json:"lostDrafts"` // the CEO's draft comments on it, gone with it
}

// RepoDiff is one repo's change in a review.
type RepoDiff struct {
	Repo  string `json:"repo"`
	Main  string `json:"main"`
	Head  string `json:"head"`  // the branch's head the diff was made at
	Since string `json:"since"` // with since: the head it starts from ("" for against main)
	Diff  string `json:"diff"`
}

// status is a review's state from its merge requests, and its withdrawal.
func status(r store.Review, mrs []store.MergeRequest) string {
	merged := false
	for _, mr := range mrs {
		switch mr.State {
		case store.MergeOpen:
			if r.ChangesRequestedAt.IsZero() {
				return ReviewNeedsYou
			}
			return ReviewChangesRequested
		case store.MergeMerged:
			merged = true
		}
	}
	switch {
	case !r.WithdrawnAt.IsZero():
		return ReviewWithdrawn
	case merged:
		return ReviewMerged
	}
	return ReviewDismissed
}

// ListReviews lists every review, newest first, without their comments.
func (w *Workflow) ListReviews(ctx context.Context) ([]Review, error) {
	list, err := w.Reviews.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Review, 0, len(list))
	for i := len(list) - 1; i >= 0; i-- {
		v, err := w.view(ctx, list[i])
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

// Review returns one review with its comments, each where it is now.
func (w *Workflow) Review(ctx context.Context, id string) (Review, error) {
	r, err := w.Reviews.Get(ctx, id)
	if err != nil {
		return Review{}, err
	}
	v, err := w.view(ctx, r)
	if err != nil {
		return Review{}, err
	}
	comments, err := w.ReviewComments.ForReview(ctx, id)
	if err != nil {
		return Review{}, err
	}
	heads := w.heads(ctx, v.MergeRequests)
	v.Comments = make([]Comment, len(comments))
	for i, c := range comments {
		v.Comments[i] = w.follow(ctx, v.MergeRequests, heads, c)
	}
	return v, nil
}

func (w *Workflow) view(ctx context.Context, r store.Review) (Review, error) {
	mrs, err := w.MergeRequests.ForReview(ctx, r.ID)
	if err != nil {
		return Review{}, err
	}
	v := Review{Review: r, Status: status(r, mrs), MergeRequests: mrs, UpdatedAt: r.CreatedAt}
	for _, mr := range mrs {
		v.Entry, v.DeploymentID, v.Lead = mr.Entry, mr.DeploymentID, mr.AuthorInstance
		for _, t := range []time.Time{mr.CreatedAt, mr.MergedAt, mr.RejectedAt, r.ChangesRequestedAt, r.WithdrawnAt} {
			if t.After(v.UpdatedAt) {
				v.UpdatedAt = t
			}
		}
	}
	if len(mrs) > 0 {
		v.Title = w.titleOf(mrs[len(mrs)-1])
	}
	return v, nil
}

// heads are the branch's heads, by repo, for the open merge requests.
func (w *Workflow) heads(ctx context.Context, mrs []store.MergeRequest) map[string]string {
	heads := map[string]string{}
	for _, mr := range mrs {
		if mr.State != store.MergeOpen {
			continue
		}
		if _, dir, err := w.repoOf(mr); err == nil {
			if head, err := w.Git.Resolve(ctx, dir, "refs/heads/"+mr.Branch); err == nil {
				heads[mr.Repo] = head
			}
		}
	}
	return heads
}

// follow places a comment on the branch's head. One on the branch's side
// moves with the lines added or removed above it, and is outdated once its
// own line changed; one on main's side (a removed line) once its file
// changed. A comment in a repo no longer open stays where it was written.
func (w *Workflow) follow(ctx context.Context, mrs []store.MergeRequest, heads map[string]string, c store.ReviewComment) Comment {
	out := Comment{ReviewComment: c, Now: c.Line}
	head, open := heads[c.Repo]
	if c.Repo == "" || !open || head == c.Commit {
		return out
	}
	i := slices.IndexFunc(mrs, func(mr store.MergeRequest) bool { return mr.Repo == c.Repo })
	_, dir, err := w.repoOf(mrs[i])
	if err != nil {
		return out
	}
	diff, err := w.Git.Compare(ctx, dir, c.Commit, head, 0, c.Path)
	switch {
	case err != nil:
		out.Outdated = true // its commit is gone: nowhere to place it
	case c.Side == "old":
		out.Outdated = diff != ""
	default:
		out.Now, out.Outdated = followLine(diff, c.Line)
	}
	return out
}

// followLine is where line (of the old side of a `git diff -U0`) is on its
// new side: moved by the hunks above it, outdated when a hunk changed it.
func followLine(diff string, line int) (int, bool) {
	shift := 0
	for _, l := range strings.Split(diff, "\n") {
		if !strings.HasPrefix(l, "@@ ") {
			continue
		}
		oldStart, oldCount, newCount, ok := hunkRange(l)
		if !ok {
			return line, true
		}
		switch {
		case oldCount > 0 && line >= oldStart && line < oldStart+oldCount:
			return line, true
		case oldCount > 0 && oldStart+oldCount <= line, oldCount == 0 && oldStart < line:
			shift += newCount - oldCount
		}
	}
	return line + shift, false
}

// hunkRange reads `@@ -a,b +c,d @@` (a count is 1 when left out).
func hunkRange(header string) (oldStart, oldCount, newCount int, ok bool) {
	fields := strings.Fields(header)
	if len(fields) < 3 || !strings.HasPrefix(fields[1], "-") || !strings.HasPrefix(fields[2], "+") {
		return 0, 0, 0, false
	}
	span := func(s string) (int, int, bool) {
		start, count, found := strings.Cut(s[1:], ",")
		a, err := strconv.Atoi(start)
		if err != nil {
			return 0, 0, false
		}
		if !found {
			return a, 1, true
		}
		b, err := strconv.Atoi(count)
		return a, b, err == nil
	}
	oldStart, oldCount, ok1 := span(fields[1])
	_, newCount, ok2 := span(fields[2])
	return oldStart, oldCount, newCount, ok1 && ok2
}

// ReviewDiff is each repo's change in a review: against its main (what a
// merge takes), or with since only what the lead did since the CEO last
// requested changes (a repo never reviewed yet shows against main).
func (w *Workflow) ReviewDiff(ctx context.Context, id string, since bool) ([]RepoDiff, error) {
	if _, err := w.Reviews.Get(ctx, id); err != nil {
		return nil, err
	}
	mrs, err := w.MergeRequests.ForReview(ctx, id)
	if err != nil {
		return nil, err
	}
	out := []RepoDiff{}
	for _, mr := range mrs {
		main, dir, err := w.repoOf(mr)
		if err != nil {
			return nil, err
		}
		d := RepoDiff{Repo: mr.Repo, Main: main}
		// A merged branch has nothing left to show against main; a dismissed one
		// may be gone. Only the open ones have a change to read.
		if mr.State != store.MergeOpen {
			out = append(out, d)
			continue
		}
		if d.Head, err = w.Git.Resolve(ctx, dir, "refs/heads/"+mr.Branch); err != nil {
			return nil, err
		}
		if since && mr.ReviewedHead != "" {
			d.Since = mr.ReviewedHead
			d.Diff, err = w.Git.Compare(ctx, dir, mr.ReviewedHead, d.Head, 3)
		} else {
			d.Diff, err = w.Git.Diff(ctx, dir, main, d.Head)
		}
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, nil
}

// AddComment keeps a draft comment on an open review: on a line of a repo it
// has open (its side, line and the head it was read at), or general (no repo).
func (w *Workflow) AddComment(ctx context.Context, reviewID string, c store.ReviewComment) (store.ReviewComment, error) {
	v, err := w.openReview(ctx, reviewID)
	if err != nil {
		return c, err
	}
	c = store.ReviewComment{ReviewID: reviewID, Repo: c.Repo, Path: c.Path, Side: c.Side, Line: c.Line, Commit: c.Commit, Snippet: c.Snippet, Body: strings.TrimSpace(c.Body)}
	if err := validComment(v, c); err != nil {
		return c, err
	}
	return w.ReviewComments.Save(ctx, c)
}

func validComment(v Review, c store.ReviewComment) error {
	switch {
	case c.Body == "":
		return fmt.Errorf("%w: write the comment", ErrInvalid)
	case teamtools.TooLong(c.Body, teamtools.MaxBody):
		return fmt.Errorf("%w: a comment is at most %d characters", ErrInvalid, teamtools.MaxBody)
	case c.Repo == "":
		if c.Path != "" || c.Line != 0 || c.Side != "" || c.Commit != "" {
			return fmt.Errorf("%w: a general comment names no file", ErrInvalid)
		}
		return nil
	case !slices.ContainsFunc(v.MergeRequests, func(mr store.MergeRequest) bool { return mr.Repo == c.Repo && mr.State == store.MergeOpen }):
		return fmt.Errorf("%w: %s isn't open in this review", ErrInvalid, c.Repo)
	case c.Path == "" || c.Line < 1 || (c.Side != "new" && c.Side != "old") || c.Commit == "":
		return fmt.Errorf("%w: a line comment names its file, side (new or old), line and the commit it was read at", ErrInvalid)
	case len(c.Snippet) > 500:
		return fmt.Errorf("%w: the line's text is at most 500 characters", ErrInvalid)
	}
	return nil
}

// EditComment rewrites a draft comment's text.
func (w *Workflow) EditComment(ctx context.Context, reviewID, id, body string) (store.ReviewComment, error) {
	c, err := w.draft(ctx, reviewID, id)
	if err != nil {
		return c, err
	}
	v, err := w.openReview(ctx, reviewID)
	if err != nil {
		return c, err
	}
	c.Body = strings.TrimSpace(body)
	if err := validComment(v, c); err != nil {
		return c, err
	}
	return w.ReviewComments.Save(ctx, c)
}

// DeleteComment drops a draft comment.
func (w *Workflow) DeleteComment(ctx context.Context, reviewID, id string) error {
	if _, err := w.draft(ctx, reviewID, id); err != nil {
		return err
	}
	return w.ReviewComments.Delete(ctx, id)
}

// draft is a review's draft comment: ErrNotFound for one of another review,
// ErrSent for one sent already.
func (w *Workflow) draft(ctx context.Context, reviewID, id string) (store.ReviewComment, error) {
	c, err := w.ReviewComments.Get(ctx, id)
	switch {
	case err != nil:
		return c, err
	case c.ReviewID != reviewID:
		return c, store.ErrNotFound
	case c.State != store.CommentDraft:
		return c, ErrSent
	}
	return c, nil
}

// openReview is a review with something open to decide: ErrNotOpen otherwise.
func (w *Workflow) openReview(ctx context.Context, id string) (Review, error) {
	r, err := w.Reviews.Get(ctx, id)
	if err != nil {
		return Review{}, err
	}
	v, err := w.view(ctx, r)
	if err != nil {
		return Review{}, err
	}
	if v.Status != ReviewNeedsYou && v.Status != ReviewChangesRequested {
		return v, ErrNotOpen
	}
	return v, nil
}

// RequestChanges sends the review's draft comments, and the CEO's note, to
// the lead in one message, and the review waits on the lead: its request is
// resolved until the lead's next request_merge. The branch heads are kept,
// so that next time the CEO can read only what changed since.
func (w *Workflow) RequestChanges(ctx context.Context, id, note string) error {
	v, err := w.openReview(ctx, id)
	if err != nil {
		return err
	}
	note = strings.TrimSpace(note)
	if teamtools.TooLong(note, teamtools.MaxBody) {
		return fmt.Errorf("%w: the note is at most %d characters", ErrInvalid, teamtools.MaxBody)
	}
	comments, err := w.ReviewComments.ForReview(ctx, id)
	if err != nil {
		return err
	}
	heads := w.heads(ctx, v.MergeRequests)
	var drafts []Comment
	for _, c := range comments {
		if c.State == store.CommentDraft {
			drafts = append(drafts, w.follow(ctx, v.MergeRequests, heads, c))
		}
	}
	if len(drafts) == 0 && note == "" {
		return fmt.Errorf("%w: write a comment or a note for the lead", ErrInvalid)
	}
	var lead store.MergeRequest
	now := time.Now().UTC()
	for _, mr := range v.MergeRequests {
		if mr.State != store.MergeOpen {
			continue
		}
		lead = mr
		if head, ok := heads[mr.Repo]; ok {
			mr.ReviewedHead = head
			w.saveMerge(ctx, mr)
		}
	}
	for _, c := range drafts {
		c.State, c.SentAt = store.CommentSent, now
		if _, err := w.ReviewComments.Save(ctx, c.ReviewComment); err != nil {
			return err
		}
	}
	r := v.Review
	r.ChangesRequestedAt = now
	if _, err := w.Reviews.Save(ctx, r); err != nil {
		return err
	}
	w.resolveReview(ctx, id, "changes requested")
	w.tell(ctx, lead, "ceo", changesMessage(v, note, drafts))
	return nil
}

// changesMessage is what the lead reads: each comment where it is on the
// branch now, then what to do.
func changesMessage(v Review, note string, comments []Comment) string {
	var b strings.Builder
	fmt.Fprintf(&b, "From the CEO: changes requested on %s (%s).", v.Branch, v.Entry)
	if note != "" {
		b.WriteString("\n\n" + note)
	}
	for _, c := range comments {
		b.WriteString("\n\n- ")
		switch {
		case c.Repo == "":
			b.WriteString(c.Body)
			continue
		case c.Side == "old":
			fmt.Fprintf(&b, "%s %s, removed line %d", c.Repo, c.Path, c.Line)
		default:
			fmt.Fprintf(&b, "%s %s:%d", c.Repo, c.Path, c.Now)
		}
		if c.Outdated {
			b.WriteString(" (changed since)")
		}
		if c.Snippet != "" {
			fmt.Fprintf(&b, " `%s`", strings.TrimSpace(c.Snippet))
		}
		b.WriteString(": " + c.Body)
	}
	b.WriteString("\n\nFix them, commit(), then call request_merge again on the same branch.")
	return b.String()
}

// MergeReview merges the review's open merge requests in the repos named,
// each at the head the CEO read (heads: repo → commit), into its repo's
// main; the others stay open in the review. A repo that can't be merged (a
// conflict, the CEO's local changes, a branch that moved since) stops
// nothing else; the error names it.
func (w *Workflow) MergeReview(ctx context.Context, id string, heads map[string]string) error {
	v, err := w.openReview(ctx, id)
	if err != nil {
		return err
	}
	if len(heads) == 0 {
		return fmt.Errorf("%w: pick the repos to merge", ErrInvalid)
	}
	var picked []store.MergeRequest
	for _, repo := range slices.Sorted(maps.Keys(heads)) {
		i := slices.IndexFunc(v.MergeRequests, func(mr store.MergeRequest) bool { return mr.Repo == repo && mr.State == store.MergeOpen })
		if i < 0 {
			return fmt.Errorf("%w: %s has nothing open in this review", ErrNotOpen, repo)
		}
		if heads[repo] == "" {
			return fmt.Errorf("%w: name the commit of %s you read", ErrInvalid, repo)
		}
		picked = append(picked, v.MergeRequests[i])
	}
	var failed []error
	for _, mr := range picked {
		if err := w.mergeToMain(ctx, mr, heads[mr.Repo]); err != nil {
			failed = append(failed, fmt.Errorf("%s: %w", mr.Repo, err))
		}
	}
	return errors.Join(failed...)
}

// DismissReview ends the review unmerged, in every repo still open in it: the
// work isn't wanted (the CEO, 2026-10-21). Its work items leave In flight.
// With StopTeam its deployment stops first (nothing is dismissed when it can't),
// and nobody is told; while it keeps running, the lead is told to stop work
// on it, with the CEO's note. With DropBranches its branches go: the lead
// branch and the members' branches it carried, each kept under
// refs/foreman/dismissed/ (a branch checked out moves to main instead, what its
// worktree held not committed kept too), only while no agent works in them:
// ErrInvalid when the team runs on. A branch that can't go (a git error)
// stays, listed in Dismissed with why.
func (w *Workflow) DismissReview(ctx context.Context, id string, c Dismiss) (Dismissed, error) {
	v, err := w.openReview(ctx, id)
	if err != nil {
		return Dismissed{}, err
	}
	running, err := w.Teams.Active(ctx, v.DeploymentID)
	if err != nil {
		return Dismissed{}, err
	}
	if c.DropBranches && running && !c.StopTeam {
		return Dismissed{}, fmt.Errorf("%w: stop the team to drop its branches: its agents work in them", ErrInvalid)
	}
	if running && c.StopTeam {
		if err := w.Teams.Stop(ctx, v.DeploymentID); err != nil {
			return Dismissed{}, fmt.Errorf("stopping the team: %w", err)
		}
	}
	note := strings.TrimSpace(c.Note)
	dismissed := Dismissed{Kept: []KeptBranch{}} // [] on the wire, never null
	var lead store.MergeRequest
	var repos []string
	for _, mr := range v.MergeRequests {
		if mr.State != store.MergeOpen {
			continue
		}
		w.reject(ctx, mr, note)
		lead, repos = mr, append(repos, mr.Repo)
		if !c.DropBranches {
			continue
		}
		for _, branch := range w.branchesOf(ctx, mr) {
			if err := w.dropBranch(ctx, mr, branch, id); err != nil {
				dismissed.Kept = append(dismissed.Kept, KeptBranch{Repo: mr.Repo, Branch: branch, Why: err.Error()})
			}
		}
	}
	w.resolveReview(ctx, id, "dismissed")
	if running && !c.StopTeam {
		text := fmt.Sprintf("From Foreman: the CEO dismissed the review of %s without merging (%s). Stop work on it, and don't request it again.", v.Branch, strings.Join(repos, ", "))
		if note != "" {
			text += " The CEO's note: " + note
		}
		w.tell(ctx, lead, "foreman", text)
	}
	return dismissed, nil
}

// branchesOf are a merge request's branches in its repo: the lead branch,
// then the members' branches its work items carry.
func (w *Workflow) branchesOf(ctx context.Context, mr store.MergeRequest) []string {
	_, dir, err := w.repoOf(mr)
	if err != nil {
		return nil
	}
	branches := []string{mr.Branch}
	items, _ := w.WorkItems.ForDeployment(ctx, mr.DeploymentID)
	for _, item := range items {
		if item.MergeRequestID == mr.ID && !slices.Contains(branches, item.Branch) && w.Git.HasBranch(ctx, dir, item.Branch) {
			branches = append(branches, item.Branch)
		}
	}
	return branches
}

// dropBranch discards one of a dismissed review's branches in a merge
// request's repo, keeping its head under refs/foreman/dismissed/<time>/<review>/.
func (w *Workflow) dropBranch(ctx context.Context, mr store.MergeRequest, branch, review string) error {
	main, dir, err := w.repoOf(mr)
	if err != nil {
		return err
	}
	defer w.pruneBackups(ctx, dir)
	return w.Git.DropBranch(ctx, dir, branch, git.BackupRef("dismissed", w.Now(), review+"/"+branch), main)
}
