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
// checked out, idle (on no branch) or not. Work not committed, work in no
// target, and a running agent's worktrees all stay, each kept one said why.
func TestCleanUpRemovesOnlyWhatIsSafe(t *testing.T) {
	f := newFixture(t)
	f.teams.instances = []store.Instance{{ID: "mara@ledger", AgentID: "mara", DeploymentID: "atlas@ledger"}}
	api := filepath.Join(f.root, "ledger", "api")
	writeFile(t, filepath.Join(f.wt("tove", "web"), "draft.txt"), "not committed\n")
	// ivo's, idle: api clean, web with an edit.
	f.ok("ivo", "open", map[string]string{"repo": "api"})
	f.ok("ivo", "open", map[string]string{"repo": "web"})
	writeFile(t, filepath.Join(f.wt("ivo", "web"), "idea.txt"), "not committed\n")
	// Work of tove's no target holds.
	writeFile(t, filepath.Join(api, "LED-1"), "fix\n")
	if err := git.New().Commit(ctx, api, "fix", "LED-1"); err != nil {
		t.Fatal(err)
	}
	f.git(api, "branch", "wt/ledger/tove/led-1", "HEAD")
	f.git(api, "reset", "-q", "--hard", "HEAD~1")

	got, err := f.w.CleanUp(ctx, "ledger", false)
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
	if want := []string{"ivo api worktree", "tove api branch wt/ledger/tove/main", "tove api worktree wt/ledger/tove/main"}; !reflect.DeepEqual(line(got.Removed), want) {
		t.Errorf("removed %q\nwant    %q", line(got.Removed), want)
	}
	if want := []string{
		"ivo web worktree  it holds changes not committed",
		"tove api branch wt/ledger/tove/led-1 not all of it is merged",
		"tove web branch wt/ledger/tove/main it is checked out",
		"tove web worktree wt/ledger/tove/main it holds changes not committed",
	}; !reflect.DeepEqual(line(got.Kept), want) {
		t.Errorf("kept %q\nwant %q", line(got.Kept), want)
	}
	if f.opened("ivo", "api") || !f.opened("ivo", "web") {
		t.Error("ivo's idle api should be gone, its web with the edit kept")
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
	if !strings.Contains(f.git(api, "for-each-ref", "refs/foreman/cleaned/"), "wt/ledger/tove/main") {
		t.Error("the dropped branch's head wasn't kept aside")
	}
	if strings.Contains(f.git(api, "branch", "--list", "wt/ledger/tove/*"), "main") {
		t.Error("wt/ledger/tove/main is still in api")
	}
}

// Asked to, Clean up removes worktrees holding changes not committed too,
// saying which files, each kept under refs/foreman/cleaned/ where git can
// bring it back. A running agent's stay, uncommitted or not.
func TestCleanUpRemovesUncommittedWorkWhenAsked(t *testing.T) {
	f := newFixture(t)
	f.teams.instances = []store.Instance{{ID: "mara@ledger", AgentID: "mara", DeploymentID: "atlas@ledger"}}
	web := filepath.Join(f.root, "ledger", "web")
	writeFile(t, filepath.Join(f.wt("tove", "web"), "notes", "draft.txt"), "not committed\n")
	writeFile(t, filepath.Join(f.wt("mara", "web"), "lead.txt"), "the lead's\n")
	f.ok("ivo", "open", map[string]string{"repo": "web"})
	writeFile(t, filepath.Join(f.wt("ivo", "web"), "idea.txt"), "idle, not committed\n")

	got, err := f.w.CleanUp(ctx, "ledger", true)
	if err != nil {
		t.Fatal(err)
	}
	byAgent := map[string]Cleaned{}
	for _, c := range got.Removed {
		if c.What == "worktree" && c.Repo == "web" {
			byAgent[c.Agent] = c
		}
	}
	for agent, file := range map[string]string{"tove": "notes/draft.txt", "ivo": "idea.txt"} {
		c := byAgent[agent]
		if !reflect.DeepEqual(c.Files, []string{file}) || !strings.HasPrefix(c.Backup, "refs/foreman/cleaned/") || !strings.HasSuffix(c.Backup, "/"+agent+"/web") {
			t.Errorf("%s's web: %+v", agent, c)
			continue
		}
		if _, err := os.Stat(f.wt(agent, "web")); !os.IsNotExist(err) {
			t.Errorf("%s's web worktree is still there: %v", agent, err)
		}
		if kept := f.git(web, "show", c.Backup+":"+file); !strings.Contains(kept, "not committed") {
			t.Errorf("%s's %s isn't kept: %q", agent, file, kept)
		}
	}
	for _, c := range append(got.Removed, got.Kept...) {
		if c.Agent == "mara" {
			t.Errorf("the running lead's: %+v", c)
		}
	}
	if data, _ := os.ReadFile(filepath.Join(f.wt("mara", "web"), "lead.txt")); string(data) != "the lead's\n" {
		t.Errorf("the running lead's work: %q", data)
	}
	// Clean worktrees go as they always do, with nothing to list.
	for _, c := range got.Removed {
		if c.What == "worktree" && c.Repo == "api" && (c.Files != nil || c.Backup != "") {
			t.Errorf("a clean worktree: %+v", c)
		}
	}
}

// An agent running on another project doesn't hold what it left here: its
// branches here carry this project, so they're this project's to clean.
func TestCleanUpIgnoresAgentsRunningElsewhere(t *testing.T) {
	f := newFixture(t)
	f.teams.instances = []store.Instance{{ID: "mara@suite", AgentID: "mara", DeploymentID: "atlas@suite"}}
	got, err := f.w.CleanUp(ctx, "ledger", false)
	if err != nil {
		t.Fatal(err)
	}
	removed := 0
	for _, c := range got.Removed {
		if c.Agent == "mara" && c.What == "worktree" {
			removed++
		}
	}
	if removed != 2 {
		t.Errorf("mara's worktrees here: removed %d of 2\n%+v", removed, got)
	}
	for _, r := range []string{"api", "web"} {
		if _, err := os.Stat(f.wt("mara", r)); !os.IsNotExist(err) {
			t.Errorf("mara's %s worktree is still there: %v", r, err)
		}
	}
}
