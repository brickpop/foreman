package workflow

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/brickpop/foreman/internal/store"
)

// smallAndBig: tove's wt/tove/a (api and web) handed off and merged by mara
// into lead/atlas/small, requested first (rev-1: mr-1 api, mr-2 web);
// lead/atlas/big made from main, with small merged in and more in api,
// requested next (rev-2: mr-3 api, mr-4 web).
func (f *fixture) smallAndBig() {
	f.t.Helper()
	f.handOff("wt/tove/a", "api", "web")
	f.ok("mara", "switch", map[string]string{"branch": "lead/atlas/small"})
	f.ok("mara", "merge", map[string]string{"branch": "wt/tove/a"})
	f.ok("mara", "request_merge", map[string]string{"branch": "lead/atlas/small", "entry": "backlog/led-44.md"})
	f.ok("mara", "switch", map[string]string{"branch": "lead/atlas/big"})
	f.ok("mara", "merge", map[string]string{"branch": "lead/atlas/small"})
	writeFile(f.t, filepath.Join(f.wt("mara", "api"), "big.go"), "package big\n")
	f.ok("mara", "commit", commits("api", "more"))
	f.ok("mara", "request_merge", map[string]string{"branch": "lead/atlas/big", "entry": "backlog/led-44.md"})
	if f.item("wt/tove/a").MergeRequestID != "mr-1" {
		f.t.Fatalf("set up: %+v", f.item("wt/tove/a"))
	}
}

// withdrawn is the last review.withdrawn event.
func (f *fixture) withdrawn() Withdrawn {
	f.t.Helper()
	for i := len(f.events) - 1; i >= 0; i-- {
		if f.events[i].Type == "review.withdrawn" {
			return f.events[i].Data.(Withdrawn)
		}
	}
	f.t.Fatal("no review.withdrawn")
	return Withdrawn{}
}

func TestWithdrawRefusals(t *testing.T) {
	f := newFixture(t)
	f.smallAndBig()
	f.ok("mara", "switch", map[string]string{"branch": "lead/atlas/other"})
	writeFile(t, filepath.Join(f.wt("mara", "api"), "other.go"), "package other\n")
	f.ok("mara", "commit", commits("api", "other"))
	f.ok("mara", "request_merge", map[string]string{"branch": "lead/atlas/other", "entry": "backlog/led-44.md"})
	w := func(branch, reason, by string) map[string]string {
		return map[string]string{"branch": branch, "reason": reason, "superseded_by": by}
	}
	f.refused("tove", "withdraw_merge", w("lead/atlas/small", "why", ""), "not_lead")
	f.refused("mara", "withdraw_merge", w("wt/tove/a", "why", ""), "branch_not_allowed")
	f.refused("mara", "withdraw_merge", w("lead/orion/x", "why", ""), "branch_not_allowed")
	f.refused("mara", "withdraw_merge", w("lead/atlas/small", "why", "lead/orion/x"), "branch_not_allowed")
	f.refused("mara", "withdraw_merge", w("lead/atlas/small", "", ""), "no_reason")
	f.refused("mara", "withdraw_merge", w("lead/atlas/small", strings.Repeat("x", 10_000), ""), "too_long")
	f.refused("mara", "withdraw_merge", w("lead/atlas/small", "why", "lead/atlas/small"), "superseded_by_itself")
	f.refused("mara", "withdraw_merge", w("lead/atlas/integration", "why", ""), "not_open")
	f.refused("mara", "withdraw_merge", w("lead/atlas/small", "why", "lead/atlas/integration"), "superseding_not_open")
	// other has a review open, but holds nothing of small; nor does small hold big.
	f.refused("mara", "withdraw_merge", w("lead/atlas/small", "why", "lead/atlas/other"), "not_superseded")
	f.refused("mara", "withdraw_merge", w("lead/atlas/big", "why", "lead/atlas/small"), "not_superseded")
	for _, id := range []string{"rev-1", "rev-2", "rev-3"} {
		if v := f.review(id); v.Status != ReviewNeedsYou {
			t.Errorf("%s after refusals: %s", id, v.Status)
		}
	}
}

func TestTheLeadWithdrawsAReviewAnotherHolds(t *testing.T) {
	f := newFixture(t)
	f.smallAndBig()
	if _, err := f.w.AddComment(ctx, "rev-1", store.ReviewComment{Body: "a draft"}); err != nil {
		t.Fatal(err)
	}
	text := f.ok("mara", "withdraw_merge", map[string]string{"branch": "lead/atlas/small", "reason": "all of it is in big", "superseded_by": "lead/atlas/big"})
	if !strings.HasPrefix(text, "Withdrew review rev-1 of lead/atlas/small (api, web): it left the CEO's queue. The branch stays. The CEO sees it superseded by rev-2 (lead/atlas/big).") {
		t.Errorf("%q", text)
	}
	v := f.review("rev-1")
	if v.Status != ReviewWithdrawn || v.WithdrawnReason != "all of it is in big" || v.SupersededBy != "rev-2" || v.WithdrawnAt.IsZero() || len(v.Comments) != 0 {
		t.Errorf("review %+v", v)
	}
	for _, id := range []string{"mr-1", "mr-2"} {
		if mr := f.mr(id); mr.State != store.MergeWithdrawn || mr.Note != "all of it is in big" {
			t.Errorf("%s %+v", id, mr)
		}
	}
	// Out of the CEO's queue; the hand-off goes with big, which holds it.
	if _, err := f.db.Requests().ForReview(ctx, "rev-1"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("its request is still open: %v", err)
	}
	if _, err := f.db.Requests().ForReview(ctx, "rev-2"); err != nil {
		t.Errorf("big's request: %v", err)
	}
	if w := f.item("wt/tove/a"); w.MergeRequestID != "mr-3" || w.Stage != store.StageIntegrated {
		t.Errorf("work item %+v", w)
	}
	if e := f.withdrawn(); e.Review.ID != "rev-1" || e.By != "lead" || e.Lead != "mara@ledger" || e.DeploymentID != "atlas@ledger" || e.LostDrafts != 1 {
		t.Errorf("event %+v", e)
	}
	if f.git(filepath.Join(f.root, "ledger", "api"), "branch", "--list", "lead/atlas/small") == "" {
		t.Error("the branch went")
	}
	// Nothing open to decide any more.
	if _, err := f.w.AddComment(ctx, "rev-1", store.ReviewComment{Body: "late"}); !errors.Is(err, ErrNotOpen) {
		t.Errorf("a comment on it: %v", err)
	}
	f.refused("mara", "withdraw_merge", map[string]string{"branch": "lead/atlas/small", "reason": "again"}, "not_open")
}

func TestAWithdrawnReviewsHandOffsGoWithTheNextRequest(t *testing.T) {
	f := newFixture(t)
	f.integrated()
	f.ok("mara", "request_merge", integration)
	f.ok("mara", "withdraw_merge", map[string]string{"branch": "lead/atlas/integration", "reason": "the approach changed"})
	if v := f.review("rev-1"); v.Status != ReviewWithdrawn || v.SupersededBy != "" {
		t.Errorf("review %+v", v)
	}
	// Freed, still integrated: its commits are on the lead branch.
	if w := f.item("wt/tove/LED-44"); w.MergeRequestID != "" || w.Stage != store.StageIntegrated {
		t.Errorf("work item %+v", w)
	}
	// Asked again, it's a new review, and takes it.
	if text := f.ok("mara", "request_merge", integration); text != "Review rev-2, api (mr-2): waiting for the CEO." {
		t.Errorf("%q", text)
	}
	if w := f.item("wt/tove/LED-44"); w.MergeRequestID != "mr-2" {
		t.Errorf("work item %+v", w)
	}
}

func TestMergingAReviewWithdrawsTheOnesItHolds(t *testing.T) {
	f := newFixture(t)
	f.smallAndBig()
	if _, err := f.w.AddComment(ctx, "rev-1", store.ReviewComment{Body: "a draft"}); err != nil {
		t.Fatal(err)
	}
	// Only api first: small's web isn't in trunk yet, so small stays open.
	sent := len(f.m.sent)
	if err := f.w.MergeReview(ctx, "rev-2", f.shown("rev-2", "api")); err != nil {
		t.Fatal(err)
	}
	if v := f.review("rev-1"); v.Status != ReviewNeedsYou {
		t.Fatalf("withdrawn with web still to merge: %+v", v)
	}
	if news := f.m.sent[sent:]; len(news) != 2 || !strings.Contains(news[1].Body, "Merge it into lead/atlas/small") {
		t.Errorf("notices %+v", news)
	}
	// Then web: main holds all of small now.
	sent = len(f.m.sent)
	if err := f.w.MergeReview(ctx, "rev-2", f.shown("rev-2", "web")); err != nil {
		t.Fatal(err)
	}
	v := f.review("rev-1")
	if v.Status != ReviewWithdrawn || v.SupersededBy != "rev-2" || v.WithdrawnReason != "lead/atlas/big, merged, holds all of it" {
		t.Errorf("review %+v", v)
	}
	if f.review("rev-2").Status != ReviewMerged {
		t.Error("big isn't merged")
	}
	if _, err := f.db.Requests().ForReview(ctx, "rev-1"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("its request is still open: %v", err)
	}
	// Its hand-off (carried by small's api) went in with big's api.
	if w := f.item("wt/tove/a"); w.MergeRequestID != "mr-3" {
		t.Errorf("work item %+v", w)
	}
	if e := f.withdrawn(); e.Review.ID != "rev-1" || e.By != "foreman" || e.LostDrafts != 1 {
		t.Errorf("event %+v", e)
	}
	// The lead hears big merged and small withdrawn; nobody to merge main into small.
	news := f.m.sent[sent:]
	if len(news) != 2 || !strings.Contains(news[1].Body, "the CEO merged lead/atlas/big, which holds all of lead/atlas/small, so Foreman withdrew the review of lead/atlas/small (rev-1, api, web). Don't request it again.") {
		t.Errorf("notices %+v", news)
	}
}

// The CEO may merge the smaller one: the bigger holds more, so it stays.
func TestMergingTheSmallerKeepsTheBigger(t *testing.T) {
	f := newFixture(t)
	f.smallAndBig()
	if err := f.w.MergeReview(ctx, "rev-1", f.shown("rev-1", "api", "web")); err != nil {
		t.Fatal(err)
	}
	if v := f.review("rev-2"); v.Status != ReviewNeedsYou || !v.WithdrawnAt.IsZero() {
		t.Errorf("big %+v", v)
	}
	if mr := f.mr("mr-3"); mr.State != store.MergeOpen {
		t.Errorf("big's api %+v", mr)
	}
}

// Merged outside Foreman, the older review that holds a newer one is
// noticed first: the newer, withdrawn, isn't then taken for merged.
func TestAMergeOutsideForemanWithdrawsTheReviewsItHolds(t *testing.T) {
	f := newFixture(t)
	f.handOff("wt/tove/a", "api")
	f.ok("mara", "switch", map[string]string{"branch": "lead/atlas/small"})
	f.ok("mara", "merge", map[string]string{"branch": "wt/tove/a"})
	f.ok("mara", "switch", map[string]string{"branch": "lead/atlas/big"})
	f.ok("mara", "merge", map[string]string{"branch": "lead/atlas/small"})
	writeFile(t, filepath.Join(f.wt("mara", "api"), "big.go"), "package big\n")
	f.ok("mara", "commit", commits("api", "more"))
	f.ok("mara", "request_merge", map[string]string{"branch": "lead/atlas/big", "entry": "backlog/led-44.md"})
	f.ok("mara", "request_merge", map[string]string{"branch": "lead/atlas/small", "entry": "backlog/led-44.md"})
	api := filepath.Join(f.root, "ledger", "api")
	f.git(api, "merge", "-q", "--squash", "lead/atlas/big")
	f.git(api, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "-m", "by hand")
	f.w.Reconcile(ctx)
	if mr := f.mr("mr-1"); mr.State != store.MergeMerged {
		t.Errorf("big %+v", mr)
	}
	if mr := f.mr("mr-2"); mr.State != store.MergeWithdrawn {
		t.Errorf("small %+v", mr)
	}
	if v := f.review("rev-2"); v.Status != ReviewWithdrawn || v.SupersededBy != "rev-1" {
		t.Errorf("small's review %+v", v)
	}
}

// A decided review still shows its change, as it was decided, however it
// ended: merged (its branch then moves to the new main), dismissed with its
// branches dropped, or withdrawn. Without a record it says so.
func TestADecidedReviewKeepsItsDiff(t *testing.T) {
	for _, end := range []string{"merged", "dismissed", "withdrawn"} {
		t.Run(end, func(t *testing.T) {
			f := newFixture(t)
			f.integrated()
			f.ok("mara", "request_merge", integration)
			before, err := f.w.ReviewDiff(ctx, "rev-1", false)
			if err != nil || len(before) != 1 || !strings.Contains(before[0].Diff, "LED-44.go") {
				t.Fatalf("before %+v %v", before, err)
			}
			switch end {
			case "merged":
				err = f.w.MergeReview(ctx, "rev-1", f.shown("rev-1", "api"))
			case "dismissed":
				_, err = f.w.DismissReview(ctx, "rev-1", Dismiss{DropBranches: true, StopTeam: true})
			case "withdrawn":
				f.ok("mara", "withdraw_merge", map[string]string{"branch": "lead/atlas/integration", "reason": "not needed"})
			}
			if err != nil {
				t.Fatal(err)
			}
			after, err := f.w.ReviewDiff(ctx, "rev-1", false)
			if err != nil || len(after) != 1 || after[0].Gone || after[0].Diff != before[0].Diff || after[0].Head != before[0].Head {
				t.Errorf("after %+v %v", after, err)
			}
			api := filepath.Join(f.root, "ledger", "api")
			if kept := f.git(api, "rev-parse", "refs/foreman/decided/mr-1"); kept != before[0].Head {
				t.Errorf("kept %q, want %s", kept, before[0].Head)
			}
			// Recorded before this was built, or git lost it: it says so.
			mr := f.mr("mr-1")
			mr.Head = ""
			if _, err := f.db.MergeRequests().Save(ctx, mr); err != nil {
				t.Fatal(err)
			}
			if gone, _ := f.w.ReviewDiff(ctx, "rev-1", false); len(gone) != 1 || !gone[0].Gone || gone[0].Diff != "" {
				t.Errorf("unrecorded %+v", gone)
			}
		})
	}
}
