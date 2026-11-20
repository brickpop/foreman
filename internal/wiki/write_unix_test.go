//go:build unix

package wiki

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agentic-wiki/wiki/index"
)

// A card move that fails for a reason other than a refusal or a bad request
// (here the folder is read-only, which Unix enforces for a user other than
// root) is neither ErrRefused nor ErrInvalid: the server answers it with a 500.
func TestMoveCardFailingIsNotARefusal(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	w, dir := fixture(t, map[string]string{"wiki.toml": sprint, "backlog/x.md": task("status: todo\n")})
	e, _ := w.Entry("p", "backlog/x.md")
	backlog := filepath.Join(dir, "backlog")
	if err := os.Chmod(backlog, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(backlog, 0o755) })
	err := w.MoveCard(ctx, "p", "flat", "backlog/x.md", "done", "", e.Version)
	if err == nil || errors.Is(err, ErrRefused) || errors.Is(err, ErrInvalid) || errors.Is(err, ErrNotFound) || errors.Is(err, ErrConflict) {
		t.Errorf("MoveCard = %v, want a plain error", err)
	}
}

// A read-only checkout: the old tables can't be renamed, so nothing reads
// them, and the board says why. The file stays as it was.
func TestMigrationCantWrite(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	w, dir := fixture(t, map[string]string{"wiki.toml": oldBoards})
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o755) })
	got, err := w.Boards("p")
	if err != nil || len(got) != 1 || got[0].Name != "wiki.toml" || len(got[0].Problems) != 1 ||
		!strings.HasPrefix(got[0].Problems[0], "wiki.toml: could not rename the [tool.wikiview] tables to [[board]] and [[graph]]: ") {
		t.Errorf("boards %+v %v", got, err)
	}
	if toml := read(t, filepath.Join(dir, "wiki.toml")); toml != oldBoards {
		t.Errorf("wiki.toml = %q, want it untouched", toml)
	}
}

// A page that is a link out of the wiki, to a file elsewhere or to the repo's
// .git/config (a cloned repo can ship either), isn't an entry: no write
// reaches through it, and the targets stay as they were.
func TestWritesNeverLeaveTheWiki(t *testing.T) {
	w, dir := fixture(t, map[string]string{"wiki.toml": sprint, "backlog/x.md": task("status: todo\n")})
	outside := filepath.Join(t.TempDir(), "elsewhere.md")
	card := task("status: todo\n") + "- [ ] step\n"
	if err := os.WriteFile(outside, []byte(card), 0o644); err != nil {
		t.Fatal(err)
	}
	gitConfig := filepath.Join(filepath.Dir(dir), ".git", "config")
	before := read(t, gitConfig)
	for name, target := range map[string]string{"backlog/out.md": outside, "backlog/cfg.md": "../../.git/config"} {
		if err := os.Symlink(target, filepath.Join(dir, filepath.FromSlash(name))); err != nil {
			t.Fatal(err)
		}
	}
	for _, p := range []string{"backlog/out.md", "backlog/cfg.md"} {
		if _, err := w.Entry("p", p); err == nil {
			t.Errorf("%s is an entry", p)
		}
		if err := w.MoveCard(ctx, "p", "flat", p, "done", "", "any"); !errors.Is(err, ErrNotFound) {
			t.Errorf("MoveCard %s: %v, want ErrNotFound", p, err)
		}
		if err := w.SetCheckbox(ctx, "p", p, 5, true, "any"); !errors.Is(err, ErrNotFound) {
			t.Errorf("SetCheckbox %s: %v, want ErrNotFound", p, err)
		}
	}
	if read(t, outside) != card || read(t, gitConfig) != before {
		t.Error("a write reached through a link")
	}
}

// A page swapped for a link out of the wiki between write's read and the
// write itself (the worst moment: write builds a fresh index each time) is
// refused by the wiki library's containment and answered as a refusal (422),
// never written through.
func TestASwappedPageIsRefused(t *testing.T) {
	w, dir := fixture(t, map[string]string{"wiki.toml": sprint, "backlog/x.md": task("status: todo\n")})
	e, err := w.Entry("p", "backlog/x.md")
	if err != nil {
		t.Fatal(err)
	}
	page := filepath.Join(dir, "backlog", "x.md")
	outside := filepath.Join(t.TempDir(), "x.md")
	if err := os.WriteFile(outside, []byte(read(t, page)), 0o644); err != nil {
		t.Fatal(err)
	}
	err = w.write(ctx, "p", "backlog/x.md", e.Version, func(e *index.Entry) (string, error) {
		os.Remove(page)
		if err := os.Symlink(outside, page); err != nil {
			t.Fatal(err)
		}
		return "Move", e.SetFields(map[string]any{"status": "done"})
	})
	if !errors.Is(err, ErrRefused) || !strings.Contains(err.Error(), "outside the wiki") {
		t.Errorf("write = %v, want ErrRefused naming it", err)
	}
	if read(t, outside) != task("status: todo\n") {
		t.Error("the write went through the link")
	}
}
