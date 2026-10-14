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
	"strings"
)

// fallbackIdentity signs commits when the CEO's git has no user configured,
// so a save never fails for that alone.
var fallbackIdentity = []string{"-c", "user.name=Foreman", "-c", "user.email=foreman@localhost"}

// CLI runs the git command line.
type CLI struct{}

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

// Init makes dir a git repo, if it isn't one already.
func (CLI) Init(ctx context.Context, dir string) error {
	_, err := run(ctx, dir, "init", "--quiet", "--initial-branch=main")
	return err
}

// ErrNothingToCommit means the files are as committed.
var ErrNothingToCommit = errors.New("git: nothing to commit")

// Commit commits exactly the given files (relative to dir), as they are on
// disk, leaving anything else staged or changed alone.
func (CLI) Commit(ctx context.Context, dir, message string, files ...string) error {
	if _, err := run(ctx, dir, append([]string{"add", "--"}, files...)...); err != nil {
		return err
	}
	if out, err := run(ctx, dir, append([]string{"status", "--porcelain", "--"}, files...)...); err != nil {
		return err
	} else if strings.TrimSpace(out) == "" {
		return ErrNothingToCommit
	}
	args := append([]string{"commit", "--quiet", "--no-verify", "-m", message, "--"}, files...)
	if !hasIdentity(ctx, dir) {
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
