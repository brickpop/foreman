//go:build unix

package wiki

import (
	"errors"
	"os"
	"path/filepath"
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
