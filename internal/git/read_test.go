package git

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// readFixture: main with a README, src/app.go, a symlink and a merge of
// wt/tove/x; that branch checked out in a worktree with an uncommitted edit.
func readFixture(t *testing.T) (*CLI, string, string) {
	t.Helper()
	g := New()
	dir := codeRepo(t, "main")
	write(t, filepath.Join(dir, "src", "app.go"), "package app\n")
	if err := os.Symlink("README", filepath.Join(dir, "link")); err != nil {
		t.Fatal(err)
	}
	mustRun(t, dir, "add", "src", "link")
	mustRun(t, dir, "commit", "--quiet", "-m", "app")
	mustRun(t, dir, "branch", "wt/tove/x")
	tree := filepath.Join(t.TempDir(), "tove")
	mustRun(t, dir, "worktree", "add", "--quiet", tree, "wt/tove/x")
	write(t, filepath.Join(tree, "notes.md"), "# Notes\n")
	mustRun(t, tree, "add", "notes.md")
	mustRun(t, tree, "commit", "--quiet", "-m", "notes")
	mustRun(t, dir, "merge", "--quiet", "--no-ff", "-m", "Merge wt/tove/x", "wt/tove/x")
	write(t, filepath.Join(tree, "notes.md"), "# Notes, edited\n") // not committed
	return g, dir, tree
}

func TestTreeAndFileAtACommit(t *testing.T) {
	g, dir, _ := readFixture(t)
	main, err := g.Resolve(ctx, dir, "main")
	if err != nil {
		t.Fatal(err)
	}
	root, err := g.Tree(ctx, dir, main, "")
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range root {
		names = append(names, e.Name+":"+e.Kind)
	}
	// Folders first, then by name.
	if want := []string{"src:dir", "README:file", "link:link", "notes.md:file"}; !slices.Equal(names, want) {
		t.Errorf("root %v, want %v", names, want)
	}
	for _, p := range []string{"src", "src/", "./src", "src/../src"} {
		if sub, err := g.Tree(ctx, dir, main, p); err != nil || len(sub) != 1 || sub[0].Name != "app.go" || sub[0].Size != 12 {
			t.Errorf("%q: %+v %v", p, sub, err)
		}
	}
	content, size, err := g.File(ctx, dir, main, "src/app.go", 1<<20)
	if err != nil || string(content) != "package app\n" || size != 12 {
		t.Errorf("file %q %d %v", content, size, err)
	}
	if content, size, err := g.File(ctx, dir, main, "src/app.go", 5); err != nil || content != nil || size != 12 {
		t.Errorf("over the limit: %q %d %v", content, size, err)
	}
	// What's committed, never the worktree: the branch has the first notes.
	tove, _ := g.Resolve(ctx, dir, "wt/tove/x")
	if content, _, _ := g.File(ctx, dir, tove, "notes.md", 1<<20); string(content) != "# Notes\n" {
		t.Errorf("notes %q", content)
	}
}

// A ref or a path from the browser can't reach anything else.
func TestReadRefusals(t *testing.T) {
	g, dir, _ := readFixture(t)
	for _, ref := range []string{"", "nope", "-h", "--all", "main:README", "main README"} {
		if _, err := g.Resolve(ctx, dir, ref); !errors.Is(err, ErrNoRef) {
			t.Errorf("ref %q: %v", ref, err)
		}
	}
	main, _ := g.Resolve(ctx, dir, "main")
	for _, p := range []string{"..", "../x", "/etc", "src/../../x", "README", "nope", "a\x00b"} {
		if _, err := g.Tree(ctx, dir, main, p); !errors.Is(err, ErrNoPath) {
			t.Errorf("tree %q: %v", p, err)
		}
	}
	for _, p := range []string{"", "src", "../README", "/README", "nope"} {
		if _, _, err := g.File(ctx, dir, main, p, 1<<20); !errors.Is(err, ErrNoPath) {
			t.Errorf("file %q: %v", p, err)
		}
	}
}

func TestLogPagesAndMarksMerges(t *testing.T) {
	g, dir, _ := readFixture(t)
	main, _ := g.Resolve(ctx, dir, "main")
	all, err := g.Log(ctx, dir, main, 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	var subjects []string
	for _, c := range all {
		subjects = append(subjects, c.Subject)
	}
	if want := []string{"Merge wt/tove/x", "notes", "app", "first"}; !slices.Equal(subjects, want) {
		t.Errorf("log %v, want %v", subjects, want)
	}
	if !all[0].Merge || all[1].Merge || all[0].Author == "" || all[0].Date.IsZero() || len(all[0].Hash) != 40 {
		t.Errorf("first %+v, second %+v", all[0], all[1])
	}
	page, _ := g.Log(ctx, dir, main, 2, 2)
	if len(page) != 2 || page[0].Subject != "app" || page[1].Subject != "first" {
		t.Errorf("skip 2: %+v", page)
	}
	if rest, err := g.Log(ctx, dir, main, 4, 2); err != nil || len(rest) != 0 {
		t.Errorf("past the end: %+v %v", rest, err)
	}
}

func TestWorktreesByBranch(t *testing.T) {
	g, dir, tree := readFixture(t)
	trees, err := g.Worktrees(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	// Paths as git reports them (a temp folder may sit behind a symlink).
	if !strings.HasSuffix(trees["wt/tove/x"], filepath.Base(tree)) || trees["main"] == "" {
		t.Errorf("worktrees %v", trees)
	}
	if clean, _ := g.Clean(ctx, trees["wt/tove/x"]); clean {
		t.Error("tove's uncommitted edit wasn't seen")
	}
}
