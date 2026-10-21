package git

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// What Foreman throws away, it keeps first: a dropped branch's head, the
// changes an agent discards. Each copy is a ref under refs/foreman/<kind>/,
// named with the time it was made, so it can be restored by hand and goes
// once it is old (wiki/product/architecture/sandboxes-and-git.md#backups).

// ErrMerging means the worktree is in an unfinished merge.
var ErrMerging = errors.New("git: a merge is under way")

// backupTime is how a backup's name carries its time: a ref name can't hold
// a colon.
const backupTime = "2006-01-02T15-04-05Z"

// BackupRef names a backup of kind (closed, discarded) made at at, under
// path (rev-3/lead/atlas/x): refs/foreman/closed/2026-10-22T09-30-00Z/rev-3/lead/atlas/x.
func BackupRef(kind string, at time.Time, path string) string {
	return "refs/foreman/" + kind + "/" + at.UTC().Format(backupTime) + "/" + path
}

// PruneBackups deletes the repo's backups made before before; a ref under
// refs/foreman/ whose name carries no time is left alone.
func (g *CLI) PruneBackups(ctx context.Context, dir string, before time.Time) error {
	defer g.lock(ctx, dir)()
	out, err := run(ctx, dir, "for-each-ref", "--format=%(refname)", "refs/foreman/")
	if err != nil {
		return err
	}
	for _, ref := range strings.Fields(out) {
		parts := strings.SplitN(ref, "/", 5) // refs, foreman, kind, time, path
		if len(parts) < 5 {
			continue
		}
		if at, err := time.Parse(backupTime, parts[3]); err == nil && at.Before(before) {
			if _, err := run(ctx, dir, "update-ref", "-d", ref); err != nil {
				return err
			}
		}
	}
	return nil
}

// Discard puts worktree back to its branch as committed: changes reverted,
// files not committed yet removed (ignored ones stay). What it threw away is
// kept first, as a commit on the branch's head under backup; saved is false
// when there was nothing to throw away. ErrMerging during an unfinished merge.
func (g *CLI) Discard(ctx context.Context, worktree, backup string) (saved bool, err error) {
	if !strings.HasPrefix(backup, "refs/foreman/") {
		return false, ErrBadBranch
	}
	defer g.lock(ctx, worktree)()
	if merging, err := g.Merging(ctx, worktree); err != nil {
		return false, err
	} else if merging {
		return false, ErrMerging
	}
	if saved, err = saveChanges(ctx, worktree, backup); err != nil || !saved {
		return false, err
	}
	return true, clean(ctx, worktree, "HEAD")
}

// saveChanges keeps the worktree's changes, files not committed yet included
// (ignored ones not), as one commit on its head under ref, through an index
// of its own, so the worktree's (a merge's conflicts, say) is left as it is.
// False when there are none.
func saveChanges(ctx context.Context, worktree, ref string) (bool, error) {
	if out, err := run(ctx, worktree, "status", "--porcelain"); err != nil || strings.TrimSpace(out) == "" {
		return false, err
	}
	tmp, err := os.MkdirTemp("", "foreman-index-")
	if err != nil {
		return false, err
	}
	defer func() { _ = os.RemoveAll(tmp) }()
	index := filepath.Join(tmp, "index")
	withIndex := func(args ...string) (string, error) {
		cmd := command(ctx, worktree, args...)
		cmd.Env = append(cmd.Env, "GIT_INDEX_FILE="+index)
		return output(cmd, args)
	}
	if _, err := withIndex("read-tree", "HEAD"); err != nil {
		return false, err
	}
	if _, err := withIndex("add", "--all"); err != nil {
		return false, err
	}
	tree, err := withIndex("write-tree")
	if err != nil {
		return false, err
	}
	var who []string
	if !hasIdentity(ctx, worktree) {
		who = fallbackIdentity
	}
	commit, err := run(ctx, worktree, append(who, "commit-tree", strings.TrimSpace(tree), "-p", "HEAD", "-m", "Changes not committed in "+worktree)...)
	if err != nil {
		return false, err
	}
	// Never over another backup: "" as the old value means none there yet.
	_, err = run(ctx, worktree, "update-ref", ref, strings.TrimSpace(commit), "")
	return err == nil, err
}

// clean puts the worktree's branch and files at commit, an unfinished merge
// abandoned and files not committed removed (ignored ones stay).
func clean(ctx context.Context, worktree, commit string) error {
	if _, err := run(ctx, worktree, "reset", "--hard", "--quiet", commit); err != nil {
		return err
	}
	_, err := run(ctx, worktree, "clean", "-d", "--force", "--quiet")
	return err
}
