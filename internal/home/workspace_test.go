package home

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// legacyProject makes a project as Foreman made them before workspaces: its
// project repo in the home's projects/, no workspace.
func legacyProject(t *testing.T, h *Home, id string) Project {
	t.Helper()
	if err := h.createProjectRepo(ctx, id, h.Path("projects", id)); err != nil {
		t.Fatal(err)
	}
	if err := h.records.Create(ctx, id, ""); err != nil {
		t.Fatal(err)
	}
	p, err := h.Checked(id)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// A project made before workspaces keeps working where it is: its files in
// the home, its harness folders inside its sandboxes.
func TestALegacyProjectStaysInTheHome(t *testing.T) {
	h := loaded(t, nil)
	p := legacyProject(t, h, "ledger")
	if p.Workspace != "" || p.Path != h.Path("projects", "ledger") || p.Sandbox("mara") != h.Path("sandboxes", "ledger", "mara") ||
		p.HarnessDir("mara") != h.Path("sandboxes", "ledger", "mara", ".harness") || len(p.Problems) != 0 {
		t.Errorf("legacy %+v, sandbox %s, harness %s", p, p.Sandbox("mara"), p.HarnessDir("mara"))
	}
}

// Migrating moves the project repo and every sandbox into the workspace, the
// harness folders into the home's harness/, carries the conversations over
// and repairs the worktrees, so nothing is lost and a prune keeps them.
func TestMigrate(t *testing.T) {
	h := loaded(t, nil)
	type call struct{ dir, from, to string }
	var calls []call
	h.conversations = func(dir, from, to string) error {
		calls = append(calls, call{dir, from, to})
		return nil
	}
	old := legacyProject(t, h, "ledger")
	api := codeRepo(t, "dev/api")
	if _, err := h.AddRepo(ctx, "ledger", "api", api, ""); err != nil {
		t.Fatal(err)
	}
	tree := sandboxWorktree(t, h, "ledger", "mara", "api", api)
	writeFile(t, filepath.Join(tree, "wip.txt"), "uncommitted\n")
	writeFile(t, filepath.Join(old.HarnessDir("mara"), "claude-code", "projects", "x", "1.jsonl"), "{}\n")
	writeFile(t, filepath.Join(old.Path, "wiki", "notes.md"), "kept\n")
	ws := workspace("ledger")

	deployed := true
	h.deployed = func(context.Context, string) bool { return deployed }
	if _, err := h.Migrate(ctx, "ledger", ws); !errors.Is(err, ErrDeployed) {
		t.Errorf("while deployed: %v", err)
	}
	deployed = false
	if _, err := h.Migrate(ctx, "ledger", h.Path("ledger")); err == nil || !strings.Contains(err.Error(), "inside Foreman's home") {
		t.Errorf("into Foreman's home: %v", err)
	}
	if _, err := h.Migrate(ctx, "nope", ws); !errors.Is(err, ErrNotFound) {
		t.Errorf("no such project: %v", err)
	}
	if !exists(old.Path) || !exists(tree) || exists(ws) {
		t.Fatal("a refused migration moved something")
	}

	p, err := h.Migrate(ctx, "ledger", ws)
	if err != nil {
		t.Fatal(err)
	}
	newTree := filepath.Join(p.Sandbox("mara"), "api")
	if p.Workspace != ws || p.Path != filepath.Join(ws, "project") || newTree != filepath.Join(ws, "agents", "mara", "api") || len(p.Problems) != 0 {
		t.Errorf("migrated %+v", p)
	}
	if exists(old.Path) || exists(h.Path("sandboxes", "ledger")) {
		t.Error("left in the home")
	}
	if got, _ := os.ReadFile(filepath.Join(p.Path, "wiki", "notes.md")); string(got) != "kept\n" || gitOut(t, p.Path, "log", "--format=%s") != "Project repo" {
		t.Error("the project repo changed")
	}
	if got, _ := os.ReadFile(filepath.Join(newTree, "wip.txt")); string(got) != "uncommitted\n" {
		t.Error("the uncommitted work was lost")
	}
	// The repo knows where its worktree went: a prune keeps it, and git works in it.
	gitOut(t, api, "worktree", "prune")
	real, _ := filepath.EvalSymlinks(newTree)
	if list := gitOut(t, api, "worktree", "list", "--porcelain"); !strings.Contains(list, "worktree "+real+"\n") {
		t.Errorf("the repo lost the worktree:\n%s", list)
	}
	if gitOut(t, newTree, "branch", "--show-current") != "wt/mara/main" {
		t.Error("the worktree is broken")
	}
	harness := h.Path("harness", "ledger", "mara")
	if p.HarnessDir("mara") != harness || !exists(filepath.Join(harness, "claude-code", "projects", "x", "1.jsonl")) || exists(filepath.Join(p.Sandbox("mara"), ".harness")) {
		t.Errorf("the harness folder didn't move to %s", harness)
	}
	if len(calls) != 1 || calls[0] != (call{harness, old.Sandbox("mara"), p.Sandbox("mara")}) {
		t.Errorf("conversations carried %+v", calls)
	}
	reload(t, h)
	if again, _ := h.Project("ledger"); again.Workspace != ws {
		t.Errorf("after a restart: %+v", again)
	}
	if _, err := h.Migrate(ctx, "ledger", workspace("other")); !errors.Is(err, ErrInvalid) || !strings.Contains(err.Error(), "in its workspace already") {
		t.Errorf("migrated twice: %v", err)
	}
}

// A migration that fails on the way puts everything back where it was: the
// folders, the worktrees' links, the conversations, and the record.
func TestAFailedMigrationPutsEverythingBack(t *testing.T) {
	h := loaded(t, nil)
	var carried []string
	h.conversations = func(dir, from, to string) error {
		if strings.HasSuffix(from, "kai") {
			return errors.New("disk full")
		}
		carried = append(carried, from+" -> "+to)
		return nil
	}
	old := legacyProject(t, h, "relay")
	api := codeRepo(t, "dev/api")
	if _, err := h.AddRepo(ctx, "relay", "api", api, ""); err != nil {
		t.Fatal(err)
	}
	trees := map[string]string{}
	for _, agent := range []string{"ada", "kai"} {
		trees[agent] = sandboxWorktree(t, h, "relay", agent, "api", api)
		writeFile(t, filepath.Join(old.HarnessDir(agent), "pi", "sessions", "x", "1.jsonl"), "{}\n")
	}
	ws := filepath.Join(os.Getenv("HOME"), "new", "place", "relay")
	if _, err := h.Migrate(ctx, "relay", ws); err == nil || !strings.Contains(err.Error(), "disk full") {
		t.Fatalf("no failure: %v", err)
	}
	if exists(filepath.Join(os.Getenv("HOME"), "new")) || !exists(old.Path) {
		t.Error("the workspace stayed, or the project repo didn't come back")
	}
	for agent, tree := range trees {
		if !exists(filepath.Join(old.HarnessDir(agent), "pi", "sessions", "x", "1.jsonl")) {
			t.Errorf("%s's harness folder didn't come back", agent)
		}
		if gitOut(t, tree, "branch", "--show-current") != "wt/"+agent+"/main" {
			t.Errorf("%s's worktree is broken", agent)
		}
	}
	gitOut(t, api, "worktree", "prune")
	if list := gitOut(t, api, "worktree", "list"); strings.Count(list, "\n") != 2 {
		t.Errorf("the repo lost a worktree:\n%s", list)
	}
	// ada's were carried over, then carried back.
	if len(carried) != 2 || !strings.HasSuffix(carried[1], "-> "+old.Sandbox("ada")) {
		t.Errorf("conversations %v", carried)
	}
	if p, _ := h.Project("relay"); p.Workspace != "" || p.Path != old.Path {
		t.Errorf("the record moved: %+v", p)
	}
}
