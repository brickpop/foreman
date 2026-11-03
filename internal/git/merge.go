package git

import (
	"context"
	"errors"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// The CEO's merge into a target (wiki/product/features/pipeline-and-merges.md#merge-to-a-target).

var (
	// ErrConflict means the branch doesn't merge cleanly into the target:
	// nothing was changed.
	ErrConflict = errors.New("git: conflicts with the target branch")
	// ErrLocalChanges means the repo's checkout has the target checked out
	// with uncommitted changes the merge would overwrite: nothing was changed.
	ErrLocalChanges = errors.New("git: uncommitted changes in the way")
	// ErrMoved means the branch isn't at the commit it was expected at
	// anymore: nothing was changed.
	ErrMoved = errors.New("git: the branch moved")
)

// Squash puts what branch changes onto target as one new commit, by the
// CEO's git identity (Foreman's when there is none), with the message alone:
// no trailers (the CEO, 2026-10-29). Only while branch is at head (the commit the CEO read, its full
// hash): ErrMoved otherwise. When target has all of it already, nothing is committed. When
// dir (the repo's own checkout, the CEO's) has target checked out, its files
// move with target; otherwise they are left alone.
func (g *CLI) Squash(ctx context.Context, dir, target, branch, head, message string) error {
	if !ValidBranch(ctx, branch) || !ValidBranch(ctx, target) {
		return ErrBadBranch
	}
	if !hash.MatchString(head) {
		return ErrNoRef
	}
	defer g.lock(ctx, dir)()
	at, err := g.Resolve(ctx, dir, "refs/heads/"+branch)
	if err != nil {
		return err
	} else if at != head {
		return ErrMoved
	}
	tree, err := mergedTree(ctx, dir, "refs/heads/"+target, at)
	if err != nil {
		return err
	}
	old, err := g.Resolve(ctx, dir, "refs/heads/"+target)
	if err != nil {
		return err
	}
	if was, err := treeOf(ctx, dir, old); err != nil || was == tree {
		return err
	}
	var who []string
	if !hasIdentity(ctx, dir) {
		who = fallbackIdentity
	}
	commit, err := run(ctx, dir, append(who, "commit-tree", tree, "-p", old, "-m", message)...)
	if err != nil {
		return err
	}
	commit = strings.TrimSpace(commit)
	if current, _ := run(ctx, dir, "branch", "--show-current"); strings.TrimSpace(current) == target {
		// A fast-forward to the new commit: the checkout's files follow, and
		// git refuses when the CEO's own changes are in the way.
		if _, err := run(ctx, dir, "merge", "--ff-only", "--quiet", commit); err != nil {
			if strings.Contains(err.Error(), "would be overwritten") {
				return ErrLocalChanges
			}
			return err
		}
		return nil
	}
	// Moved only if target is still where it was read: never over someone else's move.
	_, err = run(ctx, dir, "update-ref", "refs/heads/"+target, commit, old)
	return err
}

// Merged reports whether merging branch into into would change nothing: all
// of branch's changes are there, however they got there (a squash, a merge
// commit, a rebase on a forge).
func (g *CLI) Merged(ctx context.Context, dir, into, branch string) (bool, error) {
	if !ValidBranch(ctx, branch) || !ValidBranch(ctx, into) {
		return false, ErrBadBranch
	}
	tree, err := mergedTree(ctx, dir, "refs/heads/"+into, "refs/heads/"+branch)
	if errors.Is(err, ErrConflict) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	was, err := treeOf(ctx, dir, "refs/heads/"+into)
	return was == tree, err
}

// mergedTree is the tree merging commit into target (both refs or hashes)
// would make, without touching any checkout; ErrConflict when it doesn't
// merge cleanly.
func mergedTree(ctx context.Context, dir, target, commit string) (string, error) {
	out, err := run(ctx, dir, "merge-tree", "--write-tree", target, commit)
	var exit *exec.ExitError
	switch {
	case errors.As(err, &exit) && exit.ExitCode() == 1: // merge-tree's "conflicts"
		return "", ErrConflict
	case err != nil:
		return "", err
	}
	return strings.Fields(out)[0], nil
}

func treeOf(ctx context.Context, dir, commit string) (string, error) {
	out, err := run(ctx, dir, "rev-parse", "--verify", "--quiet", commit+"^{tree}")
	return strings.TrimSpace(out), err
}

// MoveBranch moves branch from the commit from to where branch to is. Where
// a worktree has branch checked out, its files follow, keeping the
// uncommitted changes git can carry over (git reset --keep); it fails,
// changing nothing, when they are in the way or a merge is unfinished there.
// ErrMoved when branch isn't at from anymore.
func (g *CLI) MoveBranch(ctx context.Context, dir, branch, from, to string) error {
	if !ValidBranch(ctx, branch) || !ValidBranch(ctx, to) {
		return ErrBadBranch
	}
	defer g.lock(ctx, dir)()
	if head, err := g.Resolve(ctx, dir, "refs/heads/"+branch); err != nil {
		return err
	} else if head != from {
		return ErrMoved
	}
	target, err := g.Resolve(ctx, dir, "refs/heads/"+to)
	if err != nil {
		return err
	}
	trees, err := g.Worktrees(ctx, dir)
	if err != nil {
		return err
	}
	if tree, ok := trees[branch]; ok {
		_, err = run(ctx, tree, "reset", "--quiet", "--keep", target)
		return err
	}
	_, err = run(ctx, dir, "update-ref", "refs/heads/"+branch, target, from)
	return err
}

// DropBranch discards branch, its head first kept under backup (a BackupRef),
// so a mistake can be undone by hand. A branch checked out in a worktree
// can't go: it moves to where branch def is instead, and the worktree is
// made clean, an unfinished merge abandoned; what it held not committed is
// kept first under backup + "@uncommitted". Only for a worktree nobody works
// in.
func (g *CLI) DropBranch(ctx context.Context, dir, branch, backup, def string) error {
	if !ValidBranch(ctx, branch) || !ValidBranch(ctx, def) || !strings.HasPrefix(backup, "refs/foreman/") {
		return ErrBadBranch
	}
	defer g.lock(ctx, dir)()
	head, err := g.Resolve(ctx, dir, "refs/heads/"+branch)
	if err != nil {
		return err
	}
	trees, err := g.Worktrees(ctx, dir)
	if err != nil {
		return err
	}
	if _, err := run(ctx, dir, "update-ref", backup, head); err != nil {
		return err
	}
	if tree, ok := trees[branch]; ok {
		target, err := g.Resolve(ctx, dir, "refs/heads/"+def)
		if err == nil {
			_, err = saveChanges(ctx, tree, backup+"@uncommitted")
		}
		if err == nil {
			err = clean(ctx, tree, target)
		}
		return err
	}
	// Deleted only if still where it was read.
	_, err = run(ctx, dir, "update-ref", "-d", "refs/heads/"+branch, head)
	return err
}

// Diff is what the commit head changes against branch target since they
// parted, as a unified diff. A moved file always reads as one rename, not a
// deletion and an addition, whatever the CEO's git config says.
func (g *CLI) Diff(ctx context.Context, dir, target, head string) (string, error) {
	if !ValidBranch(ctx, target) {
		return "", ErrBadBranch
	}
	if !hash.MatchString(head) {
		return "", ErrNoRef
	}
	return run(ctx, dir, "diff", "--find-renames", "refs/heads/"+target+"..."+head)
}

// hash is a commit's id as git prints it, in full or abbreviated.
var hash = regexp.MustCompile(`^[0-9a-f]{4,64}$`)

// Compare is what changed from one commit to another (both hashes), as a
// unified diff with that many lines of context around each change; only in paths when any are
// named. Unlike Diff it compares the two commits themselves, not since they
// parted. Renames read as Diff's do.
func (g *CLI) Compare(ctx context.Context, dir, from, to string, lines int, paths ...string) (string, error) {
	if !hash.MatchString(from) || !hash.MatchString(to) || lines < 0 {
		return "", ErrNoRef
	}
	args := append([]string{"diff", "--find-renames", "--unified=" + strconv.Itoa(lines), from, to, "--"}, paths...)
	return run(ctx, dir, args...)
}
