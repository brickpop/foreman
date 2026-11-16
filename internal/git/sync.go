package git

import (
	"context"
	"fmt"
	"strings"
)

// Syncing an HQ with its origin, with the CEO's own git: their credentials,
// identity and signing (wiki/backlog/portable-projects.md). Only origin and
// one branch: an HQ is on main.

// Pull brings origin's branch into dir's checkout of it: fetched, then merged
// (a fast-forward when dir has nothing of its own). Nothing to do when origin
// hasn't the branch yet, or dir has it all already. It never loses or mixes
// in uncommitted changes: with some, only a fast-forward that leaves them
// alone is made, else ErrUncommitted; a merge that conflicts is aborted,
// leaving dir as it was (ErrConflict, naming the files). ErrNoRemote when dir
// has no origin.
func (g *CLI) Pull(ctx context.Context, dir, branch string) error {
	if g.Origin(ctx, dir) == "" {
		return ErrNoRemote
	}
	defer g.lock(ctx, dir)()
	if _, err := remote(ctx, dir, "fetch", "--prune", "--no-tags", "--no-recurse-submodules", "--quiet", "origin"); err != nil {
		return err
	}
	upstream := "refs/remotes/origin/" + branch
	if _, err := run(ctx, dir, "rev-parse", "--verify", "--quiet", upstream); err != nil {
		return nil // nothing pushed there yet: the push makes it
	}
	if on, err := run(ctx, dir, "symbolic-ref", "--quiet", "--short", "HEAD"); err != nil || strings.TrimSpace(on) != branch {
		return fmt.Errorf("git: the checkout isn't on %s: switch back to it to bring in origin's", branch)
	}
	if ancestor(ctx, dir, upstream, "HEAD") {
		return nil
	}
	clean, err := g.Clean(ctx, dir)
	if err != nil {
		return err
	}
	if !clean {
		if !ancestor(ctx, dir, "HEAD", upstream) {
			return fmt.Errorf("%w: origin's %s moved on while this one has commits of its own: commit or put aside the changes, then it merges", ErrUncommitted, branch)
		}
		// git refuses, touching nothing, if it would overwrite one of them.
		_, err := run(ctx, dir, "merge", "--ff-only", "--quiet", upstream)
		return err
	}
	args := []string{"merge", "--no-edit", "--quiet", upstream}
	if !hasIdentity(ctx, dir) {
		args = append(fallbackIdentity, args...)
	}
	if _, mergeErr := run(ctx, dir, args...); mergeErr != nil {
		files, _ := g.unmerged(ctx, dir)
		if merging, _ := g.Merging(ctx, dir); merging {
			if _, err := run(ctx, dir, "merge", "--abort"); err != nil {
				return fmt.Errorf("%w (%s), and aborting it failed: %v", ErrConflict, strings.Join(files, ", "), err)
			}
			return fmt.Errorf("%w: origin's %s and this one changed %s: merge them by hand", ErrConflict, branch, strings.Join(files, ", "))
		}
		return mergeErr
	}
	return nil
}

// Push pushes dir's branch to origin's, never forcing: refused when origin
// has commits dir hasn't (Pull first). ErrNoRemote when dir has no origin.
func (g *CLI) Push(ctx context.Context, dir, branch string) error {
	if g.Origin(ctx, dir) == "" {
		return ErrNoRemote
	}
	defer g.lock(ctx, dir)()
	ref := "refs/heads/" + branch
	_, err := remote(ctx, dir, "push", "--quiet", "--no-verify", "origin", ref+":"+ref)
	return err
}

// ancestor says whether commit a is in b's history.
func ancestor(ctx context.Context, dir, a, b string) bool {
	_, err := run(ctx, dir, "merge-base", "--is-ancestor", a, b)
	return err == nil
}
