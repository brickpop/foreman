package git

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// mergeFixture: a repo on main with README, and lead/atlas/x one commit ahead
// adding feature.go.
func mergeFixture(t *testing.T) (*CLI, string) {
	t.Helper()
	dir := repo(t)
	g := New()
	write(t, filepath.Join(dir, "README"), "hello\n")
	if err := g.Commit(ctx, dir, "first", "README"); err != nil {
		t.Fatal(err)
	}
	mustRun(t, dir, "checkout", "-q", "-b", "lead/atlas/x")
	write(t, filepath.Join(dir, "feature.go"), "package f\n")
	if err := g.Commit(ctx, dir, "feature", "feature.go"); err != nil {
		t.Fatal(err)
	}
	mustRun(t, dir, "checkout", "-q", "main")
	return g, dir
}

func mustRun(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := run(ctx, dir, append([]string{"-c", "user.name=t", "-c", "user.email=t@t"}, args...)...)
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(out)
}

// squashed checks main's head is one commit on top of base, with message.
func squashed(t *testing.T, dir, base, message string) {
	t.Helper()
	if parents := mustRun(t, dir, "log", "-1", "--format=%P", "main"); parents != base {
		t.Errorf("main's head has parents %q, want only %s", parents, base)
	}
	if got := mustRun(t, dir, "log", "-1", "--format=%B", "main"); got != message {
		t.Errorf("message %q, want %q", got, message)
	}
}

// change commits file on the branch checked out in dir, by the author.
func change(t *testing.T, g *CLI, dir, file, text string, by Author) {
	t.Helper()
	write(t, filepath.Join(dir, file), text)
	if err := g.CommitAs(ctx, dir, file, by, file); err != nil {
		t.Fatal(err)
	}
}

func TestSquashTheCheckedOutTarget(t *testing.T) {
	g, dir := mergeFixture(t)
	mustRun(t, dir, "checkout", "-q", "lead/atlas/x")
	mara := Author{Name: "Mara", Email: "mara@foreman.invalid"}
	change(t, g, dir, "cal.go", "package cal\n", tove)
	change(t, g, dir, "pay.go", "package pay\n", mara)
	change(t, g, dir, "cal.go", "package cal // v2\n", tove)
	mustRun(t, dir, "checkout", "-q", "main")
	base, head := mustRun(t, dir, "rev-parse", "main"), mustRun(t, dir, "rev-parse", "lead/atlas/x")
	write(t, filepath.Join(dir, "notes.txt"), "the CEO's own, unrelated\n")
	if err := g.Squash(ctx, dir, "main", "main", "lead/atlas/x", head, "Settlement v2"); err != nil {
		t.Fatal(err)
	}
	// The message alone, however many authors wrote the branch.
	squashed(t, dir, base, "Settlement v2")
	if who := mustRun(t, dir, "log", "-1", "--format=%an|%cn", "main"); who != "Foreman|Foreman" {
		t.Errorf("by %q, want the CEO's identity (Foreman's with none)", who)
	}
	// The checkout's files moved with main; the CEO's unrelated file stayed.
	if data, _ := os.ReadFile(filepath.Join(dir, "cal.go")); string(data) != "package cal // v2\n" {
		t.Errorf("the checkout's files didn't move with main: %q", data)
	}
	if status := mustRun(t, dir, "status", "--porcelain"); status != "?? notes.txt" {
		t.Errorf("status %q", status)
	}
}

func TestSquashRefusesToOverwriteTheCEOsEdits(t *testing.T) {
	g, dir := mergeFixture(t)
	mustRun(t, dir, "checkout", "-q", "lead/atlas/x")
	change(t, g, dir, "README", "changed by the lead\n", tove)
	mustRun(t, dir, "checkout", "-q", "main")
	head := mustRun(t, dir, "rev-parse", "main")
	write(t, filepath.Join(dir, "README"), "the CEO's edit\n")
	if err := g.Squash(ctx, dir, "main", "main", "lead/atlas/x", mustRun(t, dir, "rev-parse", "lead/atlas/x"), "x"); !errors.Is(err, ErrLocalChanges) {
		t.Fatalf("err %v, want ErrLocalChanges", err)
	}
	if mustRun(t, dir, "rev-parse", "main") != head {
		t.Error("main moved")
	}
	if data, _ := os.ReadFile(filepath.Join(dir, "README")); string(data) != "the CEO's edit\n" {
		t.Errorf("the CEO's edit was lost: %q", data)
	}
}

func TestSquashBacksOutOfAConflict(t *testing.T) {
	for _, checkedOut := range []bool{true, false} {
		g, dir := mergeFixture(t)
		mustRun(t, dir, "checkout", "-q", "lead/atlas/x")
		change(t, g, dir, "README", "the lead's\n", tove)
		mustRun(t, dir, "checkout", "-q", "main")
		change(t, g, dir, "README", "main's\n", Author{})
		if !checkedOut {
			mustRun(t, dir, "checkout", "-q", "-b", "elsewhere")
		}
		head := mustRun(t, dir, "rev-parse", "main")
		if err := g.Squash(ctx, dir, "main", "main", "lead/atlas/x", mustRun(t, dir, "rev-parse", "lead/atlas/x"), "x"); !errors.Is(err, ErrConflict) {
			t.Fatalf("checked out %v: err %v, want ErrConflict", checkedOut, err)
		}
		if mustRun(t, dir, "rev-parse", "main") != head || mustRun(t, dir, "status", "--porcelain") != "" {
			t.Errorf("checked out %v: something changed", checkedOut)
		}
		if merged, err := g.Merged(ctx, dir, "main", "lead/atlas/x"); merged || err != nil {
			t.Errorf("checked out %v: a conflicting branch is merged? %v %v", checkedOut, merged, err)
		}
	}
}

func TestSquashWithoutTouchingAnotherCheckout(t *testing.T) {
	g, dir := mergeFixture(t)
	base := mustRun(t, dir, "rev-parse", "main")
	mustRun(t, dir, "checkout", "-q", "-b", "the-ceos-work")
	write(t, filepath.Join(dir, "draft.txt"), "wip\n")
	if err := g.Squash(ctx, dir, "main", "main", "lead/atlas/x", mustRun(t, dir, "rev-parse", "lead/atlas/x"), "Settlement v2"); err != nil {
		t.Fatal(err)
	}
	squashed(t, dir, base, "Settlement v2")
	if branch := mustRun(t, dir, "branch", "--show-current"); branch != "the-ceos-work" {
		t.Errorf("checkout moved to %s", branch)
	}
	if _, err := os.Stat(filepath.Join(dir, "feature.go")); err == nil {
		t.Error("the checkout's files changed")
	}
	if status := mustRun(t, dir, "status", "--porcelain"); status != "?? draft.txt" {
		t.Errorf("status %q", status)
	}
}

func TestSquashWhatMainHasAlready(t *testing.T) {
	g, dir := mergeFixture(t)
	if err := g.Squash(ctx, dir, "main", "main", "lead/atlas/x", mustRun(t, dir, "rev-parse", "lead/atlas/x"), "once"); err != nil {
		t.Fatal(err)
	}
	head := mustRun(t, dir, "rev-parse", "main")
	if err := g.Squash(ctx, dir, "main", "main", "lead/atlas/x", mustRun(t, dir, "rev-parse", "lead/atlas/x"), "twice"); err != nil {
		t.Fatal(err)
	}
	if mustRun(t, dir, "rev-parse", "main") != head {
		t.Error("an empty commit was made")
	}
}

func TestSquashOnlyTheCommitRead(t *testing.T) {
	g, dir := mergeFixture(t)
	read := mustRun(t, dir, "rev-parse", "lead/atlas/x")
	main := mustRun(t, dir, "rev-parse", "main")
	mustRun(t, dir, "checkout", "-q", "lead/atlas/x")
	change(t, g, dir, "late.go", "package late\n", tove)
	mustRun(t, dir, "checkout", "-q", "main")
	// The lead committed after the CEO read it: nothing is merged.
	for _, head := range []string{read, read[:12]} {
		if err := g.Squash(ctx, dir, "main", "main", "lead/atlas/x", head, "x"); !errors.Is(err, ErrMoved) {
			t.Errorf("%s: %v, want ErrMoved", head, err)
		}
	}
	for _, head := range []string{"", "main", "-x", "HEAD~1"} {
		if err := g.Squash(ctx, dir, "main", "main", "lead/atlas/x", head, "x"); !errors.Is(err, ErrNoRef) {
			t.Errorf("%q: %v, want ErrNoRef", head, err)
		}
	}
	if mustRun(t, dir, "rev-parse", "main") != main {
		t.Error("main moved")
	}
}

func TestMerged(t *testing.T) {
	g, dir := mergeFixture(t)
	merged := func(want bool) {
		t.Helper()
		if got, err := g.Merged(ctx, dir, "main", "lead/atlas/x"); got != want || err != nil {
			t.Errorf("merged %v %v, want %v", got, err, want)
		}
	}
	merged(false)
	if err := g.Squash(ctx, dir, "main", "main", "lead/atlas/x", mustRun(t, dir, "rev-parse", "lead/atlas/x"), "x"); err != nil {
		t.Fatal(err)
	}
	// Its commits aren't in main, its changes are.
	merged(true)
	// main moving on changes nothing; a branch that went back to main is merged too.
	change(t, g, dir, "README", "later\n", Author{})
	merged(true)
	mustRun(t, dir, "branch", "-f", "lead/atlas/x", "main")
	merged(true)
	if _, err := g.Merged(ctx, dir, "main", "-x"); !errors.Is(err, ErrBadBranch) {
		t.Errorf("a bad branch: %v", err)
	}
}

func TestMoveBranchNotCheckedOut(t *testing.T) {
	g, dir := mergeFixture(t)
	head := mustRun(t, dir, "rev-parse", "lead/atlas/x")
	if err := g.MoveBranch(ctx, dir, "lead/atlas/x", mustRun(t, dir, "rev-parse", "main"), "main"); !errors.Is(err, ErrMoved) {
		t.Errorf("from a stale commit: %v, want ErrMoved", err)
	}
	if mustRun(t, dir, "rev-parse", "lead/atlas/x") != head {
		t.Fatal("moved from a stale commit")
	}
	if err := g.MoveBranch(ctx, dir, "lead/atlas/x", head, "main"); err != nil {
		t.Fatal(err)
	}
	if mustRun(t, dir, "rev-parse", "lead/atlas/x") != mustRun(t, dir, "rev-parse", "main") {
		t.Error("not moved to main")
	}
}

func TestMoveBranchCheckedOutInAWorktree(t *testing.T) {
	g, dir := mergeFixture(t)
	wt := filepath.Join(t.TempDir(), "lead")
	mustRun(t, dir, "worktree", "add", "-q", wt, "lead/atlas/x")
	if err := g.Squash(ctx, dir, "main", "main", "lead/atlas/x", mustRun(t, dir, "rev-parse", "lead/atlas/x"), "x"); err != nil {
		t.Fatal(err)
	}
	change(t, g, dir, "README", "main moved on\n", Author{})
	head := mustRun(t, wt, "rev-parse", "HEAD")

	// A change git can't carry over: nothing moves.
	write(t, filepath.Join(wt, "README"), "the lead's, uncommitted\n")
	if err := g.MoveBranch(ctx, dir, "lead/atlas/x", head, "main"); err == nil {
		t.Error("moved over the lead's change to a file main changed")
	}
	if mustRun(t, wt, "rev-parse", "HEAD") != head {
		t.Fatal("moved anyway")
	}
	mustRun(t, wt, "checkout", "--", "README")

	// One it can: kept, and the files follow the branch.
	write(t, filepath.Join(wt, "feature.go"), "package f // the lead's, uncommitted\n")
	if err := g.MoveBranch(ctx, dir, "lead/atlas/x", head, "main"); err != nil {
		t.Fatal(err)
	}
	if mustRun(t, wt, "rev-parse", "HEAD") != mustRun(t, dir, "rev-parse", "main") || mustRun(t, wt, "branch", "--show-current") != "lead/atlas/x" {
		t.Error("the worktree's branch didn't move to main")
	}
	if data, _ := os.ReadFile(filepath.Join(wt, "README")); string(data) != "main moved on\n" {
		t.Errorf("the files didn't follow: %q", data)
	}
	if status := mustRun(t, wt, "status", "--porcelain"); status != "M feature.go" {
		t.Errorf("status %q, want the lead's change kept", status)
	}
	if diff, err := g.Diff(ctx, dir, "main", mustRun(t, dir, "rev-parse", "lead/atlas/x")); err != nil || diff != "" {
		t.Errorf("after the move, nothing left to review: %q %v", diff, err)
	}
}

func TestDiff(t *testing.T) {
	g, dir := mergeFixture(t)
	diff, err := g.Diff(ctx, dir, "main", mustRun(t, dir, "rev-parse", "lead/atlas/x"))
	if err != nil || !strings.Contains(diff, "+++ b/feature.go") {
		t.Errorf("diff %q %v", diff, err)
	}
	if _, err := g.Diff(ctx, dir, "-x", mustRun(t, dir, "rev-parse", "lead/atlas/x")); !errors.Is(err, ErrBadBranch) {
		t.Errorf("a bad branch: %v", err)
	}
	if _, err := g.Diff(ctx, dir, "main", "lead/atlas/x"); !errors.Is(err, ErrNoRef) {
		t.Errorf("a branch for the head: %v", err)
	}
}

func TestDiffIgnoresAHostileConfig(t *testing.T) {
	g, dir := mergeFixture(t)
	change(t, g, dir, "blank.txt", "a\n\nb\n", Author{})
	from := mustRun(t, dir, "rev-parse", "main")
	mustRun(t, dir, "checkout", "-q", "-B", "lead/atlas/x", "main")
	mustRun(t, dir, "mv", "README", "docs.md")
	write(t, filepath.Join(dir, "blank.txt"), "a\n\nB\n")
	mustRun(t, dir, "add", "blank.txt")
	mustRun(t, dir, "commit", "-q", "-m", "move")
	to := mustRun(t, dir, "rev-parse", "HEAD")
	mustRun(t, dir, "checkout", "-q", "main")

	// A config an agent could have written: programs for git to run, and
	// settings that change what the review screen reads.
	ran := filepath.Join(t.TempDir(), "ran")
	script := filepath.Join(t.TempDir(), "evil.sh")
	write(t, script, "#!/bin/sh\ntouch "+ran+"\necho evil\n")
	if err := os.Chmod(script, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, kv := range [][2]string{{"diff.external", script}, {"diff.ev.textconv", script},
		{"diff.noprefix", "true"}, {"color.diff", "always"}, {"diff.suppressBlankEmpty", "true"},
		{"diff.renames", "false"}} {
		mustRun(t, dir, "config", kv[0], kv[1])
	}
	write(t, filepath.Join(dir, mustRun(t, dir, "rev-parse", "--git-path", "info/attributes")), "* diff=ev\n")

	want := "diff --git a/blank.txt b/blank.txt\nindex a1a53b5..9c1e21d 100644\n--- a/blank.txt\n+++ b/blank.txt\n" +
		"@@ -1,3 +1,3 @@\n a\n \n-b\n+B\n" +
		"diff --git a/README b/docs.md\nsimilarity index 100%\nrename from README\nrename to docs.md\n"
	if diff, err := g.Diff(ctx, dir, "main", to); err != nil || diff != want {
		t.Errorf("diff %q %v", diff, err)
	}
	if diff, err := g.Compare(ctx, dir, from, to, 3); err != nil || diff != want {
		t.Errorf("compare %q %v", diff, err)
	}
	if _, err := os.Stat(ran); err == nil {
		t.Error("the config's program ran")
	}
}

func TestCompareTwoCommits(t *testing.T) {
	g, dir := mergeFixture(t)
	from := mustRun(t, dir, "rev-parse", "main")
	to := mustRun(t, dir, "rev-parse", "lead/atlas/x")
	diff, err := g.Compare(ctx, dir, from, to, 0)
	if err != nil || !strings.Contains(diff, "+++ b/feature.go") {
		t.Errorf("diff %q %v", diff, err)
	}
	// Only the paths named.
	if diff, err := g.Compare(ctx, dir, from, to, 3, "elsewhere.go"); err != nil || diff != "" {
		t.Errorf("another path %q %v", diff, err)
	}
	// A path is itself, never a pattern that names others.
	if diff, err := g.Compare(ctx, dir, from, to, 3, "*.go"); err != nil || diff != "" {
		t.Errorf("a pattern %q %v", diff, err)
	}
	for _, bad := range [][2]string{{"main", to}, {from, "--output=x"}, {from, ""}} {
		if _, err := g.Compare(ctx, dir, bad[0], bad[1], 0); !errors.Is(err, ErrNoRef) {
			t.Errorf("%q: %v", bad, err)
		}
	}
	if _, err := g.Compare(ctx, dir, from, to, -1); !errors.Is(err, ErrNoRef) {
		t.Errorf("negative context: %v", err)
	}
}

func TestDropBranch(t *testing.T) {
	g, dir := mergeFixture(t)
	head := mustRun(t, dir, "rev-parse", "lead/atlas/x")
	if err := g.DropBranch(ctx, dir, "lead/atlas/x", "refs/foreman/dismissed/rev-1/lead/atlas/x", "main"); err != nil {
		t.Fatal(err)
	}
	if g.HasBranch(ctx, dir, "lead/atlas/x") {
		t.Error("the branch is still there")
	}
	if kept := mustRun(t, dir, "rev-parse", "refs/foreman/dismissed/rev-1/lead/atlas/x"); kept != head {
		t.Errorf("backup at %s, want %s", kept, head)
	}
	// Not a branch: no listing shows it.
	if branches := mustRun(t, dir, "branch", "--list"); strings.Contains(branches, "dismissed") {
		t.Errorf("branches %q", branches)
	}
	for _, bad := range [][2]string{{"-x", "refs/foreman/dismissed/x"}, {"main", "refs/heads/other"}, {"main", ""}} {
		if err := g.DropBranch(ctx, dir, bad[0], bad[1], "main"); !errors.Is(err, ErrBadBranch) {
			t.Errorf("%q: %v", bad, err)
		}
	}
}

func TestDropBranchCheckedOutInAWorktree(t *testing.T) {
	g, dir := mergeFixture(t)
	wt := filepath.Join(t.TempDir(), "lead")
	mustRun(t, dir, "worktree", "add", "-q", wt, "lead/atlas/x")
	// A merge left unfinished, with its conflict, and a file not committed yet.
	write(t, filepath.Join(dir, "README"), "main's\n")
	mustRun(t, dir, "commit", "-q", "-am", "main's")
	write(t, filepath.Join(wt, "README"), "the lead's\n")
	mustRun(t, wt, "commit", "-q", "-am", "the lead's")
	head, main := mustRun(t, wt, "rev-parse", "HEAD"), mustRun(t, dir, "rev-parse", "main")
	if _, err := run(ctx, wt, "-c", "user.name=t", "-c", "user.email=t@t", "merge", "main"); err == nil {
		t.Fatal("no conflict to leave unfinished")
	}
	write(t, filepath.Join(wt, "notes.md"), "half a thought\n")

	backup := BackupRef("dismissed", time.Now(), "rev-1/lead/atlas/x")
	if err := g.DropBranch(ctx, dir, "lead/atlas/x", backup, "main"); err != nil {
		t.Fatal(err)
	}
	// It moves to main, the worktree on it and clean, the merge abandoned.
	if mustRun(t, wt, "rev-parse", "HEAD") != main || mustRun(t, wt, "branch", "--show-current") != "lead/atlas/x" {
		t.Error("the worktree's branch isn't at main")
	}
	if merging, _ := g.Merging(ctx, wt); merging {
		t.Error("still merging")
	}
	if status := mustRun(t, wt, "status", "--porcelain"); status != "" {
		t.Errorf("not clean: %q", status)
	}
	// Its commits and what it held not committed, each kept aside.
	if mustRun(t, dir, "rev-parse", backup) != head {
		t.Error("no backup of the branch")
	}
	saved := backup + "@uncommitted"
	if mustRun(t, dir, "rev-parse", saved+"^") != head ||
		mustRun(t, dir, "show", saved+":notes.md") != "half a thought" ||
		!strings.Contains(mustRun(t, dir, "show", saved+":README"), "<<<<<<<") {
		t.Error("what wasn't committed isn't kept as it was")
	}
}

// Where a branch's change starts, however main took it: open, squashed, by a
// merge commit; nothing to tell apart after a fast-forward.
func TestPartedFrom(t *testing.T) {
	for _, tc := range []struct {
		name  string
		merge func(t *testing.T, g *CLI, dir, head string)
		none  bool // a fast-forward: "" expected
	}{
		{"open", func(*testing.T, *CLI, string, string) {}, false},
		{"squashed", func(t *testing.T, g *CLI, dir, head string) {
			if err := g.Squash(ctx, dir, "main", "main", "lead/atlas/x", head, "Feature"); err != nil {
				t.Fatal(err)
			}
		}, false},
		{"merge commit", func(t *testing.T, _ *CLI, dir, _ string) {
			mustRun(t, dir, "merge", "-q", "--no-ff", "-m", "merge x", "lead/atlas/x")
			write(t, filepath.Join(dir, "after"), "later\n")
			mustRun(t, dir, "add", "after")
			mustRun(t, dir, "commit", "-q", "-m", "after the merge")
		}, false},
		{"fast-forward", func(t *testing.T, _ *CLI, dir, _ string) {
			mustRun(t, dir, "reset", "-q", "--hard", "lead/atlas/x")
		}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g, dir := mergeFixture(t)
			fork := mustRun(t, dir, "rev-parse", "main")
			head := mustRun(t, dir, "rev-parse", "lead/atlas/x")
			if tc.name != "fast-forward" { // main moves on meanwhile
				write(t, filepath.Join(dir, "other"), "other\n")
				mustRun(t, dir, "add", "other")
				mustRun(t, dir, "commit", "-q", "-m", "other work")
			}
			tc.merge(t, g, dir, head)
			got, err := g.PartedFrom(ctx, dir, "main", head)
			want := fork
			if tc.none {
				want = ""
			}
			if err != nil || got != want {
				t.Errorf("parted from %q %v, want %q", got, err, want)
			}
		})
	}
	g, dir := mergeFixture(t)
	head := mustRun(t, dir, "rev-parse", "lead/atlas/x")
	if _, err := g.PartedFrom(ctx, dir, "-x", head); !errors.Is(err, ErrBadBranch) {
		t.Errorf("a bad main: %v", err)
	}
	if _, err := g.PartedFrom(ctx, dir, "main", "lead/atlas/x"); !errors.Is(err, ErrNoRef) {
		t.Errorf("a branch for the head: %v", err)
	}
}

// A kept commit outlives its branch and the backups' pruning.
func TestKeep(t *testing.T) {
	g, dir := mergeFixture(t)
	head := mustRun(t, dir, "rev-parse", "lead/atlas/x")
	if err := g.Keep(ctx, dir, "refs/foreman/decided/mr-1", head); err != nil {
		t.Fatal(err)
	}
	mustRun(t, dir, "branch", "-D", "lead/atlas/x")
	if err := g.PruneBackups(ctx, dir, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if got := mustRun(t, dir, "rev-parse", "refs/foreman/decided/mr-1"); got != head {
		t.Errorf("kept %q, want %s", got, head)
	}
	for _, bad := range [][2]string{{"refs/heads/x", head}, {"refs/foreman/decided/mr-1", "main"}} {
		if err := g.Keep(ctx, dir, bad[0], bad[1]); !errors.Is(err, ErrNoRef) {
			t.Errorf("%q: %v", bad, err)
		}
	}
}

// DeleteBranch keeps the head aside and deletes the branch; a branch checked
// out anywhere (a worktree, or the repo's own checkout) stays, untouched.
func TestDeleteBranch(t *testing.T) {
	g, dir := mergeFixture(t)
	head := mustRun(t, dir, "rev-parse", "lead/atlas/x")
	backup := BackupRef("cleaned", time.Now(), "lead/atlas/x")
	if err := g.DeleteBranch(ctx, dir, "lead/atlas/x", backup); err != nil {
		t.Fatal(err)
	}
	if g.HasBranch(ctx, dir, "lead/atlas/x") {
		t.Error("the branch is still there")
	}
	if kept := mustRun(t, dir, "rev-parse", backup); kept != head {
		t.Errorf("backup at %s, want %s", kept, head)
	}
	if err := g.DeleteBranch(ctx, dir, "lead/atlas/x", backup+"2"); !errors.Is(err, ErrNoRef) {
		t.Errorf("a branch that isn't there: %v", err)
	}
	for _, bad := range [][2]string{{"-x", "refs/foreman/cleaned/x"}, {"main", "refs/heads/other"}, {"main", ""}} {
		if err := g.DeleteBranch(ctx, dir, bad[0], bad[1]); !errors.Is(err, ErrBadBranch) {
			t.Errorf("%q: %v", bad, err)
		}
	}
}

func TestDeleteBranchCheckedOut(t *testing.T) {
	g, dir := mergeFixture(t)
	wt := filepath.Join(t.TempDir(), "lead")
	mustRun(t, dir, "worktree", "add", "-q", wt, "lead/atlas/x")
	write(t, filepath.Join(wt, "notes.md"), "half a thought\n")
	head := mustRun(t, wt, "rev-parse", "HEAD")
	for _, b := range []string{"lead/atlas/x", "main"} { // a worktree's, the repo's own
		if err := g.DeleteBranch(ctx, dir, b, BackupRef("cleaned", time.Now(), b)); !errors.Is(err, ErrCheckedOut) {
			t.Errorf("%s: %v, want ErrCheckedOut", b, err)
		}
	}
	if mustRun(t, wt, "rev-parse", "HEAD") != head || mustRun(t, wt, "branch", "--show-current") != "lead/atlas/x" {
		t.Error("the worktree moved")
	}
	if data, _ := os.ReadFile(filepath.Join(wt, "notes.md")); string(data) != "half a thought\n" {
		t.Error("the worktree's work went")
	}
	if refs := mustRun(t, dir, "for-each-ref", "refs/foreman/"); refs != "" {
		t.Errorf("a backup of what stayed: %q", refs)
	}
}

// Squash onto a landing branch of its own (the CEO, 2026-11-11): made at the
// target's head with the squash on it, the target untouched; one already
// there is used only while at the target's head.
func TestSquashOntoALanding(t *testing.T) {
	g, dir := mergeFixture(t) // lead/atlas/x one commit ahead of main
	main := mustRun(t, dir, "rev-parse", "main")
	head := mustRun(t, dir, "rev-parse", "lead/atlas/x")
	if err := g.Squash(ctx, dir, "main", "f/feature", "lead/atlas/x", head, "Feature"); err != nil {
		t.Fatal(err)
	}
	if mustRun(t, dir, "rev-parse", "main") != main {
		t.Error("main moved")
	}
	if p := mustRun(t, dir, "log", "-1", "--format=%P %B", "f/feature"); p != main+" Feature" {
		t.Errorf("f/feature's head: %q", p)
	}
	if mustRun(t, dir, "show", "f/feature:feature.go") != "package f" {
		t.Error("the change isn't on f/feature")
	}

	// One already at main's head is used; one elsewhere is never moved.
	mustRun(t, dir, "branch", "f/same", "main")
	if err := g.Squash(ctx, dir, "main", "f/same", "lead/atlas/x", head, "Same"); err != nil {
		t.Fatal(err)
	}
	if p := mustRun(t, dir, "log", "-1", "--format=%P", "f/same"); p != main {
		t.Errorf("f/same's head has parents %q", p)
	}
	elsewhere := mustRun(t, dir, "rev-parse", "f/feature")
	if err := g.Squash(ctx, dir, "main", "f/feature", "lead/atlas/x", head, "Again"); !errors.Is(err, ErrBranchExists) {
		t.Errorf("onto a branch elsewhere: %v", err)
	}
	if mustRun(t, dir, "rev-parse", "f/feature") != elsewhere {
		t.Error("the branch elsewhere moved")
	}

	// Checked out in the CEO's checkout: its files follow.
	mustRun(t, dir, "branch", "f/out", "main")
	mustRun(t, dir, "checkout", "-q", "f/out")
	if err := g.Squash(ctx, dir, "main", "f/out", "lead/atlas/x", head, "Out"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "feature.go")); err != nil || mustRun(t, dir, "status", "--porcelain") != "" {
		t.Errorf("the checkout didn't follow: %v", err)
	}
	mustRun(t, dir, "checkout", "-q", "main")

	// A conflict makes nothing.
	mustRun(t, dir, "checkout", "-q", "lead/atlas/x")
	change(t, g, dir, "README", "the lead's\n", tove)
	mustRun(t, dir, "checkout", "-q", "main")
	change(t, g, dir, "README", "main's\n", Author{})
	if err := g.Squash(ctx, dir, "main", "f/conflict", "lead/atlas/x", mustRun(t, dir, "rev-parse", "lead/atlas/x"), "x"); !errors.Is(err, ErrConflict) {
		t.Errorf("a conflict: %v", err)
	}
	if g.HasBranch(ctx, dir, "f/conflict") {
		t.Error("a landing made though nothing merged")
	}

	// Nothing to add: the landing is still made, at main's head.
	mustRun(t, dir, "branch", "lead/atlas/none", "main")
	if err := g.Squash(ctx, dir, "main", "f/none", "lead/atlas/none", mustRun(t, dir, "rev-parse", "main"), "x"); err != nil {
		t.Fatal(err)
	}
	if mustRun(t, dir, "rev-parse", "f/none") != mustRun(t, dir, "rev-parse", "main") {
		t.Error("f/none isn't at main")
	}
	if err := g.Squash(ctx, dir, "main", "-x", "lead/atlas/x", head, "x"); !errors.Is(err, ErrBadBranch) {
		t.Errorf("a bad landing: %v", err)
	}
}
