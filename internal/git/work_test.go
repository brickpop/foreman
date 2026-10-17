package git

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

var tove = Author{Name: "Tove", Email: "tove@foreman.invalid"}

// worktrees makes a repo on main and a worktree of it per agent, each on its
// home branch, as a deploy does.
func worktrees(t *testing.T, agents ...string) (string, map[string]string) {
	t.Helper()
	r := codeRepo(t, "main")
	wts := map[string]string{}
	for _, a := range agents {
		wts[a] = filepath.Join(t.TempDir(), a, "api")
		if err := New().Prepare(ctx, r, wts[a], "wt/"+a+"/main", "main"); err != nil {
			t.Fatal(err)
		}
	}
	return r, wts
}

func gitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := run(ctx, dir, args...)
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(out)
}

func TestSwitchCreatesFromMainThenComesBack(t *testing.T) {
	r, wts := worktrees(t, "tove")
	g := New()
	// main moves after the home branch was made: a new branch starts from it.
	write(t, filepath.Join(r, "NEWS"), "v2\n")
	if err := g.Commit(ctx, r, "news", "NEWS"); err != nil {
		t.Fatal(err)
	}
	created, err := g.Switch(ctx, wts["tove"], "wt/tove/LED-44", "main")
	if err != nil || !created {
		t.Fatalf("created %v, %v", created, err)
	}
	if b := branchOf(t, wts["tove"]); b != "wt/tove/LED-44" {
		t.Errorf("on %q", b)
	}
	if _, err := os.Stat(filepath.Join(wts["tove"], "NEWS")); err != nil {
		t.Error("the new branch didn't start from main as it is now")
	}
	if created, err := g.Switch(ctx, wts["tove"], "wt/tove/main", "main"); err != nil || created {
		t.Errorf("back home: created %v, %v", created, err)
	}
	if b := branchOf(t, r); b != "main" {
		t.Errorf("the CEO's checkout moved to %q", b)
	}
}

func TestSwitchRefusals(t *testing.T) {
	_, wts := worktrees(t, "tove")
	g := New()
	if _, err := g.Switch(ctx, wts["tove"], "wt/tove/bad..name", "main"); !errors.Is(err, ErrBadBranch) {
		t.Errorf("bad name: %v", err)
	}
	if _, err := g.Switch(ctx, wts["tove"], "-x", "main"); !errors.Is(err, ErrBadBranch) {
		t.Errorf("option-like name: %v", err)
	}
	// Untracked files count: switching would carry them along.
	write(t, filepath.Join(wts["tove"], "draft.go"), "package x\n")
	if _, err := g.Switch(ctx, wts["tove"], "wt/tove/other", "main"); !errors.Is(err, ErrUncommitted) {
		t.Errorf("with a new file: %v", err)
	}
	if b := branchOf(t, wts["tove"]); b != "wt/tove/main" {
		t.Errorf("moved to %q anyway", b)
	}
	if _, err := g.Switch(ctx, wts["tove"], "wt/tove/other", "trunk"); err == nil {
		os.Remove(filepath.Join(wts["tove"], "draft.go"))
		t.Error("no base branch, yet no error")
	}
}

func TestCommitAllTakesEverythingAsTheAgent(t *testing.T) {
	_, wts := worktrees(t, "tove")
	wt := wts["tove"]
	write(t, filepath.Join(wt, "cal.go"), "package cal\n")
	write(t, filepath.Join(wt, "README"), "changed\n")
	sha, err := New().CommitAll(ctx, wt, "cal v0", tove)
	if err != nil {
		t.Fatal(err)
	}
	if head := gitOut(t, wt, "rev-parse", "HEAD"); head != sha {
		t.Errorf("returned %s, HEAD is %s", sha, head)
	}
	show := gitOut(t, wt, "show", "--name-only", "--format=%s|%an <%ae>|%cn", "HEAD")
	if !strings.Contains(show, "cal v0|Tove <tove@foreman.invalid>|Tove") || !strings.Contains(show, "cal.go") || !strings.Contains(show, "README") {
		t.Errorf("HEAD = %q", show)
	}
	if b := gitOut(t, wt, "branch", "--show-current"); b != "wt/tove/main" {
		t.Errorf("committed on %q", b)
	}
	if _, err := New().CommitAll(ctx, wt, "again", tove); !errors.Is(err, ErrNothingToCommit) {
		t.Errorf("nothing changed: %v", err)
	}
}

func TestCommitAsTakesOneFileAsTheAgent(t *testing.T) {
	dir := repo(t)
	write(t, filepath.Join(dir, "inbox", "note.md"), "hi\n")
	write(t, filepath.Join(dir, "lead-draft.md"), "unfinished\n")
	if err := New().CommitAs(ctx, dir, "contribution", tove, "inbox/note.md"); err != nil {
		t.Fatal(err)
	}
	show := gitOut(t, dir, "show", "--name-only", "--format=%an", "HEAD")
	if !strings.Contains(show, "Tove") || strings.Contains(show, "lead-draft.md") {
		t.Errorf("HEAD = %q", show)
	}
}

// Tove and Mara both change one line; Mara merges Tove's branch in.
func conflict(t *testing.T) (map[string]string, *CLI) {
	t.Helper()
	_, wts := worktrees(t, "tove", "mara")
	g := New()
	write(t, filepath.Join(wts["tove"], "README"), "tove's line\n")
	write(t, filepath.Join(wts["mara"], "README"), "mara's line\n")
	write(t, filepath.Join(wts["tove"], "my file.txt"), "a space in the name\n")
	write(t, filepath.Join(wts["mara"], "my file.txt"), "another\n")
	for _, a := range []string{"tove", "mara"} {
		if _, err := g.CommitAll(ctx, wts[a], a+"'s change", Author{Name: a, Email: a + "@foreman.invalid"}); err != nil {
			t.Fatal(err)
		}
	}
	return wts, g
}

func TestMergeLeavesConflictsThenCommitConcludesIt(t *testing.T) {
	wts, g := conflict(t)
	mara := Author{Name: "Mara", Email: "mara@foreman.invalid"}
	files, err := g.Merge(ctx, wts["mara"], "wt/tove/main", mara)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(files, "|") != "README|my file.txt" {
		t.Errorf("conflicts %q", files)
	}
	// While markers remain, commit refuses and names the files.
	var left *ConflictsError
	if _, err := g.CommitAll(ctx, wts["mara"], "merged", mara); !errors.As(err, &left) || len(left.Files) != 2 {
		t.Fatalf("with markers: %v", err)
	}
	// A second merge or a switch is refused while the merge is unfinished.
	if _, err := g.Merge(ctx, wts["mara"], "wt/tove/main", mara); !errors.Is(err, ErrUncommitted) {
		t.Errorf("merge mid-merge: %v", err)
	}
	if _, err := g.Switch(ctx, wts["mara"], "wt/mara/x", "main"); !errors.Is(err, ErrUncommitted) {
		t.Errorf("switch mid-merge: %v", err)
	}
	write(t, filepath.Join(wts["mara"], "README"), "both lines\n")
	if _, err := g.CommitAll(ctx, wts["mara"], "merged", mara); !errors.As(err, &left) || strings.Join(left.Files, "|") != "my file.txt" {
		t.Fatalf("one file left: %v", err)
	}
	write(t, filepath.Join(wts["mara"], "my file.txt"), "resolved\n")
	if _, err := g.CommitAll(ctx, wts["mara"], "merged", mara); err != nil {
		t.Fatal(err)
	}
	if parents := strings.Fields(gitOut(t, wts["mara"], "rev-list", "--parents", "-n1", "HEAD")); len(parents) != 3 {
		t.Errorf("not a merge commit: %v", parents)
	}
	if n, err := g.Ahead(ctx, wts["mara"], "wt/tove/main", "wt/mara/main"); err != nil || n != 0 {
		t.Errorf("tove's work not in mara's branch: %d, %v", n, err)
	}
}

// Deleting a conflicted file resolves it too.
func TestCommitConcludesAMergeResolvedByDeleting(t *testing.T) {
	wts, g := conflict(t)
	mara := Author{Name: "Mara", Email: "mara@foreman.invalid"}
	if _, err := g.Merge(ctx, wts["mara"], "wt/tove/main", mara); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(wts["mara"], "README"), "resolved\n")
	os.Remove(filepath.Join(wts["mara"], "my file.txt"))
	if _, err := g.CommitAll(ctx, wts["mara"], "merged", mara); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(wts["mara"], "my file.txt")); err == nil {
		t.Error("the deleted file came back")
	}
}

func TestMergeIsCleanWithoutConflicts(t *testing.T) {
	r, wts := worktrees(t, "tove", "mara")
	g := New()
	write(t, filepath.Join(wts["tove"], "cal.go"), "package cal\n")
	if _, err := g.CommitAll(ctx, wts["tove"], "cal", tove); err != nil {
		t.Fatal(err)
	}
	mara := Author{Name: "Mara", Email: "mara@foreman.invalid"}
	if files, err := g.Merge(ctx, wts["mara"], "wt/tove/main", mara); err != nil || files != nil {
		t.Fatalf("%v, %v", files, err)
	}
	if _, err := os.Stat(filepath.Join(wts["mara"], "cal.go")); err != nil {
		t.Error("tove's file didn't arrive")
	}
	// Up to date: nothing happens, no error.
	if files, err := g.Merge(ctx, wts["mara"], "wt/tove/main", mara); err != nil || files != nil {
		t.Errorf("again: %v, %v", files, err)
	}
	if n, _ := g.Ahead(ctx, r, "wt/tove/main", "main"); n != 1 {
		t.Errorf("ahead of main: %d", n)
	}
	write(t, filepath.Join(wts["mara"], "wip"), "x\n")
	if _, err := g.Merge(ctx, wts["mara"], "main", mara); !errors.Is(err, ErrUncommitted) {
		t.Errorf("with changes: %v", err)
	}
}

func TestHasBranch(t *testing.T) {
	_, wts := worktrees(t, "tove")
	g := New()
	if !g.HasBranch(ctx, wts["tove"], "main") || !g.HasBranch(ctx, wts["tove"], "wt/tove/main") || g.HasBranch(ctx, wts["tove"], "wt/tove/none") {
		t.Error("wrong answer")
	}
}

func TestAbortMergePutsTheBranchBack(t *testing.T) {
	wts, g := conflict(t)
	mara := Author{Name: "Mara", Email: "mara@foreman.invalid"}
	if err := g.AbortMerge(ctx, wts["mara"]); !errors.Is(err, ErrNoMerge) {
		t.Errorf("no merge yet: %v", err)
	}
	before := gitOut(t, wts["mara"], "rev-parse", "HEAD")
	if _, err := g.Merge(ctx, wts["mara"], "wt/tove/main", mara); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(wts["mara"], "README"), "half resolved\n")
	if err := g.AbortMerge(ctx, wts["mara"]); err != nil {
		t.Fatal(err)
	}
	if clean, err := g.Clean(ctx, wts["mara"]); err != nil || !clean {
		t.Errorf("not clean after the abort: %v, %v", clean, err)
	}
	if after := gitOut(t, wts["mara"], "rev-parse", "HEAD"); after != before {
		t.Error("the branch moved")
	}
	if data, _ := os.ReadFile(filepath.Join(wts["mara"], "README")); string(data) != "mara's line\n" {
		t.Errorf("README is %q", data)
	}
}

func TestBranchesAndContains(t *testing.T) {
	dir := repo(t)
	g := New()
	write(t, filepath.Join(dir, "a"), "a\n")
	if err := g.Commit(ctx, dir, "first", "a"); err != nil {
		t.Fatal(err)
	}
	for _, b := range []string{"lead/atlas/integration", "lead/atlas/x", "lead/atlasy/z", "wt/tove/LED-44"} {
		run(ctx, dir, "branch", b)
	}
	if got, err := g.Branches(ctx, dir, "lead/atlas/"); err != nil || !slices.Equal(got, []string{"lead/atlas/integration", "lead/atlas/x"}) {
		t.Errorf("branches %v %v", got, err)
	}
	if got, _ := g.Branches(ctx, dir, "lead/nobody/"); len(got) != 0 {
		t.Errorf("no such branches: %v", got)
	}
	// Tove commits on her branch: the lead's doesn't contain it until merged.
	run(ctx, dir, "checkout", "-q", "wt/tove/LED-44")
	write(t, filepath.Join(dir, "b"), "b\n")
	if err := g.Commit(ctx, dir, "tove", "b"); err != nil {
		t.Fatal(err)
	}
	if in, err := g.Contains(ctx, dir, "lead/atlas/integration", "wt/tove/LED-44"); err != nil || in {
		t.Errorf("before the merge: %v %v", in, err)
	}
	run(ctx, dir, "checkout", "-q", "lead/atlas/integration")
	if _, err := run(ctx, dir, "-c", "user.name=t", "-c", "user.email=t@t", "merge", "-q", "--no-ff", "-m", "merge", "wt/tove/LED-44"); err != nil {
		t.Fatal(err)
	}
	if in, err := g.Contains(ctx, dir, "lead/atlas/integration", "wt/tove/LED-44"); err != nil || !in {
		t.Errorf("after the merge: %v %v", in, err)
	}
	if _, err := g.Contains(ctx, dir, "lead/atlas/integration", "wt/tove/nope"); err == nil {
		t.Error("an unknown branch is an error, not 'not contained'")
	}
}
