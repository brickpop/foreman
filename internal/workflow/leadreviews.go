package workflow

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/brickpop/foreman/internal/agentlink"
	"github.com/brickpop/foreman/internal/store"
	"github.com/brickpop/foreman/internal/teamtools"
)

// The lead's reviews(): where its reviews stand, read again whenever its
// session lost the notices (compacted, restarted). Read-only, and built from
// what the CEO's review page reads, so the two can't disagree
// (wiki/product/architecture/protocol.md#reviews).

// recentlyDecided is how long a decided review is still listed.
const recentlyDecided = 7 * 24 * time.Hour

// reviews lists the caller's reviews on its project: the open ones, and those
// decided in the last 7 days, newest first.
func (t tool) reviews(ctx context.Context) agentlink.Result {
	if !t.c.Lead {
		return teamtools.Refusal("not_lead", "only the lead has reviews")
	}
	list, err := t.w.ListReviews(ctx)
	if err != nil {
		return teamtools.Refusal("internal", "%v", err)
	}
	since := t.w.Now().Add(-recentlyDecided)
	var each []string
	for _, v := range list {
		open := v.Status == ReviewNeedsYou || v.Status == ReviewChangesRequested
		if v.DeploymentID != t.c.Deployment.ID || (!open && v.UpdatedAt.Before(since)) {
			continue
		}
		text, err := t.reviewText(ctx, v)
		if err != nil {
			return teamtools.Refusal("internal", "%v", err)
		}
		each = append(each, text)
	}
	if len(each) == 0 {
		return agentlink.Result{Text: "You have no review open, and none decided in the last 7 days."}
	}
	return agentlink.Result{Text: "Your reviews, open or decided in the last 7 days, newest first:\n\n" + strings.Join(each, "\n\n")}
}

// reviewText is one review: its branch and entry, each repo's state, why it
// was withdrawn, and while changes are requested, the CEO's note and the
// comments sent with it, each where it is now (general ones last).
func (t tool) reviewText(ctx context.Context, v Review) (string, error) {
	var b strings.Builder
	fmt.Fprintf(&b, "%s %s: %s (%s)", v.ID, v.Branch, v.Title, v.Entry)
	var repos []string
	for _, mr := range v.MergeRequests {
		repos = append(repos, mr.Repo+" → "+t.targetOf(mr)+": "+t.repoState(v, mr))
	}
	b.WriteString("\n  " + strings.Join(repos, " · "))
	if v.Status == ReviewWithdrawn {
		b.WriteString("\n  Withdrawn: " + v.WithdrawnReason)
		if v.SupersededBy != "" {
			b.WriteString(", superseded by " + v.SupersededBy)
		}
		b.WriteString(".")
	}
	if v.Status != ReviewChangesRequested {
		return b.String(), nil
	}
	full, err := t.w.Review(ctx, v.ID)
	if err != nil {
		return "", err
	}
	if v.ChangesNote != "" {
		b.WriteString("\n  The CEO's note: " + v.ChangesNote)
	}
	var lines, general []string
	for _, c := range full.Comments {
		if c.State != store.CommentSent || !c.SentAt.Equal(v.ChangesRequestedAt) {
			continue // a draft, or sent with an earlier Request changes
		}
		if c.Repo == "" {
			general = append(general, commentText(c))
		} else {
			lines = append(lines, commentText(c))
		}
	}
	for _, c := range append(lines, general...) {
		b.WriteString("\n  - " + c)
	}
	return b.String(), nil
}

// repoState is how one repo of a review stands.
func (t tool) repoState(v Review, mr store.MergeRequest) string {
	switch mr.State {
	case store.MergeMerged:
		return "merged"
	case store.MergeWithdrawn:
		return "withdrawn"
	case store.MergeRejected:
		if mr.Note != "" {
			return "dismissed (the CEO's note: " + mr.Note + ")"
		}
		return "dismissed"
	}
	switch {
	case mr.Conflicting:
		return "conflicts"
	case v.Status == ReviewChangesRequested:
		return "changes requested"
	}
	return "waiting for the CEO"
}

// targetOf is the branch a merge request merges into: its repo's default
// when it names none.
func (t tool) targetOf(mr store.MergeRequest) string {
	if mr.Target == "" {
		return t.project.Default(mr.Repo)
	}
	return mr.Target
}
