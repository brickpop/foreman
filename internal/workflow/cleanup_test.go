package workflow

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/brickpop/foreman/internal/git"
	"github.com/brickpop/foreman/internal/store"
)

// Cleaning up removes only what is safe of what agents no longer running
// left: a clean worktree, and a wt/ branch its target holds that nothing has
// checked out. Work not committed, work in no target, and a running agent's
// worktrees all stay, each kept one said why.
func TestCleanUpRemovesOnlyWhatIsSafe(t *testing.T) {
	f := newFixture(t)
	f.teams.instances = []store.Instance{{ID: "mara@ledger", AgentID: "mara", DeploymentID: "atlas@ledger"}}
	api := filepath.Join(f.root, "ledger", "api")
	writeFile(t, filepath.Join(f.wt("tove", "web"), "draft.txt"), "not committed\n")
	// Work of tove's no target holds.
	writeFile(t, filepath.Join(api, "LED-1"), "fix\n")
	if err := git.New().Commit(ctx, api, "fix", "LED-1"); err != nil {
		t.Fatal(err)
	}
	f.git(api, "branch", "wt/tove/led-1", "HEAD")
	f.git(api, "reset", "-q", "--hard", "HEAD~1")

	got, err := f.w.CleanUp(ctx, "ledger")
	if err != nil {
		t.Fatal(err)
	}
	line := func(list []Cleaned) []string {
		var out []string
		for _, c := range list {
			out = append(out, strings.TrimSpace(c.Agent+" "+c.Repo+" "+c.What+" "+c.Branch+" "+c.Why))
		}
		return out
	}
	if want := []string{"tove api branch wt/tove/main", "tove api worktree wt/tove/main"}; !reflect.DeepEqual(line(got.Removed), want) {
		t.Errorf("removed %q\nwant    %q", line(got.Removed), want)
	}
	if want := []string{
		"tove api branch wt/tove/led-1 no target holds all of it",
		"tove web branch wt/tove/main it is checked out",
		"tove web worktree wt/tove/main it holds changes not committed",
	}; !reflect.DeepEqual(line(got.Kept), want) {
		t.Errorf("kept %q\nwant %q", line(got.Kept), want)
	}
	if _, err := os.Stat(f.wt("tove", "api")); !os.IsNotExist(err) {
		t.Errorf("tove's api worktree is still there: %v", err)
	}
	if data, _ := os.ReadFile(filepath.Join(f.wt("tove", "web"), "draft.txt")); string(data) != "not committed\n" {
		t.Errorf("tove's work went: %q", data)
	}
	for _, r := range []string{"api", "web"} {
		if _, err := os.Stat(f.wt("mara", r)); err != nil {
			t.Errorf("the running lead's %s worktree: %v", r, err)
		}
	}
	if !strings.Contains(f.git(api, "for-each-ref", "refs/foreman/cleaned/"), "wt/tove/main") {
		t.Error("the dropped branch's head wasn't kept aside")
	}
	if strings.Contains(f.git(api, "branch", "--list", "wt/tove/*"), "main") {
		t.Error("wt/tove/main is still in api")
	}
}
