package workflow

import (
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/brickpop/foreman/internal/git"
	"github.com/brickpop/foreman/internal/store"
)

// runGit runs git in dir, for a command that may fail.
func runGit(dir string, args ...string) (string, error) {
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

// Where a merge goes (wiki/product/features/pipeline-and-merges.md#where-a-merge-goes):
// any branch of the CEO's, none listed anywhere.

// release has the CEO make release/1.0 in api at its first commit, with git,
// then move api's main on (NEWS), so work for the release and work for main
// differ.
func (f *fixture) release() {
	f.t.Helper()
	api := filepath.Join(f.root, "ledger", "api")
	f.git(api, "branch", "release/1.0", "main")
	writeFile(f.t, filepath.Join(api, "NEWS"), "v2\n")
	if err := git.New().Commit(ctx, api, "news", "NEWS"); err != nil {
		f.t.Fatal(err)
	}
}

// Work for a branch of the CEO's starts from it and lands on it: the review
// is reckoned against it, the merge goes there, idle desks stay on the
// default, and the notice that it moved goes only to merge requests into it.
func TestWorkLandsOnTheBranchItNames(t *testing.T) {
	f := newFixture(t)
	f.release()
	api := filepath.Join(f.root, "ledger", "api")
	desk := f.git(api, "rev-parse", "wt/ledger/tove/main")

	text := f.ok("mara", "switch", map[string]string{"branch": "lead/ledger/atlas/fix", "from": "release/1.0"})
	if text != "Switched api, web to lead/ledger/atlas/fix. New in api (from release/1.0), web (from trunk)." {
		t.Errorf("switch: %q", text)
	}
	writeFile(t, filepath.Join(f.wt("mara", "api"), "fix.go"), "package fix\n")
	f.ok("mara", "commit", commits("api", "the fix"))
	// Another lead branch, into main, open meanwhile.
	f.ok("mara", "switch", map[string]string{"branch": "lead/ledger/atlas/other"})
	writeFile(t, filepath.Join(f.wt("mara", "api"), "other.go"), "package other\n")
	f.ok("mara", "commit", commits("api", "other"))
	f.ok("mara", "request_merge", map[string]string{"branch": "lead/ledger/atlas/other", "title": "Invoices as CSV", "entry": "backlog/led-44.md"})

	fix := map[string]string{"branch": "lead/ledger/atlas/fix", "title": "Invoices as CSV", "entry": "backlog/led-44.md", "into": "release/1.0"}
	if text := f.ok("mara", "request_merge", fix); text != "Review rev-2, api (mr-2) into release/1.0: waiting for the CEO." {
		t.Fatalf("request: %q", text)
	}
	diffs, err := f.w.ReviewDiff(ctx, "rev-2", false)
	if err != nil || len(diffs) != 1 || diffs[0].Target != "release/1.0" || !strings.Contains(diffs[0].Diff, "fix.go") || strings.Contains(diffs[0].Diff, "NEWS") {
		t.Fatalf("diff %+v %v", diffs, err)
	}
	if v := f.review("rev-2"); len(v.MergeRequests) != 1 || v.MergeRequests[0].Target != "release/1.0" {
		t.Errorf("review %+v", v.MergeRequests)
	}

	before := len(f.m.sent)
	if err := f.w.MergeReview(ctx, "rev-2", f.shown("rev-2", "api")); err != nil {
		t.Fatal(err)
	}
	if out := f.git(api, "show", "release/1.0:fix.go"); out != "package fix" {
		t.Errorf("release/1.0 has %q", out)
	}
	if out, err := runGit(api, "show", "main:fix.go"); err == nil {
		t.Errorf("main got the fix too: %q", out)
	}
	if at := f.git(api, "rev-parse", "wt/ledger/tove/main"); at != desk {
		t.Error("an idle desk followed a branch other than the default")
	}
	if f.mr("mr-2").State != store.MergeMerged || f.mr("mr-1").State != store.MergeOpen {
		t.Errorf("states %s %s", f.mr("mr-2").State, f.mr("mr-1").State)
	}
	for _, m := range f.m.sent[before:] {
		if strings.Contains(m.Body, "release/1.0 moved") {
			t.Errorf("told a merge request into main that release/1.0 moved: %q", m.Body)
		}
		if strings.Contains(m.Body, "was merged into") && !strings.Contains(m.Body, "lead/ledger/atlas/fix was merged into release/1.0 in api") {
			t.Errorf("notice %q", m.Body)
		}
	}
}

// A review spans every repo with new commits: into must be a branch of each,
// never an agent's, or nothing is made. A refresh keeps the branch it goes
// to; a new into changes it.
func TestIntoMustBeABranchOfEveryRepoWithNewCommits(t *testing.T) {
	f := newFixture(t)
	f.release()
	f.ok("mara", "switch", map[string]string{"branch": "lead/ledger/atlas/fix", "from": "release/1.0"})
	for _, r := range []string{"api", "web"} {
		writeFile(t, filepath.Join(f.wt("mara", r), "fix.go"), "package fix\n")
	}
	f.ok("mara", "commit", commits("api", "fix", "web", "fix"))
	fix := map[string]string{"branch": "lead/ledger/atlas/fix", "title": "Invoices as CSV", "entry": "backlog/led-44.md", "into": "release/1.0"}
	if text := f.refused("mara", "request_merge", fix, "no_such_branch"); !strings.Contains(text, "in web (no release/1.0), which lacks the branch") {
		t.Errorf("%q", text)
	}
	if all, _ := f.db.MergeRequests().ForProject(ctx, "ledger"); len(all) != 0 {
		t.Errorf("made %+v", all)
	}
	if list, _ := f.db.Reviews().List(ctx); len(list) != 0 {
		t.Errorf("a review was made: %+v", list)
	}
	fix["into"] = "nope"
	f.refused("mara", "request_merge", fix, "no_such_branch")
	// An agent's branch, which every repo has, is never one to merge into.
	for _, into := range []string{"wt/ledger/tove/main", "lead/ledger/atlas/integration"} {
		fix["into"] = into
		f.refused("mara", "request_merge", fix, "agent_branch")
	}

	// Into the defaults, then the CEO moves api's to the release: a refresh
	// keeps it there.
	delete(fix, "into")
	f.ok("mara", "request_merge", fix)
	if err := f.w.RetargetReview(ctx, "rev-1", "api", "release/1.0"); err != nil {
		t.Fatal(err)
	}
	if text := f.ok("mara", "request_merge", fix); text != "Review rev-1, api (mr-1) into release/1.0, refreshed, web (mr-2) into trunk, refreshed: waiting for the CEO." {
		t.Errorf("refresh: %q", text)
	}
	fix["into"] = "main"
	f.refused("mara", "request_merge", fix, "no_such_branch") // web has no main

	// A branch the CEO makes in web too, after: it can be named at once.
	f.git(filepath.Join(f.root, "ledger", "web"), "branch", "release/1.0", "trunk")
	fix["into"] = "release/1.0"
	if text := f.ok("mara", "request_merge", fix); text != "Review rev-1, api (mr-1) into release/1.0, refreshed, web (mr-2) into release/1.0, refreshed: waiting for the CEO." {
		t.Errorf("into a branch made after: %q", text)
	}
}

// The CEO changes where one repo of a review merges: to any branch of the
// repo but an agent's; the diff follows, a conflict found before is forgotten, and
// the lead is told.
func TestRetargetReview(t *testing.T) {
	f := newFixture(t)
	f.release()
	f.integrated()
	f.ok("mara", "request_merge", integration)
	mr := f.mr("mr-1")
	mr.Conflicting, mr.Note = true, "conflicts with main"
	if _, err := f.db.MergeRequests().Save(ctx, mr); err != nil {
		t.Fatal(err)
	}
	for name, c := range map[string]struct {
		repo, target string
		want         error
	}{
		"an agent's branch":    {"api", "wt/ledger/tove/main", ErrInvalid},
		"no such branch":       {"api", "nope", ErrInvalid},
		"another repo's":       {"web", "release/1.0", ErrNotOpen},
		"a repo of no request": {"docs", "main", ErrNotOpen},
	} {
		if err := f.w.RetargetReview(ctx, "rev-1", c.repo, c.target); !errors.Is(err, c.want) {
			t.Errorf("%s: %v, want %v", name, err, c.want)
		}
	}
	if err := f.w.RetargetReview(ctx, "rev-1", "api", "release/1.0"); err != nil {
		t.Fatal(err)
	}
	if mr := f.mr("mr-1"); mr.Target != "release/1.0" || mr.Conflicting || mr.Note != "" {
		t.Errorf("merge request %+v", mr)
	}
	if n := f.lastNotice(); n.ToInstance != "mara@ledger" || !strings.Contains(n.Body, "the CEO changed where lead/ledger/atlas/integration merges in api: into release/1.0, no longer main") {
		t.Errorf("notice %+v", n)
	}
	if diffs, _ := f.w.ReviewDiff(ctx, "rev-1", false); diffs[0].Target != "release/1.0" || !strings.Contains(diffs[0].Diff, "LED-44.go") {
		t.Errorf("diff %+v", diffs)
	}
	// A branch deleted outside Foreman.
	f.git(filepath.Join(f.root, "ledger", "api"), "branch", "-D", "release/1.0")
	if err := f.w.RetargetReview(ctx, "rev-1", "api", "main"); err != nil {
		t.Errorf("back to main: %v", err)
	}
	if err := f.w.RetargetReview(ctx, "rev-1", "api", "release/1.0"); !errors.Is(err, ErrInvalid) {
		t.Errorf("to a branch that's gone: %v", err)
	}
}

// switch from: any branch or commit at least one repo has, used only for a
// new branch.
func TestSwitchFromAnywhere(t *testing.T) {
	f := newFixture(t)
	f.release()
	f.refused("tove", "switch", map[string]string{"branch": "wt/ledger/tove/x", "from": "nope"}, "no_such_ref")
	text := f.ok("tove", "switch", map[string]string{"branch": "wt/ledger/tove/x", "from": "release/1.0"})
	if !strings.Contains(text, "New in api (from release/1.0), web (from trunk).") {
		t.Errorf("%q", text)
	}
	if _, err := runGit(f.wt("tove", "api"), "show", "HEAD:NEWS"); err == nil {
		t.Error("started from main, not the release")
	}
	f.ok("tove", "switch", map[string]string{"branch": "wt/ledger/tove/main"})
	if text := f.ok("tove", "switch", map[string]string{"branch": "wt/ledger/tove/x", "from": "release/1.0"}); !strings.Contains(text, "It existed already, so from (release/1.0) changed nothing.") {
		t.Errorf("again: %q", text)
	}
	// From a teammate's branch, and from a commit only api has.
	writeFile(t, filepath.Join(f.wt("mara", "api"), "lead.go"), "package l\n")
	f.ok("mara", "commit", commits("api", "the lead's"))
	leads := f.git(f.wt("mara", "api"), "rev-parse", "HEAD")
	text = f.ok("tove", "switch", map[string]string{"branch": "wt/ledger/tove/y", "from": "lead/ledger/atlas/integration"})
	if !strings.Contains(text, "New in api (from lead/ledger/atlas/integration), web (from lead/ledger/atlas/integration).") || f.git(f.wt("tove", "api"), "rev-parse", "HEAD") != leads {
		t.Errorf("from a teammate's: %q", text)
	}
	text = f.ok("tove", "switch", map[string]string{"branch": "wt/ledger/tove/z", "from": leads})
	if !strings.Contains(text, "New in api (from "+leads+"), web (from trunk).") {
		t.Errorf("from a commit: %q", text)
	}
}

// Work a branch of the CEO's holds isn't new: no hand-off of it, wherever
// that branch came from.
func TestWorkTheCEOsBranchesHoldIsNotNew(t *testing.T) {
	f := newFixture(t)
	f.release()
	f.ok("tove", "switch", map[string]string{"branch": "wt/ledger/tove/x", "from": "release/1.0"})
	f.refused("tove", "handoff", map[string]string{"teammate": "mara", "branch": "wt/ledger/tove/x", "summary": "nothing"}, "nothing_new")
	// Its work, once in a branch the CEO made and named nowhere, isn't new either.
	writeFile(t, filepath.Join(f.wt("tove", "api"), "x.go"), "package x\n")
	f.ok("tove", "commit", commits("api", "x"))
	f.ok("tove", "handoff", map[string]string{"teammate": "mara", "branch": "wt/ledger/tove/x", "summary": "x"})
	f.git(filepath.Join(f.root, "ledger", "api"), "branch", "spike", "wt/ledger/tove/x")
	f.refused("tove", "handoff", map[string]string{"teammate": "mara", "branch": "wt/ledger/tove/x", "summary": "again"}, "nothing_new")
}

// A dismissed review's branch still checked out goes back to the default
// branch, as idle desks do, not to the branch it was for.
func TestDroppedBranchGoesToTheDefault(t *testing.T) {
	f := newFixture(t)
	f.release()
	api := filepath.Join(f.root, "ledger", "api")
	f.ok("mara", "switch", map[string]string{"branch": "lead/ledger/atlas/fix", "from": "release/1.0"})
	writeFile(t, filepath.Join(f.wt("mara", "api"), "fix.go"), "package fix\n")
	f.ok("mara", "commit", commits("api", "the fix"))
	f.ok("mara", "request_merge", map[string]string{"branch": "lead/ledger/atlas/fix", "title": "Invoices as CSV", "entry": "backlog/led-44.md", "into": "release/1.0"})
	if _, err := f.w.DismissReview(ctx, "rev-1", Dismiss{DropBranches: true, StopTeam: true}); err != nil {
		t.Fatal(err)
	}
	if f.git(f.wt("mara", "api"), "rev-parse", "HEAD") != f.git(api, "rev-parse", "main") {
		t.Error("the lead's worktree isn't at the default branch")
	}
}

// A refresh naming into takes every repo the review has open along, or is
// refused: a review never ends up split between branches unasked.
func TestIntoMovesTheWholeReview(t *testing.T) {
	f := newFixture(t)
	f.release()
	f.ok("mara", "switch", map[string]string{"branch": "lead/ledger/atlas/fix", "from": "release/1.0"})
	for _, r := range []string{"api", "web"} {
		writeFile(t, filepath.Join(f.wt("mara", r), "fix.go"), "package fix\n")
	}
	f.ok("mara", "commit", commits("api", "fix", "web", "fix"))
	fix := map[string]string{"branch": "lead/ledger/atlas/fix", "title": "Invoices as CSV", "entry": "backlog/led-44.md"}
	f.ok("mara", "request_merge", fix) // api into main, web into trunk
	// web's work reaches trunk outside Foreman: nothing new there any more,
	// but its merge request is still open.
	web := filepath.Join(f.root, "ledger", "web")
	f.git(web, "update-ref", "refs/heads/trunk", "lead/ledger/atlas/fix")
	fix["into"] = "release/1.0"
	if text := f.refused("mara", "request_merge", fix, "no_such_branch"); !strings.Contains(text, "in web (no release/1.0)") {
		t.Errorf("%q", text)
	}
	if f.mr("mr-1").Target != "main" || f.mr("mr-2").Target != "trunk" {
		t.Errorf("moved on a refusal: %s %s", f.mr("mr-1").Target, f.mr("mr-2").Target)
	}
}

// Moving a merge request to a branch that holds all of it already settles it
// there and then: the review never shows it open, and nothing waits on the CEO.
func TestIntoABranchThatHoldsItIsMerged(t *testing.T) {
	f := newFixture(t)
	f.release()
	f.ok("mara", "switch", map[string]string{"branch": "lead/ledger/atlas/fix", "from": "release/1.0"})
	writeFile(t, filepath.Join(f.wt("mara", "api"), "fix.go"), "package fix\n")
	f.ok("mara", "commit", commits("api", "fix"))
	fix := map[string]string{"branch": "lead/ledger/atlas/fix", "title": "Invoices as CSV", "entry": "backlog/led-44.md"}
	f.ok("mara", "request_merge", fix) // into main
	f.git(filepath.Join(f.root, "ledger", "api"), "update-ref", "refs/heads/release/1.0", "lead/ledger/atlas/fix")
	fix["into"] = "release/1.0"
	if text := f.ok("mara", "request_merge", fix); !strings.Contains(text, "api (mr-1) into release/1.0, which holds all of it already: counted as merged") || !strings.HasSuffix(text, ": nothing left for the CEO.") {
		t.Errorf("%q", text)
	}
	if mr := f.mr("mr-1"); mr.State != store.MergeMerged || mr.Target != "release/1.0" {
		t.Errorf("%s into %s", mr.State, mr.Target)
	}
	if open, _ := f.w.OpenRequests(ctx); len(open) != 0 {
		t.Errorf("still waits on the CEO: %+v", open)
	}
}

// Moving a review's repo to another branch outdates the comments on its
// removed lines (they were the old branch's), not those on the branch's.
func TestRetargetOutdatesRemovedLineComments(t *testing.T) {
	f := newFixture(t)
	f.release()
	f.integrated()
	f.ok("mara", "request_merge", integration)
	head := f.git(filepath.Join(f.root, "ledger", "api"), "rev-parse", "lead/ledger/atlas/integration")
	for _, side := range []string{"old", "new"} {
		if _, err := f.w.AddComment(ctx, "rev-1", store.ReviewComment{Repo: "api", Path: "LED-44.go", Side: side, Line: 1, Commit: head, Body: side}); err != nil {
			t.Fatal(err)
		}
	}
	outdated := func() []bool {
		var each []bool
		for _, c := range f.review("rev-1").Comments {
			each = append(each, c.Outdated)
		}
		return each
	}
	if got := outdated(); len(got) != 2 || got[0] || got[1] {
		t.Fatalf("before: %v", got)
	}
	if err := f.w.RetargetReview(ctx, "rev-1", "api", "release/1.0"); err != nil {
		t.Fatal(err)
	}
	if got := outdated(); !got[0] || got[1] {
		t.Errorf("after: %v, want the removed line's only", got)
	}
}

// A branch deleted outside Foreman while a merge request goes into it: its
// repo shows no diff and no head, so it can't merge until moved elsewhere.
func TestBranchGoneUnderAnOpenReview(t *testing.T) {
	f := newFixture(t)
	f.release()
	f.ok("mara", "switch", map[string]string{"branch": "lead/ledger/atlas/fix", "from": "release/1.0"})
	writeFile(t, filepath.Join(f.wt("mara", "api"), "fix.go"), "package fix\n")
	f.ok("mara", "commit", commits("api", "the fix"))
	f.ok("mara", "request_merge", map[string]string{"branch": "lead/ledger/atlas/fix", "title": "Invoices as CSV", "entry": "backlog/led-44.md", "into": "release/1.0"})
	f.git(filepath.Join(f.root, "ledger", "api"), "branch", "-D", "release/1.0")
	diffs, err := f.w.ReviewDiff(ctx, "rev-1", false)
	if err != nil || len(diffs) != 1 || !diffs[0].TargetGone || diffs[0].Head != "" || diffs[0].Diff != "" {
		t.Fatalf("%+v %v", diffs, err)
	}
	if err := f.w.RetargetReview(ctx, "rev-1", "api", "main"); err != nil {
		t.Fatal(err)
	}
	if diffs, _ := f.w.ReviewDiff(ctx, "rev-1", false); diffs[0].TargetGone || diffs[0].Head == "" {
		t.Errorf("moved to main: %+v", diffs[0])
	}
}
