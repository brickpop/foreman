package wiki

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var ctx = context.Background()

// gitOut runs git in dir, for what a write left in the history.
func gitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func read(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

const sprint = "spec = \"0.1\"\n\n[[board]]\nid = \"sprint\"\npath = \"/backlog\"\nlane = \"priority\"\n\n" +
	"[[board]]\nid = \"flat\"\npath = \"/backlog\"\n"

func TestSetCheckbox(t *testing.T) {
	w, dir := fixture(t, map[string]string{"a.md": note("- [ ] one\n- [x] two\n\ntext\n")})
	e, _ := w.Entry("p", "a.md")

	if err := w.SetCheckbox(ctx, "p", "a.md", 4, true, e.Version); err != nil {
		t.Fatal(err)
	}
	if got := read(t, filepath.Join(dir, "a.md")); got != note("- [x] one\n- [x] two\n\ntext\n") {
		t.Errorf("file = %q: exactly one character should change", got)
	}
	// One commit, holding only that file, saying what it did.
	if log := gitOut(t, dir, "log", "-1", "--format=%s", "--name-only"); log != "Tick \"one\" in a.md\n\nwiki/a.md" {
		t.Errorf("commit = %q", log)
	}
	// Read back at once: the version moved, and the old one is now stale.
	after, _ := w.Entry("p", "a.md")
	if after.Version == e.Version || !after.Checkboxes[0].Done {
		t.Errorf("entry after = %+v", after.Checkboxes)
	}
	if err := w.SetCheckbox(ctx, "p", "a.md", 5, false, e.Version); !errors.Is(err, ErrConflict) {
		t.Errorf("a stale version: %v", err)
	}
	if err := w.SetCheckbox(ctx, "p", "a.md", 5, false, after.Version); err != nil {
		t.Fatal(err)
	}
	if log := gitOut(t, dir, "log", "-1", "--format=%s"); log != "Untick \"two\" in a.md" {
		t.Errorf("commit = %q", log)
	}

	now, _ := w.Entry("p", "a.md")
	for line, want := range map[int]error{7: ErrInvalid, 1: ErrInvalid, 99: ErrInvalid} {
		if err := w.SetCheckbox(ctx, "p", "a.md", line, true, now.Version); !errors.Is(err, want) {
			t.Errorf("line %d: %v, want %v", line, err, want)
		}
	}
	if err := w.SetCheckbox(ctx, "p", "../secret.md", 1, true, now.Version); !errors.Is(err, ErrNotFound) {
		t.Errorf("outside the wiki: %v", err)
	}
	if got := read(t, filepath.Join(dir, "a.md")); got != note("- [x] one\n- [ ] two\n\ntext\n") {
		t.Errorf("a refused write changed the file: %q", got)
	}
}

// The lead's uncommitted edits in that file aren't committed as the CEO's:
// the write lands in the file and waits for their next commit.
func TestWriteLeavesTheLeadsEditsUncommitted(t *testing.T) {
	w, dir := fixture(t, map[string]string{"a.md": note("- [ ] one\n"), "b.md": note("- [ ] two\n")})
	write(t, dir, map[string]string{"a.md": note("- [ ] one\n\nThe lead's draft.\n"), "c.md": note("new, uncommitted")})
	head := gitOut(t, dir, "rev-parse", "HEAD")

	a, _ := w.Entry("p", "a.md")
	if err := w.SetCheckbox(ctx, "p", "a.md", 4, true, a.Version); err != nil {
		t.Fatal(err)
	}
	if got := read(t, filepath.Join(dir, "a.md")); got != note("- [x] one\n\nThe lead's draft.\n") {
		t.Errorf("file = %q", got)
	}
	if gitOut(t, dir, "rev-parse", "HEAD") != head {
		t.Error("the lead's edits were committed as the CEO's")
	}

	// Another file's pending edits don't hold a clean one back, nor get committed with it.
	b, _ := w.Entry("p", "b.md")
	if err := w.SetCheckbox(ctx, "p", "b.md", 4, true, b.Version); err != nil {
		t.Fatal(err)
	}
	if files := gitOut(t, dir, "show", "--name-only", "--format=", "HEAD"); files != "wiki/b.md" {
		t.Errorf("committed %q", files)
	}
	if status := gitOut(t, dir, "status", "--porcelain"); !strings.Contains(status, "M wiki/a.md") || !strings.Contains(status, "?? wiki/c.md") {
		t.Errorf("status = %q", status)
	}
}

func TestMoveCard(t *testing.T) {
	w, dir := fixture(t, map[string]string{
		"wiki.toml":    sprint,
		"backlog/x.md": task("status: todo\npriority: low # keep me\nowner: me\n"),
		"notes/y.md":   task("status: todo\n"),
		"backlog/n.md": note(""),
		"backlog/z.md": task("priority: high\n"),
	})
	x, _ := w.Entry("p", "backlog/x.md")

	// Status and lane in one write, one commit; nothing else in the file moves.
	if err := w.MoveCard(ctx, "p", "sprint", "backlog/x.md", "done", "high", x.Version); err != nil {
		t.Fatal(err)
	}
	got := read(t, filepath.Join(dir, "backlog", "x.md"))
	if !strings.Contains(got, "status: done\n") || !strings.Contains(got, "priority: high") || !strings.Contains(got, "owner: me\n") {
		t.Errorf("file = %q", got)
	}
	if log := gitOut(t, dir, "log", "-1", "--format=%s", "--name-only"); log != "Move backlog/x.md to done, high\n\nwiki/backlog/x.md" {
		t.Errorf("commit = %q", log)
	}
	x, _ = w.Entry("p", "backlog/x.md")

	// No lane leaves the lane alone.
	if err := w.MoveCard(ctx, "p", "sprint", "backlog/x.md", "review", "", x.Version); err != nil {
		t.Fatal(err)
	}
	if got := read(t, filepath.Join(dir, "backlog", "x.md")); !strings.Contains(got, "status: review\n") || !strings.Contains(got, "priority: high") {
		t.Errorf("file = %q", got)
	}
	x, _ = w.Entry("p", "backlog/x.md")

	for _, c := range []struct {
		board, path, column, lane string
		want                      error
	}{
		{"sprint", "backlog/x.md", "", "", ErrInvalid},       // to no status: that's removing the field
		{"flat", "backlog/x.md", "done", "high", ErrInvalid}, // a lane on a board without lanes
		{"nope", "backlog/x.md", "done", "", ErrNotFound},
		{"#3", "backlog/x.md", "done", "", ErrNotFound},
		{"", "backlog/x.md", "done", "", ErrNotFound},
		{"sprint", "notes/y.md", "done", "", ErrNotFound}, // not on that board
		{"sprint", "backlog/n.md", "done", "", ErrNotFound},
	} {
		if err := w.MoveCard(ctx, "p", c.board, c.path, c.column, c.lane, x.Version); !errors.Is(err, c.want) {
			t.Errorf("%+v: %v", c, err)
		}
	}
	if err := w.MoveCard(ctx, "p", "sprint", "backlog/x.md", "done", "", "stale"); !errors.Is(err, ErrConflict) {
		t.Errorf("a stale version: %v", err)
	}
	// A card with no status yet can be given one.
	z, _ := w.Entry("p", "backlog/z.md")
	if err := w.MoveCard(ctx, "p", "sprint", "backlog/z.md", "todo", "", z.Version); err != nil {
		t.Fatal(err)
	}
	if got := read(t, filepath.Join(dir, "backlog", "z.md")); !strings.Contains(got, "status: todo\n") {
		t.Errorf("file = %q", got)
	}
}

// While watched, a write is read back at once, and the watcher still tells
// everyone else.
func TestWriteWhileWatched(t *testing.T) {
	w, _ := fixture(t, map[string]string{"a.md": note("- [ ] one\n")})
	changes := make(chan Changed, 4)
	wctx, cancel := context.WithCancel(ctx)
	t.Cleanup(cancel)
	if _, err := w.Watch(wctx, func() []string { return []string{"p"} }, func(c Changed) { changes <- c }); err != nil {
		t.Fatal(err)
	}
	e, _ := w.Entry("p", "a.md")
	if err := w.SetCheckbox(ctx, "p", "a.md", 4, true, e.Version); err != nil {
		t.Fatal(err)
	}
	if after, _ := w.Entry("p", "a.md"); !after.Checkboxes[0].Done {
		t.Error("the write isn't read back at once")
	}
	select {
	case c := <-changes:
		if c.Paths[0] != "a.md" {
			t.Errorf("change %+v", c)
		}
	case <-time.After(3 * time.Second):
		t.Error("no wiki.changed for the write")
	}
}
