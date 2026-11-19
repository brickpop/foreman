package workflow

import (
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

// A member's prune() deletes only its own merged branches: never the one it is on, unmerged work
// it didn't name, or anyone else's.
func TestPruneMember(t *testing.T) {
	f := newFixture(t)
	f.branchAt("wt/ledger/tove/done", false)
	f.branchAt("wt/ledger/tove/wip", true)
	f.branchAt("wt/ledger/ivo/done", false)
	f.branchAt("lead/ledger/atlas/done", false)

	got := f.ok("tove", "prune", map[string]any{})
	want := "api: deleted wt/ledger/tove/done; kept wt/ledger/tove/main (checked out by tove), wt/ledger/tove/wip (not merged: name it to delete it anyway)\n" +
		"web: kept wt/ledger/tove/main (checked out by tove)\nForeman keeps a copy of each for 30 days."
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
	// Anyone else's, never; the one it is on, named, stays.
	for _, b := range []string{"wt/ledger/ivo/done", "lead/ledger/atlas/done", "main", "wt/ledger/tove/"} {
		f.refused("tove", "prune", map[string]any{"branches": []string{b}}, "not_yours")
	}
	if got := f.ok("tove", "prune", map[string]any{"branches": []string{"wt/ledger/tove/main"}}); !strings.Contains(got, "wt/ledger/tove/main (checked out by tove)") || !f.has("wt/ledger/tove/main") {
		t.Errorf("the branch it is on: %q", got)
	}
	if got := f.ok("tove", "prune", map[string]any{"branches": []string{"wt/ledger/tove/gone"}}); !strings.Contains(got, "wt/ledger/tove/gone: no such branch") {
		t.Errorf("a branch that isn't there: %q", got)
	}
}

// The lead's prune() also deletes its teammates' merged branches, but never
// what a teammate works on: the branch it has checked out (whatever it holds
// uncommitted), nor its unmerged work, even named. A home branch from before
// idle worktrees went detached is an ordinary branch: merged and not checked
// out, it goes.
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
		"deleted lead/ledger/atlas/done, wt/ledger/ivo/done, wt/ledger/tove/main",
		"lead/ledger/atlas/integration (checked out by mara)",
		"wt/ledger/ivo/parked (not merged: only its owner deletes unmerged work)",
		"wt/ledger/tove/led-1 (checked out by tove)",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in\n%s", want, got)
		}
	}
	f.ok("mara", "prune", map[string]any{"branches": []string{"wt/ledger/ivo/parked", "wt/ledger/tove/led-1"}})
	for _, b := range []string{"wt/ledger/ivo/parked", "wt/ledger/tove/led-1", "lead/ledger/atlas/integration"} {
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

	list, err := f.cleanupIn("api")
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
		if b.Free != "" {
			line += " free " + b.Free
		}
		if b.Stays != "" {
			line += " (" + b.Stays + ")"
		}
		got = append(got, line)
	}
	want := []string{
		"feature/base superseded yours by wt/ledger/tove/on-base",
		"feature/done merged yours",
		"lead/ledger/atlas/integration merged (checked out by mara, a review keeps it in api)",
		"release/2 merged yours (an open review merges into it)",
		"wt/ledger/tove/main merged free tove",
		"wt/ledger/tove/old unmerged",
		"wt/ledger/tove/older superseded by wt/ledger/tove/older-final",
		"wt/ledger/tove/older-final unmerged",
		"wt/ledger/tove/on-base unmerged",
		"wt/tove/legacy merged legacy",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got  %q\nwant %q", got, want)
	}

	done, err := f.deleteIn("api", []string{"wt/ledger/tove/old", "wt/tove/legacy", "wt/ledger/tove/main",
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
		// tove, not at work, has it checked out in web too: picked in api only, it stays in both.
		"wt/ledger/tove/main pick it in every repo tove has it: api, web",
		"wt/ledger/tove/main pick it in every repo tove has it: api, web",
		"-x no such branch",
		"feature/wip not merged",
		"main the default branch",
		"release/2 an open review merges into it",
		"wt/ledger/tove/none no such branch",
		"wt/suite/tove/x another project's",
	}; !reflect.DeepEqual(line(done.Kept), want) {
		t.Errorf("kept %q\nwant %q", line(done.Kept), want)
	}
	// The CEO's feature/base was superseded only by tove's on-base: once that is gone, it is unmerged and stays.
	if done, _ := f.deleteIn("api", []string{"wt/ledger/tove/on-base"}); len(done.Removed) != 1 {
		t.Errorf("on-base: %+v", done)
	}
	if done, _ := f.deleteIn("api", []string{"feature/base"}); !reflect.DeepEqual(line(done.Kept), []string{"feature/base not merged"}) {
		t.Errorf("feature/base: %q", line(done.Kept))
	}
}

// cleanupIn is what the branch cleanup offers in one of ledger's repos.
func (f *fixture) cleanupIn(repo string) ([]CleanupBranch, error) {
	all, err := f.w.CleanupBranches(ctx, "ledger")
	var list []CleanupBranch
	for _, b := range all {
		if b.Repo == repo {
			list = append(list, b)
		}
	}
	return list, err
}

// deleteIn deletes branches of one of ledger's repos through the branch cleanup.
func (f *fixture) deleteIn(repo string, branches []string) (Cleanup, error) {
	var picks []BranchPick
	for _, b := range branches {
		picks = append(picks, BranchPick{Repo: repo, Branch: b})
	}
	return f.w.DeleteBranches(ctx, "ledger", picks)
}

// The branch cleanup is the project's: every code repo at once, each branch
// with its repo, in the project's order; a pick goes in its own repo only,
// and one in a repo the project hasn't is kept, saying so.
func TestCleanupBranchesAcrossRepos(t *testing.T) {
	f := newFixture(t)
	f.git(filepath.Join(f.root, "ledger", "api"), "branch", "feature/done", "main")
	f.git(filepath.Join(f.root, "ledger", "web"), "branch", "feature/done", "trunk")
	f.git(filepath.Join(f.root, "ledger", "web"), "branch", "feature/web-only", "trunk")
	list, err := f.w.CleanupBranches(ctx, "ledger")
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, b := range list {
		if strings.HasPrefix(b.Branch, "feature/") {
			got = append(got, b.Repo+" "+b.Branch)
		}
	}
	if want := []string{"api feature/done", "web feature/done", "web feature/web-only"}; !reflect.DeepEqual(got, want) {
		t.Errorf("listed %q, want %q", got, want)
	}
	done, err := f.w.DeleteBranches(ctx, "ledger", []BranchPick{{"web", "feature/done"}, {"web", "feature/web-only"}, {"nope", "feature/done"}})
	if err != nil {
		t.Fatal(err)
	}
	var removed, kept []string
	for _, c := range done.Removed {
		removed = append(removed, c.Repo+" "+c.Branch)
	}
	for _, c := range done.Kept {
		kept = append(kept, c.Repo+" "+c.Branch+" "+c.Why)
	}
	if want := []string{"web feature/done", "web feature/web-only"}; !reflect.DeepEqual(removed, want) {
		t.Errorf("removed %q", removed)
	}
	if want := []string{"nope feature/done no such repo"}; !reflect.DeepEqual(kept, want) {
		t.Errorf("kept %q", kept)
	}
	if !f.has("feature/done") {
		t.Error("api's feature/done went with web's")
	}
	if _, err := f.w.CleanupBranches(ctx, "nope"); err == nil {
		t.Error("no such project: no error")
	}
}

// Long-lived names are never offered, merged or not.
func TestCleanupNeverOffersLongLivedBranches(t *testing.T) {
	f := newFixture(t)
	api := filepath.Join(f.root, "ledger", "api")
	for _, b := range []string{"master", "develop", "staging", "feature/done"} {
		f.git(api, "branch", b, "main")
	}
	list, err := f.cleanupIn("api")
	if err != nil {
		t.Fatal(err)
	}
	for _, b := range list {
		if b.Branch == "master" || b.Branch == "develop" || b.Branch == "staging" {
			t.Errorf("%s offered", b.Branch)
		}
	}
	done, _ := f.deleteIn("api", []string{"develop", "feature/done"})
	if len(done.Removed) != 1 || len(done.Kept) != 1 || done.Kept[0].Why != "a long-lived branch" || !f.has("develop") {
		t.Errorf("%+v", done)
	}
}

// A branch checked out in a stray worktree (no project's agent's, not the
// CEO's checkout: one moved by hand, or a folder since deleted) is listed with
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

	list, err := f.cleanupIn("api")
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

	done, err := f.deleteIn("api", []string{"wt/leady/doc", "wt/ledger/ivo/draft", "wt/ledger/tove/gone"})
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

// A squash merge leaves the lead's integration commits out of main and moves
// the lead branch on: a teammate's branch it carried is then held only by the
// merged review's kept head (refs/foreman/decided/<mr>). That counts as
// merged, for any project with the repo, while every line the branch changed
// is still there; a rejected review's kept head never does, nor one where the
// lead dropped some of the branch (resolving a conflict, say).
func TestPruneMergedReview(t *testing.T) {
	f := newFixture(t)
	api := filepath.Join(f.root, "ledger", "api")
	// review carries branch in a lead branch with a commit of the lead's on
	// top, decides it as state, keeps its head and deletes the lead branch
	// (as a squash moves it on). drop: the lead's commit removes the
	// branch's file.
	review := func(branch, project, repo, state string, drop bool) {
		lead := "lead/ledger/atlas/carried-" + filepath.Base(branch)
		f.ahead(lead, branch)
		if drop {
			f.git(api, "checkout", "-q", lead)
			f.git(api, "rm", "-q", strings.ReplaceAll(branch, "/", "-"))
			f.git(api, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-qm", "dropped")
			f.git(api, "checkout", "-q", "main")
		}
		mr, err := f.db.MergeRequests().Save(ctx, store.MergeRequest{ProjectID: project, DeploymentID: "atlas@ledger",
			Repo: repo, Branch: lead, Target: "main", State: state})
		if err != nil {
			t.Fatal(err)
		}
		if err := git.New().Keep(ctx, api, "refs/foreman/decided/"+mr.ID, f.git(api, "rev-parse", lead)); err != nil {
			t.Fatal(err)
		}
		f.git(api, "branch", "-D", lead)
	}
	f.ahead("wt/ledger/tove/squashed", "main")
	review("wt/ledger/tove/squashed", "ledger", "api", store.MergeMerged, false)
	f.ahead("wt/ledger/tove/rejected", "main")
	review("wt/ledger/tove/rejected", "ledger", "api", store.MergeRejected, false)
	f.ahead("wt/ledger/tove/elsewhere", "main")
	review("wt/ledger/tove/elsewhere", "suite", "core", store.MergeMerged, false)
	f.ahead("wt/ledger/tove/dropped", "main")
	review("wt/ledger/tove/dropped", "ledger", "api", store.MergeMerged, true)

	list, err := f.cleanupIn("api")
	if err != nil {
		t.Fatal(err)
	}
	states := map[string]string{}
	for _, b := range list {
		states[b.Branch] = b.State
	}
	want := map[string]string{"wt/ledger/tove/squashed": Merged, "wt/ledger/tove/elsewhere": Merged,
		"wt/ledger/tove/rejected": Unmerged, "wt/ledger/tove/dropped": Unmerged}
	for b, s := range want {
		if states[b] != s {
			t.Errorf("%s: %q, want %q", b, states[b], s)
		}
	}
	f.ok("mara", "prune", map[string]any{})
	for b, s := range want {
		if f.has(b) != (s == Unmerged) {
			t.Errorf("after the lead's prune, %s: kept %v", b, f.has(b))
		}
	}
}

// A repo in two projects: work that landed in either's default, or in a
// target of either's merge requests, is merged for both.
func TestMergedInAnotherProject(t *testing.T) {
	f := newFixture(t)
	api := filepath.Join(f.root, "ledger", "api")
	f.ahead("wt/ledger/tove/picked", "main")
	// Squashed into suite's default (stable): its commit isn't there, its change is.
	f.git(api, "checkout", "-q", "-b", "stable", "main")
	f.git(api, "-c", "user.name=t", "-c", "user.email=t@t", "cherry-pick", "wt/ledger/tove/picked")
	f.git(api, "checkout", "-q", "main")
	f.ahead("wt/ledger/tove/unpicked", "main")

	list, err := f.cleanupIn("api")
	if err != nil {
		t.Fatal(err)
	}
	states := map[string]string{}
	for _, b := range list {
		states[b.Branch] = b.State
	}
	if states["wt/ledger/tove/picked"] != Merged || states["wt/ledger/tove/unpicked"] != Unmerged {
		t.Errorf("states: %v", states)
	}
	if _, ok := states["stable"]; ok {
		t.Error("suite's default was offered")
	}
}

// toveMain is tove's home branch in the fixture, checked out in its
// worktrees of api and web, merged in both.
const toveMain = "wt/ledger/tove/main"

// rowsOf are the cleanup's rows of branch, in the project's order, read as
// "<repo> free <agent>" or "<repo> (<stays>)".
func (f *fixture) rowsOf(branch string) []string {
	f.t.Helper()
	list, err := f.w.CleanupBranches(ctx, "ledger")
	if err != nil {
		f.t.Fatal(err)
	}
	var out []string
	for _, b := range list {
		switch {
		case b.Branch != branch:
		case b.Free != "" && b.Stays == "":
			out = append(out, b.Repo+" free "+b.Free)
		default:
			out = append(out, b.Repo+" ("+b.Stays+")")
		}
	}
	return out
}

// mergeUnderWay leaves tove's worktree of repo in a merge not committed yet,
// its branch where it was.
func (f *fixture) mergeUnderWay(repo string) {
	f.t.Helper()
	dir := filepath.Join(f.root, "ledger", repo)
	f.git(dir, "branch", "side", mainOf(repo))
	side := filepath.Join(f.t.TempDir(), "side")
	f.git(dir, "worktree", "add", "-q", side, "side")
	writeFile(f.t, filepath.Join(side, "side.md"), "side\n")
	f.git(side, "add", ".")
	f.git(side, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-qm", "side")
	f.git(f.wt("tove", repo), "-c", "user.name=t", "-c", "user.email=t@t", "merge", "--no-commit", "--no-ff", "-q", "side")
}

// An agent of the project not at work frees its merged branches: in every
// repo where it has one checked out, or in none, saying why.
func TestCleanupOffersToFreeABranch(t *testing.T) {
	at := func(status string) func(f *fixture) {
		return func(f *fixture) {
			f.teams.instances = []store.Instance{{ID: "tove@ledger", DeploymentID: "atlas@ledger", AgentID: "tove", Status: status}}
		}
	}
	for _, tc := range []struct {
		name  string
		setup func(f *fixture)
		want  []string
	}{
		{"no deployment", func(*fixture) {}, []string{"api free tove", "web free tove"}},
		{"stopped", at("stopped"), []string{"api free tove", "web free tove"}},
		{"asleep", at("asleep"), []string{"api free tove", "web free tove"}},
		{"running", at("running"), []string{"api (checked out by tove, who is running)", "web (checked out by tove, who is running)"}},
		{"idle", at("idle"), []string{"api (checked out by tove, who is idle)", "web (checked out by tove, who is idle)"}},
		{"queued", at("queued"), []string{"api (checked out by tove, who is queued)", "web (checked out by tove, who is queued)"}},
		{"not merged in a sibling", func(f *fixture) {
			writeFile(f.t, filepath.Join(f.wt("tove", "web"), "wip.md"), "wip\n")
			f.git(f.wt("tove", "web"), "add", ".")
			f.git(f.wt("tove", "web"), "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-qm", "wip")
		}, []string{"api (checked out by tove, not merged in web)", "web (checked out by tove, not merged in web)"}},
		{"under review in a sibling", func(f *fixture) {
			if _, err := f.db.MergeRequests().Save(ctx, store.MergeRequest{ProjectID: "ledger", DeploymentID: "atlas@ledger",
				Repo: "web", Branch: toveMain, Target: "trunk", State: store.MergeOpen}); err != nil {
				f.t.Fatal(err)
			}
		}, []string{"api (checked out by tove, a review keeps it in web)", "web (checked out by tove, a review keeps it in web)"}},
		{"a merge under way", func(f *fixture) { f.mergeUnderWay("api") },
			[]string{"api (checked out by tove, a merge under way in api)", "web (checked out by tove, a merge under way in api)"}},
		{"a sibling with a problem", func(f *fixture) { f.problems["web"] = "missing" },
			[]string{"api (checked out by tove, web has a problem)"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			tc.setup(f)
			if got := f.rowsOf(toveMain); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got  %q\nwant %q", got, tc.want)
			}
		})
	}

	// A worktree git can't read is an error, never a reason made up.
	f := newFixture(t)
	if err := os.Remove(filepath.Join(f.wt("tove", "web"), ".git")); err != nil {
		t.Fatal(err)
	}
	if _, err := f.w.CleanupBranches(ctx, "ledger"); err == nil {
		t.Error("an unreadable worktree: no error")
	}

	// Another project's agent keeps its branch, as before.
	f = newFixture(t)
	api := filepath.Join(f.root, "ledger", "api")
	f.git(api, "worktree", "add", "-q", "-b", "wt/ledger/tove/elsewhere", filepath.Join(f.root, "suite", ".sandboxes", "tove", "core"), "main")
	if got := f.rowsOf("wt/ledger/tove/elsewhere"); !reflect.DeepEqual(got, []string{"api (checked out by tove@suite)"}) {
		t.Errorf("another project's agent: %q", got)
	}
}

// Picked in every repo, a free branch goes: each worktree detached at its
// commit, with its changes, the branch deleted (backed up), and the next
// start finds the repos idle. Otherwise nothing changes, and every sibling is
// kept with one why.
func TestCleanupFreesABranch(t *testing.T) {
	both := []BranchPick{{Repo: "api", Branch: toveMain}, {Repo: "web", Branch: toveMain}}
	f := newFixture(t)
	heads := map[string]string{}
	for _, r := range []string{"api", "web"} {
		heads[r] = f.git(f.wt("tove", r), "rev-parse", "HEAD")
		writeFile(t, filepath.Join(f.wt("tove", r), "notes.md"), "half a thought\n")
	}
	done, err := f.w.DeleteBranches(ctx, "ledger", both)
	if err != nil {
		t.Fatal(err)
	}
	want := []Cleaned{
		{Repo: "api", What: "branch", Branch: toveMain}, {Agent: "tove", Repo: "api", What: "detached", Branch: toveMain},
		{Repo: "web", What: "branch", Branch: toveMain}, {Agent: "tove", Repo: "web", What: "detached", Branch: toveMain},
	}
	if !reflect.DeepEqual(done.Removed, want) || len(done.Kept) != 0 {
		t.Errorf("removed %+v, kept %+v", done.Removed, done.Kept)
	}
	g := git.New()
	for _, r := range []string{"api", "web"} {
		dir, wt := filepath.Join(f.root, "ledger", r), f.wt("tove", r)
		if on := f.git(wt, "branch", "--show-current"); on != "" || f.git(wt, "rev-parse", "HEAD") != heads[r] {
			t.Errorf("%s: on %q", r, on)
		}
		if data, _ := os.ReadFile(filepath.Join(wt, "notes.md")); string(data) != "half a thought\n" {
			t.Errorf("%s: the changes went", r)
		}
		if g.HasBranch(ctx, dir, toveMain) || f.git(dir, "for-each-ref", "--format=%(objectname)", "refs/foreman/cleaned/") != heads[r] {
			t.Errorf("%s: the branch not deleted, or not kept", r)
		}
		if err := g.Prepare(ctx, dir, wt, "", mainOf(r)); err != nil {
			t.Fatal(err)
		}
		if on, _ := g.Branch(ctx, wt); on != "" {
			t.Errorf("%s: the next start found it on %q", r, on)
		}
	}

	for _, tc := range []struct {
		name  string
		setup func(f *fixture)
		picks []BranchPick
		why   string
	}{
		{"a sibling missing", func(*fixture) {}, both[:1], "pick it in every repo tove has it: api, web"},
		{"started meanwhile", func(f *fixture) {
			f.teams.instances = []store.Instance{{ID: "tove@ledger", DeploymentID: "atlas@ledger", AgentID: "tove", Status: "starting"}}
		}, both, "tove started meanwhile"},
		{"a merge under way", func(f *fixture) { f.mergeUnderWay("web") }, both, "a merge is under way in tove's worktree of web"},
		{"no longer merged", func(f *fixture) {
			writeFile(f.t, filepath.Join(f.wt("tove", "web"), "wip.md"), "wip\n")
			f.git(f.wt("tove", "web"), "add", ".")
			f.git(f.wt("tove", "web"), "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-qm", "wip")
		}, both, "no longer merged in web"},
		{"a sibling with a problem now", func(f *fixture) { f.problems["web"] = "missing" }, both[:1], "web has a problem"},
		{"under review in a sibling", func(f *fixture) {
			if _, err := f.db.MergeRequests().Save(ctx, store.MergeRequest{ProjectID: "ledger", DeploymentID: "atlas@ledger",
				Repo: "web", Branch: toveMain, Target: "trunk", State: store.MergeOpen}); err != nil {
				f.t.Fatal(err)
			}
		}, both, "a review keeps it in web"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			tc.setup(f)
			done, err := f.w.DeleteBranches(ctx, "ledger", tc.picks)
			if err != nil {
				t.Fatal(err)
			}
			if len(done.Removed) != 0 || len(done.Kept) == 0 {
				t.Fatalf("removed %+v, kept %+v", done.Removed, done.Kept)
			}
			for _, c := range done.Kept {
				if c.Why != tc.why || c.Branch != toveMain {
					t.Errorf("kept %+v, want why %q", c, tc.why)
				}
			}
			for _, r := range []string{"api", "web"} {
				if on := f.git(f.wt("tove", r), "branch", "--show-current"); on != toveMain {
					t.Errorf("%s: on %q", r, on)
				}
			}
			if refs := f.git(filepath.Join(f.root, "ledger", "api"), "for-each-ref", "refs/foreman/cleaned/"); refs != "" {
				t.Errorf("a backup of what stayed: %q", refs)
			}
		})
	}
}

// GET and POST read the agent's siblings the same way: a branch checked out
// in web by the CEO, not by tove, is no sibling of tove's in api.
func TestFreeingAgreesWithTheList(t *testing.T) {
	f := newFixture(t)
	web := filepath.Join(f.root, "ledger", "web")
	f.git(f.wt("tove", "web"), "checkout", "-q", "--detach")
	f.git(web, "checkout", "-q", toveMain)
	if got, want := f.rowsOf(toveMain), []string{"api free tove", "web (checked out in your checkout)"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got  %q\nwant %q", got, want)
	}
	done, err := f.w.DeleteBranches(ctx, "ledger", []BranchPick{{Repo: "api", Branch: toveMain}})
	if err != nil {
		t.Fatal(err)
	}
	want := []Cleaned{{Repo: "api", What: "branch", Branch: toveMain}, {Agent: "tove", Repo: "api", What: "detached", Branch: toveMain}}
	if !reflect.DeepEqual(done.Removed, want) || len(done.Kept) != 0 {
		t.Errorf("removed %+v, kept %+v", done.Removed, done.Kept)
	}
	if on := f.git(web, "branch", "--show-current"); on != toveMain {
		t.Errorf("your checkout of web is on %q", on)
	}
}
