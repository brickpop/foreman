package git

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
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
	if err := (CLI{}).Init(ctx, dir); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestCommitTakesOnlyTheNamedFiles(t *testing.T) {
	dir := repo(t)
	write(t, filepath.Join(dir, "teams", "atlas.toml"), "team = 'atlas'\n")
	write(t, filepath.Join(dir, "scratch.txt"), "not mine\n")
	if err := (CLI{}).Commit(ctx, dir, "Atlas: saved in the UI", "teams/atlas.toml"); err != nil {
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
	if err := (CLI{}).Commit(ctx, dir, "first", "settings.toml"); err != nil {
		t.Fatal(err)
	}
	if err := (CLI{}).Commit(ctx, dir, "again", "settings.toml"); !errors.Is(err, ErrNothingToCommit) {
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
	if err := (CLI{}).Commit(ctx, dir, "m", "f"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(marker); err == nil {
		t.Error("a hook ran")
	}
}

func TestInitIsIdempotent(t *testing.T) {
	dir := repo(t)
	write(t, filepath.Join(dir, "f"), "x\n")
	if err := (CLI{}).Commit(ctx, dir, "m", "f"); err != nil {
		t.Fatal(err)
	}
	if err := (CLI{}).Init(ctx, dir); err != nil {
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
	if err := (CLI{}).Commit(ctx, dir, "m", "f"); err != nil {
		t.Errorf("commit with signing configured: %v", err)
	}
}
