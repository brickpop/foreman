package workflow

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/brickpop/foreman/internal/store"
)

func TestReviewsRefusalsAndNone(t *testing.T) {
	f := newFixture(t)
	f.refused("tove", "reviews", map[string]string{}, "not_lead")
	if text := f.ok("mara", "reviews", map[string]string{}); text != "You have no review open, and none decided in the last 7 days." {
		t.Errorf("%q", text)
	}
}

// The lead reads its reviews as they stand: each repo's state, why one was
// withdrawn or dismissed, and for changes requested only the last round's
// note and comments, line ones first, each where it is now.
func TestReviewsTellTheLeadWhereItsReviewsStand(t *testing.T) {
	f := newFixture(t)
	f.smallAndBig() // rev-1 small, rev-2 big, both in api and web
	head := f.git(f.wt("mara", "api"), "rev-parse", "HEAD")
	line := store.ReviewComment{Repo: "api", Path: "big.go", Side: "new", Line: 1, Commit: head, Snippet: "package big", Body: "Name it bigger"}

	// A first round, then a second: only the second shows.
	if _, err := f.w.AddComment(ctx, "rev-2", store.ReviewComment{Body: "an old round"}); err != nil {
		t.Fatal(err)
	}
	if err := f.w.RequestChanges(ctx, "rev-2", "first note"); err != nil {
		t.Fatal(err)
	}
	f.ok("mara", "request_merge", map[string]string{"branch": "lead/ledger/atlas/big", "title": "Invoices as CSV", "entry": "backlog/led-44.md"})
	if _, err := f.w.AddComment(ctx, "rev-2", store.ReviewComment{Body: "Add a test"}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.w.AddComment(ctx, "rev-2", line); err != nil {
		t.Fatal(err)
	}
	if err := f.w.RequestChanges(ctx, "rev-2", "Nearly there."); err != nil {
		t.Fatal(err)
	}
	if _, err := f.w.AddComment(ctx, "rev-2", store.ReviewComment{Body: "a draft, unsent"}); err != nil {
		t.Fatal(err)
	}
	// The lead changes that line since: the comment is outdated. And web conflicts.
	writeFile(t, filepath.Join(f.wt("mara", "api"), "big.go"), "package bigger\n")
	f.ok("mara", "commit", commits("api", "rename"))
	web := f.mr("mr-4")
	web.Conflicting = true
	if _, err := f.db.MergeRequests().Save(ctx, web); err != nil {
		t.Fatal(err)
	}
	// small is dismissed with a note.
	if _, err := f.w.DismissReview(ctx, "rev-1", Dismiss{Note: "not now"}); err != nil {
		t.Fatal(err)
	}

	text := f.ok("mara", "reviews", map[string]string{})
	want := "Your reviews, open or decided in the last 7 days, newest first:\n\n" +
		"rev-2 lead/ledger/atlas/big: Invoices as CSV (backlog/led-44.md)\n" +
		"  api → main: changes requested · web → trunk: conflicts\n" +
		"  The CEO's note: Nearly there.\n" +
		"  - api big.go:1 (changed since) `package big`: Name it bigger\n" +
		"  - Add a test\n\n" +
		"rev-1 lead/ledger/atlas/small: Invoices as CSV (backlog/led-44.md)\n" +
		"  api → main: dismissed (the CEO's note: not now) · web → trunk: dismissed (the CEO's note: not now)"
	if text != want {
		t.Errorf("got\n%s\nwant\n%s", text, want)
	}
}

// Only the lead's own: another team's review on the project, and one decided
// more than 7 days ago, aren't listed; a withdrawn one says why.
func TestReviewsListOnlyTheLeadsRecentOnes(t *testing.T) {
	f := newFixture(t)
	f.smallAndBig()
	f.ok("mara", "withdraw_merge", map[string]string{"branch": "lead/ledger/atlas/small", "reason": "all of it is in big", "superseded_by": "lead/ledger/atlas/big"})
	old := time.Date(2026, 9, 1, 9, 0, 0, 0, time.UTC) // the fixture's today is 2026-10-16
	review := func(deployment, branch, state string, at time.Time) {
		r, err := f.db.Reviews().Save(ctx, store.Review{ProjectID: "ledger", Branch: branch})
		if err != nil {
			t.Fatal(err)
		}
		r.CreatedAt = at
		if _, err := f.db.Reviews().Save(ctx, r); err != nil {
			t.Fatal(err)
		}
		mr, err := f.db.MergeRequests().Save(ctx, store.MergeRequest{ProjectID: "ledger", DeploymentID: deployment, Repo: "api", Branch: branch, State: state, ReviewID: r.ID})
		if err != nil {
			t.Fatal(err)
		}
		mr.CreatedAt, mr.MergedAt = at, at
		if _, err := f.db.MergeRequests().Save(ctx, mr); err != nil {
			t.Fatal(err)
		}
	}
	// Orion worked on ledger before atlas came back: its reviews are its own.
	deps := f.db.Deployments()
	if err := deps.Stop(ctx, "atlas@ledger", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := deps.Deploy(ctx, "orion", "ledger"); err != nil {
		t.Fatal(err)
	}
	if err := deps.Stop(ctx, "orion@ledger", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := deps.Deploy(ctx, "atlas", "ledger"); err != nil {
		t.Fatal(err)
	}
	review("orion@ledger", "lead/ledger/orion/x", store.MergeOpen, time.Now())
	review("atlas@ledger", "lead/ledger/atlas/long-ago", store.MergeMerged, old)
	review("atlas@ledger", "lead/ledger/atlas/still-open", store.MergeOpen, old)

	text := f.ok("mara", "reviews", map[string]string{})
	for _, absent := range []string{"lead/ledger/orion/x", "long-ago"} {
		if strings.Contains(text, absent) {
			t.Errorf("lists %s:\n%s", absent, text)
		}
	}
	for _, want := range []string{
		"lead/ledger/atlas/still-open", // open, however old
		"rev-1 lead/ledger/atlas/small: Invoices as CSV (backlog/led-44.md)\n  api → main: withdrawn · web → trunk: withdrawn\n  Withdrawn: all of it is in big, superseded by rev-2.",
		"rev-2 lead/ledger/atlas/big: Invoices as CSV (backlog/led-44.md)\n  api → main: waiting for the CEO · web → trunk: waiting for the CEO",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q in:\n%s", want, text)
		}
	}
}
