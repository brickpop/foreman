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
	if len(all[0].Parents) != 2 || len(all[1].Parents) != 1 || len(all[3].Parents) != 0 || all[3].Parents == nil || all[0].Author == "" || all[0].Date.IsZero() || len(all[0].Hash) != 40 {
		t.Errorf("first %+v, second %+v", all[0], all[1])
	}
	// The branches at each commit, never HEAD or a tag; two at one commit both show.
	mustRun(t, dir, "tag", "v1", main)
	mustRun(t, dir, "branch", "lead/atlas/integration", "wt/tove/x")
	if all, _ = g.Log(ctx, dir, main, 0, 10); !slices.Equal(all[0].Branches, []string{"main"}) ||
		!slices.Equal(all[1].Branches, []string{"lead/atlas/integration", "wt/tove/x"}) || all[2].Branches == nil || len(all[2].Branches) != 0 {
		t.Errorf("branches %q, %q, %q", all[0].Branches, all[1].Branches, all[2].Branches)
	}
	page, _ := g.Log(ctx, dir, main, 2, 2)
	if len(page) != 2 || page[0].Subject != "app" || page[1].Subject != "first" {
		t.Errorf("skip 2: %+v", page)
	}
	if rest, err := g.Log(ctx, dir, main, 4, 2); err != nil || len(rest) != 0 {
		t.Errorf("past the end: %+v %v", rest, err)
	}
	// Every branch's: one main lacks too, each commit once, children first.
	first := all[len(all)-1].Hash
	side := mustRun(t, dir, "commit-tree", "-p", first, "-m", "on the side", first+"^{tree}")
	mustRun(t, dir, "update-ref", "refs/heads/side", side)
	every, err := g.Log(ctx, dir, "", 0, 10)
	if err != nil || len(every) != len(all)+1 || every[len(every)-1].Hash != first || !slices.ContainsFunc(every, func(c Commit) bool { return c.Subject == "on the side" }) {
		t.Errorf("every branch %+v %v", every, err)
	}
	// Count: the whole history, past any page; every branch's for "".
	if n, err := g.Count(ctx, dir, main); err != nil || n != len(all) {
		t.Errorf("count %d %v, want %d", n, err, len(all))
	}
	if n, err := g.Count(ctx, dir, ""); err != nil || n != len(every) {
		t.Errorf("count every branch %d %v, want %d", n, err, len(every))
	}
}

// By the last commit, not by name or creation: a branch made long ago and
// committed to just now comes first; same-second ties by name.
func TestRecentBranchesNewestFirst(t *testing.T) {
	g := New()
	dir := codeRepo(t, "main") // committed now
	commitAt := func(branch, date string) {
		t.Setenv("GIT_COMMITTER_DATE", date)
		mustRun(t, dir, "checkout", "--quiet", "-B", branch, "main")
		mustRun(t, dir, "commit", "--quiet", "--allow-empty", "-m", branch)
	}
	commitAt("a-old", "2020-01-01T10:00:00Z")
	commitAt("z-new", "2026-10-17T10:00:00Z")
	commitAt("m-mid", "2024-01-01T10:00:00Z")
	commitAt("b-mid", "2024-01-01T10:00:00Z")
	mustRun(t, dir, "tag", "v1")
	got, err := g.RecentBranches(ctx, dir)
	if want := []string{"main", "z-new", "b-mid", "m-mid", "a-old"}; err != nil || !slices.Equal(got, want) {
		t.Errorf("%v %v, want %v", got, err, want)
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

// The root commit names a repo wherever it goes: the same from a clone, the
// same after another history is merged in (the lowest hash of the two), none
// before the first commit.
func TestRootCommit(t *testing.T) {
	g := New()
	if _, err := g.RootCommit(ctx, repo(t)); !errors.Is(err, ErrNoRef) {
		t.Errorf("no commit yet: %v", err)
	}
	dir := codeRepo(t, "main")
	first, err := g.RootCommit(ctx, dir)
	if err != nil || len(first) != 40 {
		t.Fatalf("%q %v", first, err)
	}
	clone := filepath.Join(t.TempDir(), "clone")
	mustRun(t, dir, "clone", "--quiet", dir, clone)
	if got, _ := g.RootCommit(ctx, clone); got != first {
		t.Errorf("a clone: %s, want %s", got, first)
	}
	mustRun(t, dir, "checkout", "--quiet", "--orphan", "other")
	write(t, filepath.Join(dir, "other"), "x\n")
	mustRun(t, dir, "add", "other")
	mustRun(t, dir, "commit", "--quiet", "-m", "another history")
	second, _ := g.RootCommit(ctx, dir)
	mustRun(t, dir, "checkout", "--quiet", "main")
	mustRun(t, dir, "merge", "--quiet", "--allow-unrelated-histories", "-m", "join", "other")
	if got, _ := g.RootCommit(ctx, dir); got != min(first, second) || first == second {
		t.Errorf("after joining another history: %s, of %s and %s", got, first, second)
	}
}

// KeptHolding finds the kept refs holding a branch: a review's decided head
// that a branch went into, though no branch has it anymore.
func TestKeptHolding(t *testing.T) {
	g, dir := mergeFixture(t) // lead/atlas/x one commit ahead of main
	head := mustRun(t, dir, "rev-parse", "lead/atlas/x")
	mustRun(t, dir, "checkout", "-q", "-b", "lead/atlas/y", "lead/atlas/x")
	write(t, filepath.Join(dir, "more.go"), "package f\n")
	if err := g.Commit(ctx, dir, "more", "more.go"); err != nil {
		t.Fatal(err)
	}
	mustRun(t, dir, "checkout", "-q", "main")
	if err := g.Keep(ctx, dir, "refs/foreman/decided/mr-2", mustRun(t, dir, "rev-parse", "lead/atlas/y")); err != nil {
		t.Fatal(err)
	}
	if err := g.Keep(ctx, dir, "refs/foreman/decided/mr-1", head); err != nil {
		t.Fatal(err)
	}
	mustRun(t, dir, "branch", "-D", "lead/atlas/y") // no branch holds it now
	if got, err := g.KeptHolding(ctx, dir, "refs/foreman/decided/", "lead/atlas/x"); err != nil || !slices.Equal(got, []string{"mr-1", "mr-2"}) {
		t.Errorf("got %v %v", got, err)
	}
	if got, _ := g.KeptHolding(ctx, dir, "refs/foreman/decided/", "main"); !slices.Equal(got, []string{"mr-1", "mr-2"}) {
		t.Errorf("main, older: %v", got)
	}
	mustRun(t, dir, "checkout", "-q", "-b", "wt/tove/new", "main")
	write(t, filepath.Join(dir, "n"), "n\n")
	if err := g.Commit(ctx, dir, "n", "n"); err != nil {
		t.Fatal(err)
	}
	if got, err := g.KeptHolding(ctx, dir, "refs/foreman/decided/", "wt/tove/new"); err != nil || len(got) != 0 {
		t.Errorf("new work: %v %v", got, err)
	}
	for _, bad := range []string{"refs/heads/", "refs/foreman/decided"} {
		if _, err := g.KeptHolding(ctx, dir, bad, "main"); !errors.Is(err, ErrNoRef) {
			t.Errorf("prefix %q: %v", bad, err)
		}
	}
}

// Survives tells a branch whose every change is still in a kept head from one
// a merge dropped part of, or that was reworked since: only the first counts.
func TestSurvives(t *testing.T) {
	g, dir := mergeFixture(t) // main: README
	write(t, filepath.Join(dir, "old"), "old\n")
	if err := g.Commit(ctx, dir, "old", "old"); err != nil {
		t.Fatal(err)
	}
	// tove's branch edits README, adds b.go, deletes old.
	mustRun(t, dir, "checkout", "-q", "-b", "wt/tove/b", "main")
	write(t, filepath.Join(dir, "README"), "hello\nfrom b\n")
	write(t, filepath.Join(dir, "b.go"), "package b\n")
	mustRun(t, dir, "rm", "-q", "old")
	mustRun(t, dir, "add", ".")
	mustRun(t, dir, "commit", "-qm", "b")
	// kept merges b into a lead branch with a commit of the lead's own on top.
	kept := func(name string, resolve func()) {
		mustRun(t, dir, "checkout", "-q", "-b", "lead/atlas/"+name, "main")
		mustRun(t, dir, "merge", "-q", "--no-ff", "--no-commit", "wt/tove/b")
		resolve()
		mustRun(t, dir, "add", "-A")
		mustRun(t, dir, "commit", "-qm", "merge b")
		write(t, filepath.Join(dir, "lead.go"), "package lead\n")
		mustRun(t, dir, "add", ".")
		mustRun(t, dir, "commit", "-qm", "lead's own")
		mustRun(t, dir, "update-ref", "refs/foreman/decided/"+name, "HEAD")
		mustRun(t, dir, "checkout", "-q", "main")
	}
	kept("whole", func() {})
	kept("dropped", func() { mustRun(t, dir, "checkout", "main", "--", "README") })
	kept("reworked", func() { write(t, filepath.Join(dir, "b.go"), "package b // reworked\n") })
	kept("revived", func() { mustRun(t, dir, "checkout", "main", "--", "old") })
	for ref, want := range map[string]bool{"whole": true, "dropped": false, "reworked": false, "revived": false} {
		if got, err := g.Survives(ctx, dir, "wt/tove/b", "main", "refs/foreman/decided/"+ref); err != nil || got != want {
			t.Errorf("%s: %v %v, want %v", ref, got, err, want)
		}
	}
	// No change of its own: nothing to lose.
	mustRun(t, dir, "branch", "wt/tove/empty", "main")
	if got, err := g.Survives(ctx, dir, "wt/tove/empty", "main", "refs/foreman/decided/dropped"); err != nil || !got {
		t.Errorf("no changes: %v %v", got, err)
	}
	// from gone: can't be told, so not kept.
	if got, err := g.Survives(ctx, dir, "wt/tove/b", "gone", "refs/foreman/decided/whole"); err != nil || got {
		t.Errorf("from gone: %v %v", got, err)
	}
	if _, err := g.Survives(ctx, dir, "wt/tove/b", "main", "refs/heads/main"); !errors.Is(err, ErrNoRef) {
		t.Errorf("a ref outside refs/foreman/: %v", err)
	}
	if _, err := g.Survives(ctx, dir, "-x", "main", "refs/foreman/decided/whole"); !errors.Is(err, ErrBadBranch) {
		t.Errorf("a bad branch: %v", err)
	}
}

// Every file not committed is listed, whatever its state, by the name it has
// now: a rename once, a new folder file by file, spaces and quotes as they
// are; ignored files and committed ones never.
func TestUncommitted(t *testing.T) {
	g, _, tree := readFixture(t)
	write(t, filepath.Join(tree, ".gitignore"), "*.log\n")
	mustRun(t, tree, "add", ".gitignore")
	mustRun(t, tree, "commit", "--quiet", "-m", "ignore logs")
	if files, err := g.Uncommitted(ctx, tree); err != nil || !slices.Equal(files, []string{"notes.md"}) {
		t.Fatalf("an edit: %q %v", files, err)
	}
	mustRun(t, tree, "mv", "README", "READ ME \"now\"")
	if err := os.Remove(filepath.Join(tree, "src", "app.go")); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(tree, "new", "deep", "a.txt"), "x")
	write(t, filepath.Join(tree, "debug.log"), "x")
	write(t, filepath.Join(tree, "staged.txt"), "x")
	mustRun(t, tree, "add", "staged.txt")
	files, err := g.Uncommitted(ctx, tree)
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(files)
	if want := []string{"READ ME \"now\"", "new/deep/a.txt", "notes.md", "src/app.go", "staged.txt"}; !slices.Equal(files, want) {
		t.Errorf("got  %q\nwant %q", files, want)
	}
	mustRun(t, tree, "add", "--all")
	mustRun(t, tree, "commit", "--quiet", "-m", "all")
	if files, err := g.Uncommitted(ctx, tree); err != nil || len(files) != 0 {
		t.Errorf("all committed: %q %v", files, err)
	}
}
