package git

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// The CEO's Merge to main (wiki/product/features/pipeline-and-merges.md#merge-to-main).

var (
	// ErrConflict means the branch doesn't merge cleanly into the target:
	// nothing was changed.
	ErrConflict = errors.New("git: conflicts with the target branch")
	// ErrLocalChanges means the repo's checkout has the target checked out
	// with uncommitted changes the merge would overwrite: nothing was changed.
	ErrLocalChanges = errors.New("git: uncommitted changes in the way")
)

// MergeInto merges branch into target with a merge commit (always, never a
// fast-forward) by the CEO's git identity, or Foreman's when there is none.
// When dir (the repo's own checkout, the CEO's) has target checked out, the
// merge runs there so its files move with target; otherwise it is made
// without a working tree, leaving the checkout's files alone.
func (g *CLI) MergeInto(ctx context.Context, dir, target, branch, message string) error {
	if !ValidBranch(ctx, branch) || !ValidBranch(ctx, target) {
		return ErrBadBranch
	}
	defer g.lock(ctx, dir)()
	var who []string
	if !hasIdentity(ctx, dir) {
		who = fallbackIdentity
	}
	if current, _ := run(ctx, dir, "branch", "--show-current"); strings.TrimSpace(current) == target {
		return mergeCheckedOut(ctx, dir, branch, message, who)
	}
	return mergeWithoutWorktree(ctx, dir, target, branch, message, who)
}

func mergeCheckedOut(ctx context.Context, dir, branch, message string, who []string) error {
	_, err := run(ctx, dir, append(who, "merge", "--no-ff", "--no-edit", "-m", message, branch)...)
	if err == nil {
		return nil
	}
	// A merge left half-way is a conflict: back out, so nothing changed.
	if gitDir, gerr := run(ctx, dir, "rev-parse", "--absolute-git-dir"); gerr == nil {
		if _, serr := os.Stat(filepath.Join(strings.TrimSpace(gitDir), "MERGE_HEAD")); serr == nil {
			_, _ = run(ctx, dir, "merge", "--abort")
			return ErrConflict
		}
	}
	if strings.Contains(err.Error(), "would be overwritten") || strings.Contains(err.Error(), "local changes") {
		return ErrLocalChanges
	}
	return err
}

func mergeWithoutWorktree(ctx context.Context, dir, target, branch, message string, who []string) error {
	out, err := run(ctx, dir, "merge-tree", "--write-tree", "refs/heads/"+target, "refs/heads/"+branch)
	var exit *exec.ExitError
	switch {
	case errors.As(err, &exit) && exit.ExitCode() == 1: // merge-tree's "conflicts"
		return ErrConflict
	case err != nil:
		return err
	}
	tree := strings.Fields(out)[0]
	old, err := run(ctx, dir, "rev-parse", "refs/heads/"+target)
	if err != nil {
		return err
	}
	head, err := run(ctx, dir, "rev-parse", "refs/heads/"+branch)
	if err != nil {
		return err
	}
	commit, err := run(ctx, dir, append(who, "commit-tree", tree, "-p", strings.TrimSpace(old), "-p", strings.TrimSpace(head), "-m", message)...)
	if err != nil {
		return err
	}
	// Moved only if target is still where it was read: never over someone else's move.
	_, err = run(ctx, dir, "update-ref", "refs/heads/"+target, strings.TrimSpace(commit), strings.TrimSpace(old))
	return err
}

// Diff is what branch changes against target since they parted, as a
// unified diff.
func (g *CLI) Diff(ctx context.Context, dir, target, branch string) (string, error) {
	if !ValidBranch(ctx, branch) || !ValidBranch(ctx, target) {
		return "", ErrBadBranch
	}
	return run(ctx, dir, "diff", "refs/heads/"+target+"...refs/heads/"+branch)
}
