package git

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

var ctx = context.Background()

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func repo(t *testing.T) string {
	t.Helper()
	// No identity from the machine running the tests: the fallback must work.
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	dir := t.TempDir()
	if err := (New()).Init(ctx, dir); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestCommitTakesOnlyTheNamedFiles(t *testing.T) {
	dir := repo(t)
	write(t, filepath.Join(dir, "teams", "atlas.toml"), "team = 'atlas'\n")
	write(t, filepath.Join(dir, "scratch.txt"), "not mine\n")
	if err := (New()).Commit(ctx, dir, "Atlas: saved in the UI", "teams/atlas.toml"); err != nil {
		t.Fatal(err)
	}
	files, _ := run(ctx, dir, "show", "--name-only", "--format=%s|%an", "HEAD")
	if !strings.Contains(files, "Atlas: saved in the UI|Foreman") || !strings.Contains(files, "teams/atlas.toml") || strings.Contains(files, "scratch.txt") {
		t.Errorf("HEAD = %q", files)
	}
	if status, _ := run(ctx, dir, "status", "--porcelain"); !strings.Contains(status, "?? scratch.txt") {
		t.Errorf("the other file was touched: %q", status)
	}
}

func TestCommittingUnchangedFilesSaysSo(t *testing.T) {
	dir := repo(t)
	write(t, filepath.Join(dir, "settings.toml"), "port = 8888\n")
	if err := (New()).Commit(ctx, dir, "first", "settings.toml"); err != nil {
		t.Fatal(err)
	}
	if err := (New()).Commit(ctx, dir, "again", "settings.toml"); !errors.Is(err, ErrNothingToCommit) {
		t.Errorf("second commit: %v", err)
	}
}

// A hook planted in the repo never runs on the host.
func TestHooksNeverRun(t *testing.T) {
	dir := repo(t)
	marker := filepath.Join(t.TempDir(), "hook-ran")
	write(t, filepath.Join(dir, ".git", "hooks", "pre-commit"), "#!/bin/sh\ntouch "+marker+"\n")
	write(t, filepath.Join(dir, ".git", "hooks", "post-commit"), "#!/bin/sh\ntouch "+marker+"\n")
	_ = os.Chmod(filepath.Join(dir, ".git", "hooks", "pre-commit"), 0o755)
	_ = os.Chmod(filepath.Join(dir, ".git", "hooks", "post-commit"), 0o755)
	// A hooks path set in the repo's own config is overridden too.
	if _, err := run(ctx, dir, "config", "core.hooksPath", ".git/hooks"); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(dir, "f"), "x\n")
	if err := (New()).Commit(ctx, dir, "m", "f"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(marker); err == nil {
		t.Error("a hook ran")
	}
}

func TestInitIsIdempotent(t *testing.T) {
	dir := repo(t)
	write(t, filepath.Join(dir, "f"), "x\n")
	if err := (New()).Commit(ctx, dir, "m", "f"); err != nil {
		t.Fatal(err)
	}
	if err := (New()).Init(ctx, dir); err != nil {
		t.Fatal(err)
	}
	if log, _ := run(ctx, dir, "log", "--oneline"); !strings.Contains(log, "m") {
		t.Errorf("re-init lost history: %q", log)
	}
}

// A machine that signs every commit doesn't make Foreman's commits prompt or
// fail: they are unsigned.
func TestCommitsAreUnsignedEvenWhenGitSignsByDefault(t *testing.T) {
	dir := repo(t)
	for _, kv := range [][2]string{{"commit.gpgsign", "true"}, {"gpg.format", "ssh"}, {"user.signingkey", "/no/such/key"}} {
		if _, err := run(ctx, dir, "config", kv[0], kv[1]); err != nil {
			t.Fatal(err)
		}
	}
	write(t, filepath.Join(dir, "f"), "x\n")
	if err := (New()).Commit(ctx, dir, "m", "f"); err != nil {
		t.Errorf("commit with signing configured: %v", err)
	}
}

// codeRepo makes a repo with one commit on main, like a code repo the CEO cloned.
func codeRepo(t *testing.T, branch string) string {
	t.Helper()
	dir := repo(t)
	if branch != "main" {
		if _, err := run(ctx, dir, "checkout", "--quiet", "-b", branch); err != nil {
			t.Fatal(err)
		}
	}
	write(t, filepath.Join(dir, "README"), "hello\n")
	if err := New().Commit(ctx, dir, "first", "README"); err != nil {
		t.Fatal(err)
	}
	return dir
}

func branchOf(t *testing.T, dir string) string {
	t.Helper()
	out, err := run(ctx, dir, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(out)
}

func TestPrepareMakesAWorktreeOnANewBranch(t *testing.T) {
	r := codeRepo(t, "main")
	wt := filepath.Join(t.TempDir(), "sandboxes", "mara", "api")
	if err := New().Prepare(ctx, r, wt, "wt/mara/main", "main"); err != nil {
		t.Fatal(err)
	}
	if b := branchOf(t, wt); b != "wt/mara/main" {
		t.Errorf("branch %q", b)
	}
	if data, _ := os.ReadFile(filepath.Join(wt, "README")); string(data) != "hello\n" {
		t.Errorf("worktree content %q", data)
	}
	if b := branchOf(t, r); b != "main" {
		t.Errorf("the CEO's checkout moved to %q", b)
	}
}

// A worktree already there is the agent's: it is left as it is, on whatever
// branch, with its uncommitted work.
func TestPrepareReusesAnExistingWorktree(t *testing.T) {
	r := codeRepo(t, "main")
	wt := filepath.Join(t.TempDir(), "mara")
	g := New()
	if err := g.Prepare(ctx, r, wt, "wt/mara/main", "main"); err != nil {
		t.Fatal(err)
	}
	if _, err := run(ctx, wt, "checkout", "--quiet", "-b", "wt/mara/led-44"); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(wt, "work.txt"), "in progress\n")
	if err := g.Prepare(ctx, r, wt, "wt/mara/main", "main"); err != nil {
		t.Fatal(err)
	}
	if b := branchOf(t, wt); b != "wt/mara/led-44" {
		t.Errorf("reuse switched the branch to %q", b)
	}
	if _, err := os.Stat(filepath.Join(wt, "work.txt")); err != nil {
		t.Error("reuse lost the uncommitted work")
	}
}

// The agent's branch survives its worktree: a new worktree picks it up as it
// was, not reset to main.
func TestPrepareChecksOutAnExistingBranch(t *testing.T) {
	r := codeRepo(t, "main")
	g := New()
	first := filepath.Join(t.TempDir(), "mara")
	if err := g.Prepare(ctx, r, first, "wt/mara/main", "main"); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(first, "done.txt"), "x\n")
	if err := g.Commit(ctx, first, "work", "done.txt"); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(first); err != nil {
		t.Fatal(err)
	}
	again := filepath.Join(t.TempDir(), "mara")
	if err := g.Prepare(ctx, r, again, "wt/mara/main", "main"); err != nil {
		t.Fatalf("after the old folder was deleted: %v", err)
	}
	if _, err := os.Stat(filepath.Join(again, "done.txt")); err != nil {
		t.Error("the branch's commit is missing: it was recreated from main")
	}
}

func TestPrepareRefusals(t *testing.T) {
	g := New()
	r := codeRepo(t, "main")

	// The CEO has the branch checked out in their own checkout.
	if _, err := run(ctx, r, "checkout", "--quiet", "-b", "wt/mara/main"); err != nil {
		t.Fatal(err)
	}
	if err := g.Prepare(ctx, r, filepath.Join(t.TempDir(), "x"), "wt/mara/main", "main"); err == nil {
		t.Error("a branch checked out elsewhere was checked out again")
	}

	master := codeRepo(t, "master")
	if err := g.Prepare(ctx, master, filepath.Join(t.TempDir(), "x"), "wt/mara/main", "main"); err == nil || !strings.Contains(err.Error(), "set `main` in project.toml") {
		t.Errorf("a repo without main: %v", err)
	}
	if err := g.Prepare(ctx, master, filepath.Join(t.TempDir(), "x"), "wt/mara/main", "master"); err != nil {
		t.Errorf("with its own main branch named: %v", err)
	}

	empty := repo(t)
	if err := g.Prepare(ctx, empty, filepath.Join(t.TempDir(), "x"), "wt/mara/main", "main"); err == nil {
		t.Error("a repo without commits")
	}
	if err := g.Prepare(ctx, t.TempDir(), filepath.Join(t.TempDir(), "x"), "wt/mara/main", "main"); err == nil {
		t.Error("a folder that isn't a repo")
	}
	other := codeRepo(t, "main")
	if err := g.Prepare(ctx, r, other, "wt/tove/main", "main"); err == nil || !strings.Contains(err.Error(), "another repo") {
		t.Errorf("a checkout of another repo in the way: %v", err)
	}
}

// A team of twelve starts at once on one repo: every worktree is made.
func TestConcurrentPreparesOnOneRepo(t *testing.T) {
	r := codeRepo(t, "main")
	g := New()
	base := t.TempDir()
	var wg sync.WaitGroup
	errs := make(chan error, 12)
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			agent := fmt.Sprintf("agent%d", i)
			errs <- g.Prepare(ctx, r, filepath.Join(base, agent), "wt/"+agent+"/main", "main")
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Error(err)
		}
	}
}

func TestExcludeKeepsTheSandboxesOutOfAnEnclosingRepo(t *testing.T) {
	outer := repo(t)
	project := filepath.Join(outer, "ledger")
	write(t, filepath.Join(project, ".sandboxes", "mara", "f"), "x\n")
	g := New()
	for i := 0; i < 2; i++ {
		if err := g.Exclude(ctx, filepath.Join(project, ".sandboxes")); err != nil {
			t.Fatal(err)
		}
	}
	data, _ := os.ReadFile(filepath.Join(outer, ".git", "info", "exclude"))
	if strings.Count(string(data), "/ledger/.sandboxes/\n") != 1 {
		t.Errorf("exclude file:\n%s", data)
	}
	if status, _ := run(ctx, outer, "status", "--porcelain", "--untracked-files=all"); strings.Contains(status, ".sandboxes") {
		t.Errorf("the sandboxes still show: %q", status)
	}
	// Outside any repo there is nothing to do.
	if err := g.Exclude(ctx, filepath.Join(t.TempDir(), ".sandboxes")); err != nil {
		t.Errorf("outside a repo: %v", err)
	}
}
