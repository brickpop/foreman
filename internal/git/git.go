// Package git runs Foreman's own git commands on the host
// (wiki/product/architecture/building-blocks.md#8-git-git). Only Foreman
// writes git: agents read it.
package git

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

// fallbackIdentity authors commits when the CEO's git has no user configured,
// so a save never fails for that alone. Not the CEO, so never signed.
var fallbackIdentity = []string{"-c", "user.name=Foreman", "-c", "user.email=foreman@localhost", "-c", "commit.gpgsign=false"}

// CLI runs the git command line, one command at a time per repo, so git's own
// lock files never collide (twelve agents starting at once share a repo).
type CLI struct {
	mu    sync.Mutex
	repos map[string]*sync.Mutex
	// FetchProtocols are what submodules are fetched over: https when empty.
	// Tests add file, for their local repos.
	FetchProtocols []string
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
	return output(command(ctx, dir, args...), args)
}

// output runs a git command made by command, for its standard output.
func output(cmd *exec.Cmd, args []string) (string, error) {
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %w: %s", verb(args), err, strings.TrimSpace(errOut.String()))
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
// identity, signed when the CEO's git config says so, or Foreman's when there
// is none.
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

// signs says whether the CEO's git config asks for signed commits.
func signs(ctx context.Context, dir string) bool {
	out, err := run(ctx, dir, "config", "--type=bool", "commit.gpgsign")
	return err == nil && strings.TrimSpace(out) == "true"
}

// Prepare makes worktree a git worktree of repo: on branch, creating it from
// base (a branch) when the repo doesn't have it, or detached at base with no
// branch ("": an idle worktree owns none). A worktree already there is reused
// as it is, on whatever its agent switched to
// (wiki/product/architecture/sandboxes-and-git.md#opening-a-repo), and
// relinked to repo if repo moved since (Locate). Either way, idle (detached),
// it follows base (Freshen).
func (g *CLI) Prepare(ctx context.Context, repo, worktree, branch, base string) error {
	if !ValidBranch(ctx, base) || (branch != "" && !ValidBranch(ctx, branch)) {
		return ErrBadBranch
	}
	defer g.lock(ctx, repo)()
	// An idle desk not brought up to date is no reason to fail a start.
	defer func() { _, _ = g.freshen(ctx, worktree, base) }()
	common, err := run(ctx, repo, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return fmt.Errorf("%s is not a git repo: %w", repo, err)
	}
	existing, err := run(ctx, worktree, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if _, statErr := os.Stat(filepath.Join(worktree, ".git")); err != nil && statErr == nil {
		// A worktree whose link to the repo's old folder is broken: repair names
		// the new one.
		_, _ = run(ctx, repo, "worktree", "repair", worktree)
		existing, err = run(ctx, worktree, "rev-parse", "--path-format=absolute", "--git-common-dir")
	}
	if err == nil {
		if strings.TrimSpace(existing) != strings.TrimSpace(common) {
			return fmt.Errorf("%s is a checkout of another repo", worktree)
		}
		// Its submodules' links break the same way when it moved.
		return relinkSubmodules(ctx, worktree)
	}
	// Forget worktrees whose folder was deleted, so their branches are free.
	if _, err := run(ctx, repo, "worktree", "prune"); err != nil {
		return err
	}
	hasBase := g.HasBranch(ctx, repo, base)
	var args []string
	switch {
	case branch != "" && g.HasBranch(ctx, repo, branch):
		args = []string{"worktree", "add", "--quiet", worktree, branch}
	case !hasBase:
		return fmt.Errorf("%s has no branch %q to start from (deleted, or its default branch is another: pick it in the project's Repos)", repo, base)
	case branch == "":
		args = []string{"worktree", "add", "--quiet", "--detach", worktree, "refs/heads/" + base}
	default:
		args = []string{"worktree", "add", "--quiet", "-b", branch, worktree, "refs/heads/" + base}
	}
	_, err = run(ctx, repo, args...)
	return err
}

// Clone clones source (a URL or a local path) into dir, a folder that doesn't
// exist yet, as the CEO: their SSH keys, agent and credential helper. It
// never prompts: with no terminal (its own session) and GIT_TERMINAL_PROMPT=0,
// a missing credential fails with git's message. Only the usual transports
// are allowed, so a URL can't run a command (ext::).
func (g *CLI) Clone(ctx context.Context, source, dir string) error {
	if strings.HasPrefix(source, "-") {
		return fmt.Errorf("%q is not a git URL or path", source)
	}
	cmd := command(ctx, filepath.Dir(dir), "-c", "protocol.allow=never", "-c", "protocol.https.allow=always", "-c", "protocol.http.allow=always",
		"-c", "protocol.ssh.allow=always", "-c", "protocol.git.allow=always", "-c", "protocol.file.allow=always",
		"clone", "--quiet", "--", source, dir)
	cmd.Env = append(cmd.Env, "GIT_TERMINAL_PROMPT=0")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	var errOut bytes.Buffer
	cmd.Stderr = &errOut
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git clone: %w: %s", err, strings.TrimSpace(errOut.String()))
	}
	return nil
}

// MoveWorktree moves one of repo's worktrees from one folder to another
// (a repo renamed in its project).
func (g *CLI) MoveWorktree(ctx context.Context, repo, from, to string) error {
	defer g.lock(ctx, repo)()
	_, err := run(ctx, repo, "worktree", "move", from, to)
	return err
}

// RemoveWorktree removes one of repo's worktrees, its folder included; its
// branch stays. Git refuses one with changes not committed (tracked or not),
// so nothing is lost: ErrUncommitted.
func (g *CLI) RemoveWorktree(ctx context.Context, repo, worktree string) error {
	defer g.lock(ctx, repo)()
	_, err := run(ctx, repo, "worktree", "remove", "--", worktree)
	if err != nil && strings.Contains(err.Error(), "contains modified or untracked files") {
		return ErrUncommitted
	}
	return err
}

// PruneWorktrees makes repo forget its worktrees whose folder is gone (a
// project's sandboxes removed); their branches stay.
func (g *CLI) PruneWorktrees(ctx context.Context, repo string) error {
	defer g.lock(ctx, repo)()
	_, err := run(ctx, repo, "worktree", "prune")
	return err
}

// RepairWorktrees tells repo where its worktrees are now, after they moved:
// each one's link to the repo still holds, the repo's record of it doesn't.
func (g *CLI) RepairWorktrees(ctx context.Context, repo string, worktrees ...string) error {
	defer g.lock(ctx, repo)()
	_, err := run(ctx, repo, append([]string{"worktree", "repair", "--"}, worktrees...)...)
	return err
}

// command is git with Foreman's settings, in dir. Its messages are always in
// English (LC_ALL=C): Foreman reads some of them, and a CEO's git in another
// language would otherwise hide what happened (found 2026-10-17, a Catalan
// git). Only git's own messages change: file names and commits don't.
//
// No optional locks (GIT_OPTIONAL_LOCKS=0): a read such as `status` then never
// takes index.lock to refresh the index, so Foreman's reads of an agent's
// worktree never get in the way of the agent's own git. And a cancelled
// command gets SIGTERM, on which git removes its lock files, before a kill:
// a SIGKILL left a stale index.lock behind (found 2026-10-18).
//
// git's housekeeping after a commit (maintenance, gc) runs inside the command,
// never detached: a detached one escapes the one-command-per-repo lock and
// can hold the repo's locks while Foreman's next command needs them (found
// 2026-10-20, as orphaned processes under an init that doesn't reap them).
func command(ctx context.Context, dir string, args ...string) *exec.Cmd {
	base := []string{"-c", "core.hooksPath=/dev/null", "-c", "core.fsmonitor=false",
		"-c", "maintenance.autoDetach=false", "-c", "gc.autoDetach=false", "-C", dir}
	cmd := exec.CommandContext(ctx, "git", append(base, args...)...)
	cmd.Env = append(os.Environ(), "GIT_OPTIONAL_LOCKS=0", "LC_ALL=C")
	cmd.Cancel = func() error { return cmd.Process.Signal(syscall.SIGTERM) }
	cmd.WaitDelay = 5 * time.Second
	return cmd
}

// verb is the git command args run, past the -c settings before it.
func verb(args []string) string {
	for i := 0; i < len(args); i++ {
		if args[i] != "-c" {
			return args[i]
		}
		i++
	}
	return "git"
}
