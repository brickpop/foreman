//go:build unix

package wiki

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
