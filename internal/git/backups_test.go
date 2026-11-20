package git

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestBackupRefCarriesItsTime(t *testing.T) {
	ctx := testCtx(t)
	at := time.Date(2026, 10, 22, 9, 30, 5, 0, time.FixedZone("CEST", 2*3600))
	if got := BackupRef("dismissed", at, "rev-3/lead/atlas/x"); got != "refs/foreman/dismissed/2026-10-22T07-30-05Z/rev-3/lead/atlas/x" {
		t.Errorf("%q", got)
	}
	if !ValidBranch(ctx, BackupRef("discarded", at, "tove/api")[len("refs/"):]) {
		t.Error("not a valid ref name")
	}
}

// Old backups go; recent ones, and refs whose name carries no time, stay.
func TestPruneBackups(t *testing.T) {
	ctx := testCtx(t)
	g, dir := mergeFixture(t)
	now := time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)
	old := BackupRef("dismissed", now.AddDate(0, 0, -31), "rev-1/lead/atlas/x")
	recent := BackupRef("discarded", now.AddDate(0, 0, -29), "tove/api")
	undated := "refs/foreman/dismissed/rev-0/lead/atlas/x"
	for _, ref := range []string{old, recent, undated} {
		mustRun(t, dir, "update-ref", ref, "lead/atlas/x")
	}
	if err := g.PruneBackups(ctx, dir, now.AddDate(0, 0, -30)); err != nil {
		t.Fatal(err)
	}
	if _, err := g.Resolve(ctx, dir, old); err == nil {
		t.Error("an old backup stayed")
	}
	for _, ref := range []string{recent, undated} {
		if _, err := g.Resolve(ctx, dir, ref); err != nil {
			t.Errorf("%s went", ref)
		}
	}
	if !g.HasBranch(ctx, dir, "lead/atlas/x") {
		t.Error("a branch went")
	}
}

func TestDiscard(t *testing.T) {
	ctx := testCtx(t)
	g, dir := mergeFixture(t)
	wt := filepath.Join(t.TempDir(), "tove")
	mustRun(t, dir, "worktree", "add", "-q", wt, "lead/atlas/x")
	backup := BackupRef("discarded", time.Now(), "tove/api")

	// Nothing to throw away: nothing kept.
	if saved, err := g.Discard(ctx, wt, backup); err != nil || saved {
		t.Fatalf("clean: %v %v", saved, err)
	}
	if _, err := g.Resolve(ctx, dir, backup); err == nil {
		t.Error("a backup of nothing")
	}

	write(t, filepath.Join(wt, ".gitignore"), "build/\n")
	mustRun(t, wt, "add", ".gitignore")
	mustRun(t, wt, "commit", "-q", "-m", "ignore build")
	head := mustRun(t, wt, "rev-parse", "HEAD")
	write(t, filepath.Join(wt, "feature.go"), "package f // changed\n")
	write(t, filepath.Join(wt, "new", "draft.go"), "package new\n")
	write(t, filepath.Join(wt, "build", "out.bin"), "built\n")
	mustRun(t, wt, "rm", "-q", "README")
	if saved, err := g.Discard(ctx, wt, backup); err != nil || !saved {
		t.Fatalf("discard: %v %v", saved, err)
	}
	// Back to the branch as committed; what is ignored stays.
	if mustRun(t, wt, "rev-parse", "HEAD") != head || mustRun(t, wt, "status", "--porcelain") != "" {
		t.Error("not back to the branch as committed")
	}
	if _, err := os.Stat(filepath.Join(wt, "new")); err == nil {
		t.Error("the new folder stayed")
	}
	if _, err := os.Stat(filepath.Join(wt, "build", "out.bin")); err != nil {
		t.Error("an ignored file went")
	}
	// Kept: a commit on the branch's head with everything thrown away.
	if mustRun(t, dir, "rev-parse", backup+"^") != head ||
		mustRun(t, dir, "show", backup+":feature.go") != "package f // changed" ||
		mustRun(t, dir, "show", backup+":new/draft.go") != "package new" {
		t.Error("what was thrown away isn't kept")
	}
	if out := mustRun(t, dir, "ls-tree", "--name-only", backup); out != ".gitignore\nfeature.go\nnew" {
		t.Errorf("kept tree %q: README deleted, build/ ignored", out)
	}

	// An unfinished merge: abort it first.
	write(t, filepath.Join(dir, "feature.go"), "package f // main's\n")
	mustRun(t, dir, "add", "feature.go")
	mustRun(t, dir, "commit", "-q", "-m", "main's")
	write(t, filepath.Join(wt, "feature.go"), "package f // the branch's\n")
	mustRun(t, wt, "commit", "-q", "-am", "the branch's")
	if _, err := run(ctx, wt, "-c", "user.name=t", "-c", "user.email=t@t", "merge", "main"); err == nil {
		t.Fatal("no conflict")
	}
	if _, err := g.Discard(ctx, wt, BackupRef("discarded", time.Now().Add(time.Second), "tove/api")); !errors.Is(err, ErrMerging) {
		t.Errorf("during a merge: %v", err)
	}
	if _, err := g.Discard(ctx, wt, "refs/heads/oops"); !errors.Is(err, ErrBadBranch) {
		t.Errorf("a backup among the branches: %v", err)
	}
}

func TestFreshen(t *testing.T) {
	ctx := testCtx(t)
	g, dir := mergeFixture(t)
	wt := filepath.Join(t.TempDir(), "tove")
	mustRun(t, dir, "worktree", "add", "-q", "--detach", wt, "main")
	write(t, filepath.Join(dir, "README"), "moved on\n")
	mustRun(t, dir, "commit", "-q", "-am", "main moves")
	main := mustRun(t, dir, "rev-parse", "main")

	// Changes not committed: the desk isn't idle.
	write(t, filepath.Join(wt, "scratch.txt"), "x\n")
	if moved, err := g.Freshen(ctx, wt, "main"); err != nil || moved {
		t.Errorf("moved over changes: %v %v", moved, err)
	}
	if err := os.Remove(filepath.Join(wt, "scratch.txt")); err != nil {
		t.Fatal(err)
	}
	// Idle, behind main: it follows, still on no branch.
	if moved, err := g.Freshen(ctx, wt, "main"); err != nil || !moved {
		t.Fatalf("idle: %v %v", moved, err)
	}
	if mustRun(t, wt, "rev-parse", "HEAD") != main || mustRun(t, wt, "status", "--porcelain") != "" || mustRun(t, wt, "branch", "--show-current") != "" {
		t.Error("the worktree didn't follow, detached")
	}
	// At main already: nothing to do.
	if moved, err := g.Freshen(ctx, wt, "main"); err != nil || moved {
		t.Errorf("at main: %v %v", moved, err)
	}
	// Detached on a commit main hasn't got (the lead's): it stays.
	mustRun(t, wt, "checkout", "-q", "--detach", "lead/atlas/x")
	at := mustRun(t, wt, "rev-parse", "HEAD")
	if moved, err := g.Freshen(ctx, wt, "main"); err != nil || moved || mustRun(t, wt, "rev-parse", "HEAD") != at {
		t.Errorf("moved over work: %v %v", moved, err)
	}
	// On a branch, even one main holds: a topic stays as it is.
	mustRun(t, wt, "checkout", "-q", "-b", "wt/tove/LED-1", main+"~1")
	if moved, err := g.Freshen(ctx, wt, "main"); err != nil || moved {
		t.Errorf("moved a topic: %v %v", moved, err)
	}
	if _, err := g.Freshen(ctx, wt, "-x"); !errors.Is(err, ErrBadBranch) {
		t.Errorf("option-like: %v", err)
	}
}

// A restart finds its idle worktree brought up to main; one on a topic stays.
func TestPrepareBringsAnIdleWorktreeUpToMain(t *testing.T) {
	ctx := testCtx(t)
	r := codeRepo(t, "main")
	idle, topic := filepath.Join(t.TempDir(), "mara"), filepath.Join(t.TempDir(), "tove")
	g := New()
	if err := g.Prepare(ctx, r, idle, "", "main"); err != nil {
		t.Fatal(err)
	}
	if err := g.Prepare(ctx, r, topic, "wt/tove/LED-1", "main"); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(r, "README"), "later\n")
	if err := g.Commit(ctx, r, "later", "README"); err != nil {
		t.Fatal(err)
	}
	for _, wt := range []string{idle, topic} {
		if err := g.Prepare(ctx, r, wt, "", "main"); err != nil {
			t.Fatal(err)
		}
	}
	if data, _ := os.ReadFile(filepath.Join(idle, "README")); string(data) != "later\n" {
		t.Errorf("README %q: the idle desk stayed behind", data)
	}
	if data, _ := os.ReadFile(filepath.Join(topic, "README")); string(data) == "later\n" {
		t.Error("a topic was moved")
	}
}
