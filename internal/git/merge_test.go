package git

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
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

// mergedCommit checks main's head is a merge commit of branch with message.
func mergedCommit(t *testing.T, dir, branch, message string) {
	t.Helper()
	if parents := strings.Fields(mustRun(t, dir, "log", "-1", "--format=%P", "main")); len(parents) != 2 || parents[1] != mustRun(t, dir, "rev-parse", branch) {
		t.Errorf("main's head isn't a merge of %s: parents %v", branch, parents)
	}
	if got := mustRun(t, dir, "log", "-1", "--format=%s", "main"); got != message {
		t.Errorf("message %q, want %q", got, message)
	}
}

func TestMergeIntoTheCheckedOutTarget(t *testing.T) {
	g, dir := mergeFixture(t)
	write(t, filepath.Join(dir, "notes.txt"), "the CEO's own, unrelated\n")
	// Fast-forwardable, but always a merge commit.
	if err := g.MergeInto(ctx, dir, "main", "lead/atlas/x", "Merge Settlement v2"); err != nil {
		t.Fatal(err)
	}
	mergedCommit(t, dir, "lead/atlas/x", "Merge Settlement v2")
	// The checkout's files moved with main; the CEO's unrelated file stayed.
	if _, err := os.Stat(filepath.Join(dir, "feature.go")); err != nil {
		t.Error("the checkout's files didn't move with main")
	}
	if status := mustRun(t, dir, "status", "--porcelain"); status != "?? notes.txt" {
		t.Errorf("status %q", status)
	}
}

func TestMergeIntoRefusesToOverwriteTheCEOsEdits(t *testing.T) {
	g, dir := mergeFixture(t)
	mustRun(t, dir, "checkout", "-q", "lead/atlas/x")
	write(t, filepath.Join(dir, "README"), "changed by the lead\n")
	if err := g.Commit(ctx, dir, "readme", "README"); err != nil {
		t.Fatal(err)
	}
	mustRun(t, dir, "checkout", "-q", "main")
	head := mustRun(t, dir, "rev-parse", "main")
	write(t, filepath.Join(dir, "README"), "the CEO's edit\n")
	if err := g.MergeInto(ctx, dir, "main", "lead/atlas/x", "Merge x"); !errors.Is(err, ErrLocalChanges) {
		t.Fatalf("err %v, want ErrLocalChanges", err)
	}
	if mustRun(t, dir, "rev-parse", "main") != head {
		t.Error("main moved")
	}
	if data, _ := os.ReadFile(filepath.Join(dir, "README")); string(data) != "the CEO's edit\n" {
		t.Errorf("the CEO's edit was lost: %q", data)
	}
}

func TestMergeIntoBacksOutOfAConflict(t *testing.T) {
	for _, checkedOut := range []bool{true, false} {
		g, dir := mergeFixture(t)
		mustRun(t, dir, "checkout", "-q", "lead/atlas/x")
		write(t, filepath.Join(dir, "README"), "the lead's\n")
		if err := g.Commit(ctx, dir, "lead", "README"); err != nil {
			t.Fatal(err)
		}
		mustRun(t, dir, "checkout", "-q", "main")
		write(t, filepath.Join(dir, "README"), "main's\n")
		if err := g.Commit(ctx, dir, "main", "README"); err != nil {
			t.Fatal(err)
		}
		if !checkedOut {
			mustRun(t, dir, "checkout", "-q", "-b", "elsewhere")
		}
		head := mustRun(t, dir, "rev-parse", "main")
		if err := g.MergeInto(ctx, dir, "main", "lead/atlas/x", "Merge x"); !errors.Is(err, ErrConflict) {
			t.Fatalf("checked out %v: err %v, want ErrConflict", checkedOut, err)
		}
		if mustRun(t, dir, "rev-parse", "main") != head || mustRun(t, dir, "status", "--porcelain") != "" {
			t.Errorf("checked out %v: something changed", checkedOut)
		}
	}
}

func TestMergeIntoWithoutTouchingAnotherCheckout(t *testing.T) {
	g, dir := mergeFixture(t)
	mustRun(t, dir, "checkout", "-q", "-b", "the-ceos-work")
	write(t, filepath.Join(dir, "draft.txt"), "wip\n")
	if err := g.MergeInto(ctx, dir, "main", "lead/atlas/x", "Merge Settlement v2"); err != nil {
		t.Fatal(err)
	}
	mergedCommit(t, dir, "lead/atlas/x", "Merge Settlement v2")
	if branch := mustRun(t, dir, "branch", "--show-current"); branch != "the-ceos-work" {
		t.Errorf("checkout moved to %s", branch)
	}
	if _, err := os.Stat(filepath.Join(dir, "feature.go")); err == nil {
		t.Error("the checkout's files changed")
	}
	if diff, err := g.Diff(ctx, dir, "main", "lead/atlas/x"); err != nil || diff != "" {
		t.Errorf("after the merge, nothing left to merge: %q %v", diff, err)
	}
}

func TestDiff(t *testing.T) {
	g, dir := mergeFixture(t)
	diff, err := g.Diff(ctx, dir, "main", "lead/atlas/x")
	if err != nil || !strings.Contains(diff, "+++ b/feature.go") {
		t.Errorf("diff %q %v", diff, err)
	}
	if _, err := g.Diff(ctx, dir, "main", "-x"); !errors.Is(err, ErrBadBranch) {
		t.Errorf("a bad branch: %v", err)
	}
}
