package git

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
)

// The git writes agents ask for through their team tools, run in their own
// worktrees (wiki/product/architecture/sandboxes-and-git.md#agents-and-git).

// Author is who a commit is by: an agent, never the CEO's identity.
type Author struct{ Name, Email string }

func (a Author) config() []string {
	return []string{"-c", "user.name=" + a.Name, "-c", "user.email=" + a.Email}
}

var (
	// ErrUncommitted means the worktree has changes, or an unfinished merge.
	ErrUncommitted = errors.New("git: uncommitted changes")
	// ErrBadBranch means the name isn't a valid branch name.
	ErrBadBranch = errors.New("git: not a valid branch name")
	// ErrNoMerge means there is no unfinished merge to abort.
	ErrNoMerge = errors.New("git: no merge in progress")
)

// ConflictsError lists the files of an unfinished merge that still hold
// conflict markers.
type ConflictsError struct{ Files []string }

func (e *ConflictsError) Error() string {
	return "git: conflict markers left in " + strings.Join(e.Files, ", ")
}

// ValidBranch reports whether name is a valid branch name.
func ValidBranch(ctx context.Context, name string) bool {
	_, err := run(ctx, ".", "check-ref-format", "--branch", name)
	return err == nil && !strings.HasPrefix(name, "-")
}

// HasBranch reports whether the repo of dir has the branch.
func (g *CLI) HasBranch(ctx context.Context, dir, branch string) bool {
	_, err := run(ctx, dir, "rev-parse", "--verify", "--quiet", "refs/heads/"+branch)
	return err == nil
}

// Ahead is how many commits branch has that base hasn't.
func (g *CLI) Ahead(ctx context.Context, dir, branch, base string) (int, error) {
	out, err := run(ctx, dir, "rev-list", "--count", "refs/heads/"+base+"..refs/heads/"+branch)
	if err != nil {
		return 0, err
	}
	var n int
	_, err = fmt.Sscan(out, &n)
	return n, err
}

// Branches lists the repo's branches whose names start with prefix
// (lead/atlas/), sorted.
func (g *CLI) Branches(ctx context.Context, dir, prefix string) ([]string, error) {
	out, err := run(ctx, dir, "for-each-ref", "--format=%(refname:short)", "refs/heads/"+prefix)
	if err != nil {
		return nil, err
	}
	var list []string
	for _, b := range strings.Fields(out) {
		if strings.HasPrefix(b, prefix) {
			list = append(list, b)
		}
	}
	slices.Sort(list)
	return list, nil
}

// Contains reports whether branch into has branch's head in its history:
// branch was merged into it (or into is branch itself, or ahead of it).
func (g *CLI) Contains(ctx context.Context, dir, into, branch string) (bool, error) {
	_, err := run(ctx, dir, "merge-base", "--is-ancestor", "refs/heads/"+branch, "refs/heads/"+into)
	var exit *exec.ExitError
	switch {
	case err == nil:
		return true, nil
	case errors.As(err, &exit) && exit.ExitCode() == 1: // not an ancestor
		return false, nil
	}
	return false, err
}

// Clean reports whether the worktree has no changes and no unfinished merge.
func (g *CLI) Clean(ctx context.Context, dir string) (bool, error) {
	out, err := run(ctx, dir, "status", "--porcelain")
	if err != nil {
		return false, err
	}
	merging, err := g.Merging(ctx, dir)
	return strings.TrimSpace(out) == "" && !merging, err
}

// Changed reports whether file (relative to dir) differs from what is
// committed: edited, staged, or not committed yet at all.
func (g *CLI) Changed(ctx context.Context, dir, file string) (bool, error) {
	out, err := run(ctx, dir, "status", "--porcelain", "--", file)
	return strings.TrimSpace(out) != "", err
}

// Merging reports whether the worktree is in an unfinished merge.
func (g *CLI) Merging(ctx context.Context, dir string) (bool, error) {
	head, err := run(ctx, dir, "rev-parse", "--path-format=absolute", "--git-path", "MERGE_HEAD")
	if err != nil {
		return false, err
	}
	_, err = os.Stat(strings.TrimSpace(head))
	return err == nil, nil
}

// Switch checks the worktree out on branch, creating it from base when the
// repo doesn't have it yet; created says it did. A worktree with changes is
// refused (ErrUncommitted), so no work is carried from one branch to another.
func (g *CLI) Switch(ctx context.Context, dir, branch, base string) (created bool, err error) {
	if !ValidBranch(ctx, branch) {
		return false, ErrBadBranch
	}
	defer g.lock(ctx, dir)()
	if clean, err := g.Clean(ctx, dir); err != nil {
		return false, err
	} else if !clean {
		return false, ErrUncommitted
	}
	if g.HasBranch(ctx, dir, branch) {
		_, err := run(ctx, dir, "checkout", "--quiet", branch)
		return false, err
	}
	if !g.HasBranch(ctx, dir, base) {
		return false, fmt.Errorf("no branch %q to start %s from", base, branch)
	}
	_, err = run(ctx, dir, "checkout", "--quiet", "-b", branch, base)
	return err == nil, err
}

// CommitAll commits everything changed in the worktree on the branch checked
// out there, by the author, and returns the commit. After a merge with
// conflicts it concludes the merge, unless a conflicted file still holds
// markers (*ConflictsError).
func (g *CLI) CommitAll(ctx context.Context, dir, message string, by Author) (string, error) {
	defer g.lock(ctx, dir)()
	if files, err := g.markersLeft(ctx, dir); err != nil {
		return "", err
	} else if len(files) > 0 {
		return "", &ConflictsError{Files: files}
	}
	if _, err := run(ctx, dir, "add", "--all"); err != nil {
		return "", err
	}
	merging, err := g.Merging(ctx, dir)
	if err != nil {
		return "", err
	}
	if _, err := run(ctx, dir, "diff", "--cached", "--quiet"); err == nil && !merging {
		return "", ErrNothingToCommit
	}
	args := append(by.config(), "commit", "--quiet", "--no-verify", "-m", message)
	if _, err := run(ctx, dir, args...); err != nil {
		return "", err
	}
	head, err := run(ctx, dir, "rev-parse", "HEAD")
	return strings.TrimSpace(head), err
}

// markersLeft are the files of an unfinished merge whose conflict markers
// are still in them.
func (g *CLI) markersLeft(ctx context.Context, dir string) ([]string, error) {
	unmerged, err := g.unmerged(ctx, dir)
	if err != nil {
		return nil, err
	}
	var left []string
	for _, f := range unmerged {
		data, err := os.ReadFile(filepath.Join(dir, f))
		if err != nil {
			continue // deleted: resolved by removing it
		}
		if hasMarkers(string(data)) {
			left = append(left, f)
		}
	}
	return left, nil
}

// unmerged are the files a merge left conflicted.
func (g *CLI) unmerged(ctx context.Context, dir string) ([]string, error) {
	out, err := run(ctx, dir, "diff", "--name-only", "-z", "--diff-filter=U")
	if err != nil {
		return nil, err
	}
	return strings.FieldsFunc(out, func(r rune) bool { return r == 0 }), nil
}

func hasMarkers(text string) bool {
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "<<<<<<< ") || strings.HasPrefix(line, ">>>>>>> ") {
			return true
		}
	}
	return false
}

// Merge merges branch into the branch checked out in the worktree, by the
// author. On a conflict the markers stay in the files, which it returns; the
// agent resolves them and commits. A worktree with changes is refused
// (ErrUncommitted), so a merge never mixes with unsaved work.
func (g *CLI) Merge(ctx context.Context, dir, branch string, by Author) (conflicts []string, err error) {
	if !ValidBranch(ctx, branch) {
		return nil, ErrBadBranch
	}
	defer g.lock(ctx, dir)()
	if clean, err := g.Clean(ctx, dir); err != nil {
		return nil, err
	} else if !clean {
		return nil, ErrUncommitted
	}
	args := append(by.config(), "merge", "--quiet", "--no-edit", branch)
	if _, mergeErr := run(ctx, dir, args...); mergeErr != nil {
		files, err := g.unmerged(ctx, dir)
		if err != nil || len(files) == 0 {
			return nil, mergeErr
		}
		return files, nil
	}
	return nil, nil
}

// Branch is the branch checked out in the worktree.
func (g *CLI) Branch(ctx context.Context, dir string) (string, error) {
	out, err := run(ctx, dir, "branch", "--show-current")
	return strings.TrimSpace(out), err
}

// AbortMerge backs out of an unfinished merge: the branch is as it was before
// it, and so are the files.
func (g *CLI) AbortMerge(ctx context.Context, dir string) error {
	defer g.lock(ctx, dir)()
	merging, err := g.Merging(ctx, dir)
	if err != nil {
		return err
	}
	if !merging {
		return ErrNoMerge
	}
	_, err = run(ctx, dir, "merge", "--abort")
	return err
}
