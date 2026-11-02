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

// Merge targets (wiki/product/features/pipeline-and-merges.md#merge-targets).

// release makes release/1.0 in api at its first commit, then moves api's main
// on (NEWS), so work for the release and work for main differ.
func (f *fixture) release() {
	f.t.Helper()
	f.ok("mara", "create_target", map[string]string{"repo": "api", "name": "release/1.0", "from": "main"})
	api := filepath.Join(f.root, "ledger", "api")
	writeFile(f.t, filepath.Join(api, "NEWS"), "v2\n")
	if err := git.New().Commit(ctx, api, "news", "NEWS"); err != nil {
		f.t.Fatal(err)
	}
}

func TestCreateTarget(t *testing.T) {
	f := newFixture(t)
	api := filepath.Join(f.root, "ledger", "api")
	first := f.git(api, "rev-parse", "main")
	text := f.ok("mara", "create_target", map[string]string{"repo": "api", "name": "release/1.0", "from": first[:7]})
	if !strings.HasPrefix(text, "release/1.0 is a target of api now, at "+first+".") {
		t.Errorf("%q", text)
	}
	if at := f.git(api, "rev-parse", "release/1.0"); at != first {
		t.Errorf("at %s", at)
	}
	if got := f.targets.made["api"]; len(got) != 1 || got[0].MadeBy != "mara@ledger" {
		t.Errorf("recorded %+v", got)
	}
	target := func(repo, name, from string) map[string]string {
		return map[string]string{"repo": repo, "name": name, "from": from}
	}
	f.refused("tove", "create_target", target("api", "release/2.0", "main"), "not_lead")
	f.refused("mara", "create_target", target("api", "x..y", "main"), "bad_branch")
	f.refused("mara", "create_target", target("api", "release/2.0", ""), "no_from")
	f.refused("mara", "create_target", target("docs", "release/2.0", "main"), "no_repo")
	f.refused("mara", "create_target", target("api", "taken", "main"), "target_exists")
	f.refused("mara", "create_target", target("api", "release/2.0", "nope"), "bad_target")
	// Kept as an action, refusals too.
	acts, err := f.db.Actions().List(ctx, "atlas@ledger")
	made, refused := 0, 0
	for _, a := range acts {
		switch {
		case a.Tool != "create_target":
		case a.Error == "" && a.Branch == "release/1.0":
			made++
		case a.Error != "":
			refused++
		}
	}
	if err != nil || made != 1 || refused != 6 {
		t.Errorf("actions: %d made, %d refused (%v)", made, refused, err)
	}
}

// Work for a target starts from it and lands on it: the review is reckoned
// against it, the merge goes there, idle desks stay on the default, and the
// notice that it moved goes only to merge requests into it.
func TestWorkLandsOnItsTarget(t *testing.T) {
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
	f.ok("mara", "request_merge", map[string]string{"branch": "lead/ledger/atlas/other", "entry": "backlog/led-44.md"})

	fix := map[string]string{"branch": "lead/ledger/atlas/fix", "entry": "backlog/led-44.md", "into": "release/1.0"}
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
		t.Error("an idle desk followed a target other than the default")
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

// A review spans every repo with new commits: into must be a target of each,
// or nothing is made. A refresh keeps the target it goes to; a new into
// changes it.
func TestIntoMustBeATargetOfEveryRepoWithNewCommits(t *testing.T) {
	f := newFixture(t)
	f.release()
	f.ok("mara", "switch", map[string]string{"branch": "lead/ledger/atlas/fix", "from": "release/1.0"})
	for _, r := range []string{"api", "web"} {
		writeFile(t, filepath.Join(f.wt("mara", r), "fix.go"), "package fix\n")
	}
	f.ok("mara", "commit", commits("api", "fix", "web", "fix"))
	fix := map[string]string{"branch": "lead/ledger/atlas/fix", "entry": "backlog/led-44.md", "into": "release/1.0"}
	if text := f.refused("mara", "request_merge", fix, "no_such_target"); !strings.Contains(text, "in web, which has no target release/1.0") {
		t.Errorf("%q", text)
	}
	if all, _ := f.db.MergeRequests().ForProject(ctx, "ledger"); len(all) != 0 {
		t.Errorf("made %+v", all)
	}
	if list, _ := f.db.Reviews().List(ctx); len(list) != 0 {
		t.Errorf("a review was made: %+v", list)
	}
	fix["into"] = "nope"
	f.refused("mara", "request_merge", fix, "no_such_target")

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
	f.refused("mara", "request_merge", fix, "no_such_target") // web has no main
}

// The CEO changes where one repo of a review merges: to another of its
// targets only; the diff follows, a conflict found before is forgotten, and
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
		"not a target":         {"api", "wt/ledger/tove/main", ErrInvalid},
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
	// A target whose branch went outside Foreman.
	f.git(filepath.Join(f.root, "ledger", "api"), "branch", "-D", "release/1.0")
	if err := f.w.RetargetReview(ctx, "rev-1", "api", "main"); err != nil {
		t.Errorf("back to main: %v", err)
	}
	if err := f.w.RetargetReview(ctx, "rev-1", "api", "release/1.0"); !errors.Is(err, ErrInvalid) {
		t.Errorf("to a branch that's gone: %v", err)
	}
}

// switch from: a target of at least one repo, used only for a new branch.
func TestSwitchFromATarget(t *testing.T) {
	f := newFixture(t)
	f.release()
	f.refused("tove", "switch", map[string]string{"branch": "wt/ledger/tove/x", "from": "nope"}, "no_such_target")
	f.refused("tove", "switch", map[string]string{"branch": "wt/ledger/tove/x", "from": "wt/ledger/tove/main"}, "no_such_target")
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
}

// Work a target holds isn't new: no hand-off of it, and a resumed agent
// isn't told about it.
func TestWorkInAnyTargetIsNotNew(t *testing.T) {
	f := newFixture(t)
	f.release()
	f.ok("tove", "switch", map[string]string{"branch": "wt/ledger/tove/x", "from": "release/1.0"})
	f.refused("tove", "handoff", map[string]string{"teammate": "mara", "branch": "wt/ledger/tove/x", "summary": "nothing"}, "nothing_new")
}

// A dismissed review's branch still checked out goes back to the default
// branch, as idle desks do, not to the target it was for.
func TestDroppedBranchGoesToTheDefault(t *testing.T) {
	f := newFixture(t)
	f.release()
	api := filepath.Join(f.root, "ledger", "api")
	f.ok("mara", "switch", map[string]string{"branch": "lead/ledger/atlas/fix", "from": "release/1.0"})
	writeFile(t, filepath.Join(f.wt("mara", "api"), "fix.go"), "package fix\n")
	f.ok("mara", "commit", commits("api", "the fix"))
	f.ok("mara", "request_merge", map[string]string{"branch": "lead/ledger/atlas/fix", "entry": "backlog/led-44.md", "into": "release/1.0"})
	if _, err := f.w.DismissReview(ctx, "rev-1", Dismiss{DropBranches: true, StopTeam: true}); err != nil {
		t.Fatal(err)
	}
	if f.git(f.wt("mara", "api"), "rev-parse", "HEAD") != f.git(api, "rev-parse", "main") {
		t.Error("the lead's worktree isn't at the default branch")
	}
}

// targets() lists each repo's targets, the default first, with who made the
// others and at which commit; for members too.
func TestTargetsTool(t *testing.T) {
	f := newFixture(t)
	f.release()
	at := f.git(filepath.Join(f.root, "ledger", "api"), "rev-parse", "release/1.0")
	text := f.ok("tove", "targets", map[string]string{})
	for _, want := range []string{"api: main (default), release/1.0 (made by mara at " + at[:8] + ")", "web: trunk (default)", "switch(<topic>, <target>)"} {
		if !strings.Contains(text, want) {
			t.Errorf("no %q in %q", want, text)
		}
	}
}

// A refresh naming into takes every repo the review has open along, or is
// refused: a review never ends up split between targets unasked.
func TestIntoMovesTheWholeReview(t *testing.T) {
	f := newFixture(t)
	f.release()
	f.ok("mara", "switch", map[string]string{"branch": "lead/ledger/atlas/fix", "from": "release/1.0"})
	for _, r := range []string{"api", "web"} {
		writeFile(t, filepath.Join(f.wt("mara", r), "fix.go"), "package fix\n")
	}
	f.ok("mara", "commit", commits("api", "fix", "web", "fix"))
	fix := map[string]string{"branch": "lead/ledger/atlas/fix", "entry": "backlog/led-44.md"}
	f.ok("mara", "request_merge", fix) // api into main, web into trunk
	// web's work reaches trunk outside Foreman: nothing new there any more,
	// but its merge request is still open.
	web := filepath.Join(f.root, "ledger", "web")
	f.git(web, "update-ref", "refs/heads/trunk", "lead/ledger/atlas/fix")
	fix["into"] = "release/1.0"
	if text := f.refused("mara", "request_merge", fix, "no_such_target"); !strings.Contains(text, "in web, which has no target release/1.0") {
		t.Errorf("%q", text)
	}
	if f.mr("mr-1").Target != "main" || f.mr("mr-2").Target != "trunk" {
		t.Errorf("moved on a refusal: %s %s", f.mr("mr-1").Target, f.mr("mr-2").Target)
	}
}

// Moving a merge request to a target that holds all of it already settles it
// there and then: the review never shows it open, and nothing waits on the CEO.
func TestIntoATargetThatHoldsItIsMerged(t *testing.T) {
	f := newFixture(t)
	f.release()
	f.ok("mara", "switch", map[string]string{"branch": "lead/ledger/atlas/fix", "from": "release/1.0"})
	writeFile(t, filepath.Join(f.wt("mara", "api"), "fix.go"), "package fix\n")
	f.ok("mara", "commit", commits("api", "fix"))
	fix := map[string]string{"branch": "lead/ledger/atlas/fix", "entry": "backlog/led-44.md"}
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

// Moving a review's repo to another target outdates the comments on its
// removed lines (they were the old target's), not those on the branch's.
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

// A target deleted outside Foreman while a merge request goes into it: its
// repo shows no diff and no head, so it can't merge until moved elsewhere.
func TestTargetGoneUnderAnOpenReview(t *testing.T) {
	f := newFixture(t)
	f.release()
	f.ok("mara", "switch", map[string]string{"branch": "lead/ledger/atlas/fix", "from": "release/1.0"})
	writeFile(t, filepath.Join(f.wt("mara", "api"), "fix.go"), "package fix\n")
	f.ok("mara", "commit", commits("api", "the fix"))
	f.ok("mara", "request_merge", map[string]string{"branch": "lead/ledger/atlas/fix", "entry": "backlog/led-44.md", "into": "release/1.0"})
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
