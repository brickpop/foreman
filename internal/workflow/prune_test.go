package workflow

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/brickpop/foreman/internal/git"
	"github.com/brickpop/foreman/internal/store"
)

// branchAt makes branch in the api repo: on main, or one commit ahead of it
// (work no target holds).
func (f *fixture) branchAt(branch string, ahead bool) {
	f.t.Helper()
	api := filepath.Join(f.root, "ledger", "api")
	if !ahead {
		f.git(api, "branch", branch, "main")
		return
	}
	f.git(api, "checkout", "-q", "-b", branch, "main")
	writeFile(f.t, filepath.Join(api, strings.ReplaceAll(branch, "/", "-")), "work\n")
	f.git(api, "add", ".")
	f.git(api, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-qm", branch)
	f.git(api, "checkout", "-q", "main")
}

func (f *fixture) has(branch string) bool {
	return git.New().HasBranch(ctx, filepath.Join(f.root, "ledger", "api"), branch)
}

// A member's prune() deletes only its own merged branches: never its home
// branch, the one it is on, unmerged work it didn't name, or anyone else's.
func TestPruneMember(t *testing.T) {
	f := newFixture(t)
	f.branchAt("wt/ledger/tove/done", false)
	f.branchAt("wt/ledger/tove/wip", true)
	f.branchAt("wt/ledger/ivo/done", false)
	f.branchAt("lead/ledger/atlas/done", false)

	got := f.ok("tove", "prune", map[string]any{})
	want := "api: deleted wt/ledger/tove/done; kept wt/ledger/tove/wip (not merged: name it to delete it anyway)\n" +
		"web: nothing to delete\nForeman keeps a copy of each for 30 days."
	if got != want {
		t.Errorf("got  %q\nwant %q", got, want)
	}
	for b, kept := range map[string]bool{"wt/ledger/tove/done": false, "wt/ledger/tove/wip": true, "wt/ledger/tove/main": true,
		"wt/ledger/ivo/done": true, "lead/ledger/atlas/done": true} {
		if f.has(b) != kept {
			t.Errorf("%s: kept %v, want %v", b, f.has(b), kept)
		}
	}
	api := filepath.Join(f.root, "ledger", "api")
	if !strings.Contains(f.git(api, "for-each-ref", "refs/foreman/cleaned/"), "wt/ledger/tove/done") {
		t.Error("no copy of the deleted branch")
	}

	// Named, its own unmerged work goes.
	f.ok("tove", "prune", map[string]any{"branches": []string{"wt/ledger/tove/wip"}})
	if f.has("wt/ledger/tove/wip") {
		t.Error("the named branch stayed")
	}
	// Anyone else's, never; a home branch named stays.
	for _, b := range []string{"wt/ledger/ivo/done", "lead/ledger/atlas/done", "main", "wt/ledger/tove/"} {
		f.refused("tove", "prune", map[string]any{"branches": []string{b}}, "not_yours")
	}
	if got := f.ok("tove", "prune", map[string]any{"branches": []string{"wt/ledger/tove/main"}}); !strings.Contains(got, "wt/ledger/tove/main (a home branch stays)") {
		t.Errorf("home branch: %q", got)
	}
	if got := f.ok("tove", "prune", map[string]any{"branches": []string{"wt/ledger/tove/gone"}}); !strings.Contains(got, "wt/ledger/tove/gone: no such branch") {
		t.Errorf("a branch that isn't there: %q", got)
	}
}

// The lead's prune() also deletes its teammates' merged branches, but never
// what a teammate works on: the branch it has checked out (whatever it holds
// uncommitted), nor its unmerged work, even named.
func TestPruneLeadKeepsTeammatesWork(t *testing.T) {
	f := newFixture(t)
	f.branchAt("lead/ledger/atlas/done", false)
	f.branchAt("wt/ledger/ivo/done", false)
	f.branchAt("wt/ledger/ivo/parked", true)
	f.branchAt("wt/ledger/tove/led-1", false)
	f.ok("tove", "switch", map[string]any{"branch": "wt/ledger/tove/led-1"})
	writeFile(t, filepath.Join(f.wt("tove", "api"), "draft.txt"), "not committed\n")

	got := f.ok("mara", "prune", map[string]any{})
	for _, want := range []string{
		"deleted lead/ledger/atlas/done, wt/ledger/ivo/done",
		"wt/ledger/ivo/parked (not merged: only its owner deletes unmerged work)",
		"wt/ledger/tove/led-1 (checked out by tove)",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in\n%s", want, got)
		}
	}
	if strings.Contains(got, "/main") || strings.Contains(got, "integration") {
		t.Errorf("home branches listed: %s", got)
	}
	f.ok("mara", "prune", map[string]any{"branches": []string{"wt/ledger/ivo/parked", "wt/ledger/tove/led-1"}})
	for _, b := range []string{"wt/ledger/ivo/parked", "wt/ledger/tove/led-1", "wt/ledger/tove/main", "lead/ledger/atlas/integration"} {
		if !f.has(b) {
			t.Errorf("%s went", b)
		}
	}
	if f.git(f.wt("tove", "api"), "branch", "--show-current") != "wt/ledger/tove/led-1" {
		t.Error("tove was moved off its branch")
	}
	if data := f.read(filepath.Join(f.wt("tove", "api"), "draft.txt")); data != "not committed\n" {
		t.Errorf("tove's work: %q", data)
	}
}

// A branch under review stays, merged or not: its review reads it.
func TestPruneKeepsBranchesUnderReview(t *testing.T) {
	f := newFixture(t)
	f.branchAt("lead/ledger/atlas/x", false)
	if _, err := f.db.MergeRequests().Save(ctx, store.MergeRequest{ProjectID: "ledger", DeploymentID: "atlas@ledger",
		Repo: "api", Branch: "lead/ledger/atlas/x", Target: "main", State: store.MergeOpen}); err != nil {
		t.Fatal(err)
	}
	if got := f.ok("mara", "prune", map[string]any{"branches": []string{"lead/ledger/atlas/x"}}); !strings.Contains(got, "lead/ledger/atlas/x (its review is open)") {
		t.Errorf("got %q", got)
	}
	if !f.has("lead/ledger/atlas/x") {
		t.Error("the branch under review went")
	}
}

// ahead makes branch in the api repo one commit ahead of from.
func (f *fixture) ahead(branch, from string) {
	f.t.Helper()
	api := filepath.Join(f.root, "ledger", "api")
	f.git(api, "checkout", "-q", "-b", branch, from)
	writeFile(f.t, filepath.Join(api, strings.ReplaceAll(branch, "/", "-")), "work\n")
	f.git(api, "add", ".")
	f.git(api, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-qm", branch)
	f.git(api, "checkout", "-q", "main")
}

// A branch whose commits are all in another goes without being named: its
// owner's own, or for the lead a teammate's held by that same teammate's. Of
// two at one commit, one stays; a teammate's held only by someone else's
// stays too.
func TestPruneSuperseded(t *testing.T) {
	f := newFixture(t)
	f.ahead("wt/ledger/tove/draft", "main")
	f.ahead("wt/ledger/tove/final", "wt/ledger/tove/draft")
	f.ahead("wt/ledger/ivo/notes", "main")
	f.ahead("wt/ledger/tove/more", "wt/ledger/ivo/notes") // holds ivo's notes
	f.git(filepath.Join(f.root, "ledger", "api"), "branch", "wt/ledger/tove/twin", "wt/ledger/tove/final")

	got := f.ok("mara", "prune", map[string]any{})
	for _, want := range []string{
		"wt/ledger/ivo/notes (not merged: only its owner deletes unmerged work)",
		"wt/ledger/tove/draft",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in\n%s", want, got)
		}
	}
	// tove's own: draft (in final) goes; final and its twin hold each other, so one stays.
	if f.has("wt/ledger/tove/draft") || !f.has("wt/ledger/ivo/notes") || !f.has("wt/ledger/tove/more") {
		t.Errorf("after the lead's prune:\n%s", got)
	}
	if f.has("wt/ledger/tove/final") == f.has("wt/ledger/tove/twin") {
		t.Errorf("final %v, twin %v: exactly one of two at one commit stays", f.has("wt/ledger/tove/final"), f.has("wt/ledger/tove/twin"))
	}
	// ivo itself may: its notes are in tove's more.
	f.ok("ivo", "prune", map[string]any{})
	if f.has("wt/ledger/ivo/notes") {
		t.Error("ivo's superseded notes stayed")
	}
}

// Work upstream has is merged, even if the CEO hasn't pulled it; a branch's
// own copy upstream is no merge.
func TestPruneUpstream(t *testing.T) {
	f := newFixture(t)
	api := filepath.Join(f.root, "ledger", "api")
	f.ahead("wt/ledger/tove/upstreamed", "main")
	f.ahead("wt/ledger/tove/pushed", "main")
	f.git(api, "update-ref", "refs/remotes/origin/development", "wt/ledger/tove/upstreamed")
	f.git(api, "update-ref", "refs/remotes/origin/wt/ledger/tove/pushed", "wt/ledger/tove/pushed")
	got := f.ok("tove", "prune", map[string]any{})
	if f.has("wt/ledger/tove/upstreamed") || !f.has("wt/ledger/tove/pushed") {
		t.Errorf("got\n%s", got)
	}
}

// The CEO's branch cleanup lists the project's agents' branches and legacy
// ones, whatever their state, and the CEO's merged or superseded ones (not the
// default, not unmerged ones); it deletes the pick, checking each again.
func TestCleanupBranchesAndDelete(t *testing.T) {
	f := newFixture(t)
	api := filepath.Join(f.root, "ledger", "api")
	f.ahead("wt/ledger/tove/old", "main")
	f.ahead("wt/ledger/tove/older", "main")
	f.git(api, "branch", "wt/tove/legacy", "main")
	f.git(api, "branch", "wt/suite/tove/x", "main") // suite is another project of the home
	f.git(api, "branch", "feature/done", "main")    // the CEO's, merged
	f.ahead("feature/wip", "main")                  // the CEO's, unmerged: not offered
	f.ahead("feature/base", "main")                 // the CEO's, held by an agent's
	f.ahead("wt/ledger/tove/on-base", "feature/base")
	f.git(api, "branch", "release/2", "main") // the CEO's, merged, but an open review's target
	f.ahead("wt/ledger/tove/older-final", "wt/ledger/tove/older")
	if _, err := f.db.MergeRequests().Save(ctx, store.MergeRequest{ProjectID: "ledger", DeploymentID: "atlas@ledger",
		Repo: "api", Branch: "lead/ledger/atlas/integration", Target: "release/2", State: store.MergeOpen}); err != nil {
		t.Fatal(err)
	}

	list, err := f.w.CleanupBranches(ctx, "ledger", "api")
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, b := range list {
		line := b.Branch + " " + b.State
		if !b.Agents {
			line += " yours"
		}
		if b.Legacy {
			line += " legacy"
		}
		if len(b.SupersededBy) > 0 {
			line += " by " + strings.Join(b.SupersededBy, ",")
		}
		if b.Stays != "" {
			line += " (" + b.Stays + ")"
		}
		got = append(got, line)
	}
	want := []string{
		"feature/base superseded yours by wt/ledger/tove/on-base",
		"feature/done merged yours",
		"lead/ledger/atlas/integration merged (checked out by mara)",
		"release/2 merged yours (an open review merges into it)",
		"wt/ledger/tove/main merged (checked out by tove)",
		"wt/ledger/tove/old unmerged",
		"wt/ledger/tove/older superseded by wt/ledger/tove/older-final",
		"wt/ledger/tove/older-final unmerged",
		"wt/ledger/tove/on-base unmerged",
		"wt/tove/legacy merged legacy",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got  %q\nwant %q", got, want)
	}

	done, err := f.w.DeleteBranches(ctx, "ledger", "api", []string{"wt/ledger/tove/old", "wt/tove/legacy", "wt/ledger/tove/main",
		"wt/suite/tove/x", "feature/wip", "main", "release/2", "feature/done", "wt/ledger/tove/none", "wt/tove/legacy", "-x"})
	if err != nil {
		t.Fatal(err)
	}
	line := func(list []Cleaned) (out []string) {
		for _, c := range list {
			out = append(out, strings.TrimSpace(c.Branch+" "+c.Why))
		}
		return out
	}
	if want := []string{"feature/done", "wt/ledger/tove/old", "wt/tove/legacy"}; !reflect.DeepEqual(line(done.Removed), want) {
		t.Errorf("removed %q, want %q", line(done.Removed), want)
	}
	if want := []string{
		"-x no such branch",
		"feature/wip not merged",
		"main the default branch",
		"release/2 an open review merges into it",
		"wt/ledger/tove/main checked out by tove",
		"wt/ledger/tove/none no such branch",
		"wt/suite/tove/x another project's",
	}; !reflect.DeepEqual(line(done.Kept), want) {
		t.Errorf("kept %q\nwant %q", line(done.Kept), want)
	}
	// The CEO's feature/base was superseded only by tove's on-base: once that is gone, it is unmerged and stays.
	if done, _ := f.w.DeleteBranches(ctx, "ledger", "api", []string{"wt/ledger/tove/on-base"}); len(done.Removed) != 1 {
		t.Errorf("on-base: %+v", done)
	}
	if done, _ := f.w.DeleteBranches(ctx, "ledger", "api", []string{"feature/base"}); !reflect.DeepEqual(line(done.Kept), []string{"feature/base not merged"}) {
		t.Errorf("feature/base: %q", line(done.Kept))
	}
	if _, err := f.w.CleanupBranches(ctx, "ledger", "nope"); !errors.Is(err, ErrNoRepo) {
		t.Errorf("no such repo: %v", err)
	}
}

// Long-lived names are never offered, merged or not.
func TestCleanupNeverOffersLongLivedBranches(t *testing.T) {
	f := newFixture(t)
	api := filepath.Join(f.root, "ledger", "api")
	for _, b := range []string{"master", "develop", "staging", "feature/done"} {
		f.git(api, "branch", b, "main")
	}
	list, err := f.w.CleanupBranches(ctx, "ledger", "api")
	if err != nil {
		t.Fatal(err)
	}
	for _, b := range list {
		if b.Branch == "master" || b.Branch == "develop" || b.Branch == "staging" {
			t.Errorf("%s offered", b.Branch)
		}
	}
	done, _ := f.w.DeleteBranches(ctx, "ledger", "api", []string{"develop", "feature/done"})
	if len(done.Removed) != 1 || len(done.Kept) != 1 || done.Kept[0].Why != "a long-lived branch" || !f.has("develop") {
		t.Errorf("%+v", done)
	}
}

// A branch checked out in a stray worktree (no project's agent's, not the
// CEO's checkout: an old workspace's, or a folder since deleted) is listed with
// it; picked, the worktree goes first, unless it holds work not committed. An
// agent's prune() never touches one.
func TestCleanupStrayWorktrees(t *testing.T) {
	f := newFixture(t)
	api := filepath.Join(f.root, "ledger", "api")
	old := t.TempDir()
	stray, dirty, gone := filepath.Join(old, "agents", "leady", "api"), filepath.Join(old, "agents", "ivo", "api"), filepath.Join(old, "agents", "tove", "x")
	f.git(api, "worktree", "add", "-q", "-b", "wt/leady/doc", stray, "main")
	f.git(api, "worktree", "add", "-q", "-b", "wt/ledger/ivo/draft", dirty, "main")
	f.git(api, "worktree", "add", "-q", "-b", "wt/ledger/tove/gone", gone, "main")
	writeFile(t, filepath.Join(dirty, "notes.md"), "not committed\n")
	if err := os.RemoveAll(gone); err != nil {
		t.Fatal(err)
	}

	list, err := f.w.CleanupBranches(ctx, "ledger", "api")
	if err != nil {
		t.Fatal(err)
	}
	strays := map[string]string{}
	for _, b := range list {
		if b.Stray != "" {
			strays[b.Branch] = b.Stays
		}
		if b.Branch == "wt/ledger/tove/main" && b.Stray != "" {
			t.Error("an agent's own worktree counted as stray")
		}
	}
	if len(strays) != 3 || !strings.HasPrefix(strays["wt/leady/doc"], "checked out in ") {
		t.Errorf("strays %q", strays)
	}

	// The agents leave them alone.
	if got := f.ok("tove", "prune", map[string]any{"branches": []string{"wt/ledger/tove/gone"}}); !strings.Contains(got, "wt/ledger/tove/gone (checked out in ") {
		t.Errorf("tove's prune: %q", got)
	}

	done, err := f.w.DeleteBranches(ctx, "ledger", "api", []string{"wt/leady/doc", "wt/ledger/ivo/draft", "wt/ledger/tove/gone"})
	if err != nil {
		t.Fatal(err)
	}
	var removed, kept []string
	for _, c := range done.Removed {
		removed = append(removed, c.What+" "+c.Branch)
	}
	for _, c := range done.Kept {
		kept = append(kept, c.Branch+" "+c.Why)
	}
	if want := []string{"worktree wt/leady/doc", "branch wt/leady/doc", "worktree wt/ledger/tove/gone", "branch wt/ledger/tove/gone"}; !reflect.DeepEqual(removed, want) {
		t.Errorf("removed %q\nwant    %q", removed, want)
	}
	if len(kept) != 1 || !strings.HasPrefix(kept[0], "wt/ledger/ivo/draft its worktree holds changes not committed") {
		t.Errorf("kept %q", kept)
	}
	if _, err := os.Stat(stray); !os.IsNotExist(err) {
		t.Errorf("the stray worktree stayed: %v", err)
	}
	if f.read(filepath.Join(dirty, "notes.md")) != "not committed\n" || !f.has("wt/ledger/ivo/draft") {
		t.Error("the dirty worktree's work went")
	}
	if strings.Contains(f.git(api, "worktree", "list"), old+"/agents/tove") {
		t.Error("git still lists the worktree whose folder was gone")
	}
}
