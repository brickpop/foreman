// Package git runs Foreman's own git commands on the host
// (wiki/product/architecture/building-blocks.md#8-git-git). Only Foreman
// writes git: agents read it.
package git

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

// fallbackIdentity signs commits when the CEO's git has no user configured,
// so a save never fails for that alone.
var fallbackIdentity = []string{"-c", "user.name=Foreman", "-c", "user.email=foreman@localhost"}

// CLI runs the git command line, one command at a time per repo, so git's own
// lock files never collide (twelve agents starting at once share a repo).
type CLI struct {
	mu    sync.Mutex
	repos map[string]*sync.Mutex
}

// New returns a git runner.
func New() *CLI { return &CLI{repos: map[string]*sync.Mutex{}} }

// lock serializes the commands on one repo, named by its common git dir.
func (g *CLI) lock(ctx context.Context, dir string) func() {
	key := dir
	if common, err := run(ctx, dir, "rev-parse", "--path-format=absolute", "--git-common-dir"); err == nil {
		key = strings.TrimSpace(common)
	}
	g.mu.Lock()
	m, ok := g.repos[key]
	if !ok {
		m = &sync.Mutex{}
		g.repos[key] = m
	}
	g.mu.Unlock()
	m.Lock()
	return m.Unlock
}

// run runs git in dir. Hooks and the fsmonitor are always off: a repo's hooks
// and config may have been written by an agent, and must never run on the host.
// Foreman's commits are unsigned: they are automatic, and a signing agent would
// prompt (or fail) on every save.
func run(ctx context.Context, dir string, args ...string) (string, error) {
	base := []string{"-c", "core.hooksPath=/dev/null", "-c", "core.fsmonitor=false", "-c", "commit.gpgsign=false", "-C", dir}
	cmd := exec.CommandContext(ctx, "git", append(base, args...)...)
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %w: %s", args[0], err, strings.TrimSpace(errOut.String()))
	}
	return out.String(), nil
}

// Init makes dir a git repo whose first branch is branch, if it isn't a repo
// already.
func (g *CLI) Init(ctx context.Context, dir, branch string) error {
	_, err := run(ctx, dir, "init", "--quiet", "--initial-branch="+branch)
	return err
}

// ErrNothingToCommit means the files are as committed.
var ErrNothingToCommit = errors.New("git: nothing to commit")

// Commit commits exactly the given files (relative to dir), as they are on
// disk, leaving anything else staged or changed alone: by the CEO's git
// identity, or Foreman's when there is none.
func (g *CLI) Commit(ctx context.Context, dir, message string, files ...string) error {
	return g.CommitAs(ctx, dir, message, Author{}, files...)
}

// CommitAs is Commit by the author (the zero Author: as Commit).
func (g *CLI) CommitAs(ctx context.Context, dir, message string, by Author, files ...string) error {
	defer g.lock(ctx, dir)()
	if _, err := run(ctx, dir, append([]string{"add", "--"}, files...)...); err != nil {
		return err
	}
	if out, err := run(ctx, dir, append([]string{"status", "--porcelain", "--"}, files...)...); err != nil {
		return err
	} else if strings.TrimSpace(out) == "" {
		return ErrNothingToCommit
	}
	args := append([]string{"commit", "--quiet", "--no-verify", "-m", message, "--"}, files...)
	switch {
	case by != Author{}:
		args = append(by.config(), args...)
	case !hasIdentity(ctx, dir):
		args = append(fallbackIdentity, args...)
	}
	_, err := run(ctx, dir, args...)
	return err
}

func hasIdentity(ctx context.Context, dir string) bool {
	name, err1 := run(ctx, dir, "config", "user.name")
	email, err2 := run(ctx, dir, "config", "user.email")
	return err1 == nil && err2 == nil && strings.TrimSpace(name) != "" && strings.TrimSpace(email) != ""
}

// Prepare makes worktree a git worktree of repo on branch, creating the branch
// from base when it doesn't exist. A worktree already there is reused as it
// is, on whatever branch its agent switched to
// (wiki/product/architecture/sandboxes-and-git.md).
func (g *CLI) Prepare(ctx context.Context, repo, worktree, branch, base string) error {
	defer g.lock(ctx, repo)()
	common, err := run(ctx, repo, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return fmt.Errorf("%s is not a git repo: %w", repo, err)
	}
	if existing, err := run(ctx, worktree, "rev-parse", "--path-format=absolute", "--git-common-dir"); err == nil {
		if strings.TrimSpace(existing) != strings.TrimSpace(common) {
			return fmt.Errorf("%s is a checkout of another repo", worktree)
		}
		return nil
	}
	// Forget worktrees whose folder was deleted, so their branches are free.
	if _, err := run(ctx, repo, "worktree", "prune"); err != nil {
		return err
	}
	args := []string{"worktree", "add", "--quiet", worktree, branch}
	if _, err := run(ctx, repo, "rev-parse", "--verify", "--quiet", "refs/heads/"+branch); err != nil {
		if _, err := run(ctx, repo, "rev-parse", "--verify", "--quiet", "refs/heads/"+base); err != nil {
			return fmt.Errorf("%s has no branch %q to start %s from (no commits yet, or a different main branch: set `main` in project.toml)", repo, base, branch)
		}
		args = []string{"worktree", "add", "--quiet", "-b", branch, worktree, base}
	}
	_, err = run(ctx, repo, args...)
	return err
}

// Exclude keeps path (a folder) out of the repo that contains it, if any,
// through that repo's .git/info/exclude: nothing committed by accident, and
// no change to the CEO's own .gitignore.
func (g *CLI) Exclude(ctx context.Context, path string) error {
	dir := filepath.Dir(path)
	top, err := run(ctx, dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return nil // not inside a repo: nothing to exclude from
	}
	defer g.lock(ctx, dir)()
	rel, err := filepath.Rel(strings.TrimSpace(top), path)
	if err != nil {
		return err
	}
	file, err := run(ctx, dir, "rev-parse", "--path-format=absolute", "--git-path", "info/exclude")
	if err != nil {
		return err
	}
	return appendLine(strings.TrimSpace(file), "/"+filepath.ToSlash(rel)+"/")
}
