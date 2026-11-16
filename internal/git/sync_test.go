package git

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// machines: two clones of one HQ (a bare origin), as on two machines, each
// with one commit pushed.
func machines(t *testing.T) (g *CLI, a, b, origin string) {
	t.Helper()
	g = New()
	a = codeRepo(t, "main")
	origin = filepath.Join(t.TempDir(), "hq.git")
	mustRun(t, a, "clone", "--quiet", "--bare", a, origin)
	mustRun(t, a, "remote", "add", "origin", origin)
	mustRun(t, a, "fetch", "--quiet", "origin")
	b = filepath.Join(t.TempDir(), "b")
	mustRun(t, a, "clone", "--quiet", origin, b)
	return g, a, b, origin
}

// commitOn commits file with content in dir.
func commitOn(t *testing.T, dir, file, content string) string {
	t.Helper()
	write(t, filepath.Join(dir, file), content)
	mustRun(t, dir, "add", file)
	mustRun(t, dir, "commit", "--quiet", "-m", "edit "+file)
	return mustRun(t, dir, "rev-parse", "HEAD")
}

// The usual handover: one machine pushed, the other fast-forwards.
func TestPullFastForwards(t *testing.T) {
	g, a, b, _ := machines(t)
	head := commitOn(t, a, "teams.toml", "atlas\n")
	if err := g.Push(ctx, a, "main"); err != nil {
		t.Fatal(err)
	}
	if err := g.Pull(ctx, b, "main"); err != nil {
		t.Fatal(err)
	}
	if got := mustRun(t, b, "rev-parse", "HEAD"); got != head {
		t.Errorf("b at %s, want %s", got, head)
	}
	if err := g.Pull(ctx, b, "main"); err != nil {
		t.Errorf("nothing new: %v", err)
	}
	// Uncommitted changes the fast-forward doesn't touch stay as they are.
	write(t, filepath.Join(b, "draft.md"), "mine\n")
	head = commitOn(t, a, "index.md", "home\n")
	if err := g.Push(ctx, a, "main"); err != nil {
		t.Fatal(err)
	}
	if err := g.Pull(ctx, b, "main"); err != nil {
		t.Fatal(err)
	}
	if got := mustRun(t, b, "rev-parse", "HEAD"); got != head {
		t.Errorf("b at %s, want %s", got, head)
	}
	if data, _ := os.ReadFile(filepath.Join(b, "draft.md")); string(data) != "mine\n" {
		t.Errorf("the uncommitted file: %q", data)
	}
}

// Both machines committed: a merge, by the CEO's git, then a push.
func TestPullMergesWhatBothDid(t *testing.T) {
	g, a, b, _ := machines(t)
	theirs := commitOn(t, a, "a.md", "a\n")
	if err := g.Push(ctx, a, "main"); err != nil {
		t.Fatal(err)
	}
	ours := commitOn(t, b, "b.md", "b\n")
	if err := g.Push(ctx, b, "main"); err == nil {
		t.Error("pushed over origin's commits")
	}
	if err := g.Pull(ctx, b, "main"); err != nil {
		t.Fatal(err)
	}
	if parents := mustRun(t, b, "log", "-1", "--format=%P"); parents != ours+" "+theirs {
		t.Errorf("merge parents %q", parents)
	}
	if err := g.Push(ctx, b, "main"); err != nil {
		t.Errorf("push after the merge: %v", err)
	}
}

// A conflict leaves the HQ exactly as it was, saying which files.
func TestPullAbortsAConflict(t *testing.T) {
	g, a, b, _ := machines(t)
	commitOn(t, a, "README", "from a\n")
	if err := g.Push(ctx, a, "main"); err != nil {
		t.Fatal(err)
	}
	ours := commitOn(t, b, "README", "from b\n")
	err := g.Pull(ctx, b, "main")
	if !errors.Is(err, ErrConflict) || !strings.Contains(err.Error(), "README") {
		t.Errorf("err %v", err)
	}
	if merging, _ := g.Merging(ctx, b); merging || mustRun(t, b, "rev-parse", "HEAD") != ours {
		t.Error("left mid-merge, or moved")
	}
	if clean, _ := g.Clean(ctx, b); !clean {
		t.Error("left changes behind")
	}
}

// With uncommitted changes, only a fast-forward: a merge could lose them
// (git can't always put them back after an abort).
func TestPullNeverMergesOverUncommittedChanges(t *testing.T) {
	g, a, b, _ := machines(t)
	commitOn(t, a, "a.md", "a\n")
	if err := g.Push(ctx, a, "main"); err != nil {
		t.Fatal(err)
	}
	ours := commitOn(t, b, "b.md", "b\n")
	write(t, filepath.Join(b, "b.md"), "editing\n")
	if err := g.Pull(ctx, b, "main"); !errors.Is(err, ErrUncommitted) {
		t.Errorf("err %v", err)
	}
	if mustRun(t, b, "rev-parse", "HEAD") != ours {
		t.Error("moved")
	}
	if data, _ := os.ReadFile(filepath.Join(b, "b.md")); string(data) != "editing\n" {
		t.Errorf("the edit: %q", data)
	}
	// A fast-forward over a file being edited: git refuses, touching nothing.
	_, c, d, _ := machines(t)
	commitOn(t, c, "README", "new\n")
	if err := g.Push(ctx, c, "main"); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(d, "README"), "editing\n")
	if err := g.Pull(ctx, d, "main"); err == nil {
		t.Error("fast-forwarded over an edited file")
	}
	if data, _ := os.ReadFile(filepath.Join(d, "README")); string(data) != "editing\n" {
		t.Errorf("the edit: %q", data)
	}
}

func TestPullAndPushEdges(t *testing.T) {
	g := New()
	alone := codeRepo(t, "main")
	if err := g.Pull(ctx, alone, "main"); !errors.Is(err, ErrNoRemote) {
		t.Errorf("pull with no origin: %v", err)
	}
	if err := g.Push(ctx, alone, "main"); !errors.Is(err, ErrNoRemote) {
		t.Errorf("push with no origin: %v", err)
	}
	// A new, empty origin: nothing to pull, and the push makes its main.
	empty := filepath.Join(t.TempDir(), "empty.git")
	mustRun(t, alone, "init", "--quiet", "--bare", empty)
	mustRun(t, alone, "remote", "add", "origin", empty)
	if err := g.Pull(ctx, alone, "main"); err != nil {
		t.Errorf("an empty origin: %v", err)
	}
	if err := g.Push(ctx, alone, "main"); err != nil {
		t.Fatal(err)
	}
	if got := mustRun(t, empty, "rev-parse", "main"); got != mustRun(t, alone, "rev-parse", "main") {
		t.Errorf("origin's main %s", got)
	}
	// Not on the branch: never merged into another.
	_, a, b, _ := machines(t)
	commitOn(t, a, "a.md", "a\n")
	if err := g.Push(ctx, a, "main"); err != nil {
		t.Fatal(err)
	}
	mustRun(t, b, "checkout", "--quiet", "-b", "draft")
	if err := g.Pull(ctx, b, "main"); err == nil || !strings.Contains(err.Error(), "isn't on main") {
		t.Errorf("on another branch: %v", err)
	}
	// An origin that can't be reached fails, never hangs.
	mustRun(t, b, "checkout", "--quiet", "main")
	mustRun(t, b, "remote", "set-url", "origin", filepath.Join(t.TempDir(), "gone.git"))
	if err := g.Pull(ctx, b, "main"); err == nil {
		t.Error("pull from nowhere: no error")
	}
	if err := g.Push(ctx, b, "main"); err == nil {
		t.Error("push to nowhere: no error")
	}
}
