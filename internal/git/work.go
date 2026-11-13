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

// config makes a commit the agent's: its name, and never the CEO's signature,
// whatever the CEO's git config says (the CEO, 2026-11-06).
func (a Author) config() []string {
	return []string{"-c", "user.name=" + a.Name, "-c", "user.email=" + a.Email, "-c", "commit.gpgsign=false"}
}

var (
	// ErrUncommitted means the worktree has changes, or an unfinished merge.
	ErrUncommitted = errors.New("git: uncommitted changes")
	// ErrBadBranch means the name isn't a valid branch name.
	ErrBadBranch = errors.New("git: not a valid branch name")
	// ErrNoMerge means there is no unfinished merge to abort.
	ErrNoMerge = errors.New("git: no merge in progress")
	// ErrNotDetached: the worktree is on a branch, not idle.
	ErrNotDetached = errors.New("git: the worktree is on a branch")
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

// BranchPrefix is what an agent's branches on a project start with:
// wt/<project>/<agent>/ for a member, lead/<project>/<team>/ for the lead
// (wiki/product/architecture/sandboxes-and-git.md#branches). The project is
// in it, so one team can work on two projects that share a repo (the CEO,
// 2026-11-03).
func BranchPrefix(project, agent, team string, lead bool) string {
	if lead {
		return "lead/" + project + "/" + team + "/"
	}
	return "wt/" + project + "/" + agent + "/"
}

// AgentBranch says whether branch is in an agents' namespace (wt/*,
// lead/*): agents write those, so a merge never goes into one, and none is a
// repo's default branch.
func AgentBranch(branch string) bool {
	return strings.HasPrefix(branch, "wt/") || strings.HasPrefix(branch, "lead/")
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
// (lead/ledger/atlas/), sorted.
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

// Holders lists the branches whose history holds all of branch: where its
// commits are. local are dir's branches but branch itself (feature/x);
// upstream a remote's as fetched (origin/main), but a remote's HEAD. Each
// sorted.
func (g *CLI) Holders(ctx context.Context, dir, branch string) (local, upstream []string, err error) {
	if !ValidBranch(ctx, branch) {
		return nil, nil, ErrBadBranch
	}
	out, err := run(ctx, dir, "for-each-ref", "--format=%(refname)", "--contains", "refs/heads/"+branch, "refs/heads/", "refs/remotes/")
	if err != nil {
		return nil, nil, err
	}
	for _, ref := range strings.Fields(out) {
		if name, ok := strings.CutPrefix(ref, "refs/heads/"); ok {
			if name != branch {
				local = append(local, name)
			}
		} else if name := strings.TrimPrefix(ref, "refs/remotes/"); !strings.HasSuffix(name, "/HEAD") {
			upstream = append(upstream, name)
		}
	}
	slices.Sort(local)
	slices.Sort(upstream)
	return local, upstream, nil
}

// Contains reports whether branch into has branch's head in its history:
// branch was merged into it (or into is branch itself, or ahead of it).
func (g *CLI) Contains(ctx context.Context, dir, into, branch string) (bool, error) {
	return g.InHistory(ctx, dir, into, "refs/heads/"+branch)
}

// InHistory reports whether commit (a hash or a ref) is in branch's history.
func (g *CLI) InHistory(ctx context.Context, dir, branch, commit string) (bool, error) {
	_, err := run(ctx, dir, "merge-base", "--is-ancestor", commit, "refs/heads/"+branch)
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
// Changes inside a submodule count, whatever .gitmodules says to ignore.
func (g *CLI) Clean(ctx context.Context, dir string) (bool, error) {
	out, err := run(ctx, dir, "status", "--porcelain", "--ignore-submodules=none")
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

// Freshen moves an idle worktree, detached on no branch, up to the head of
// branch def, when its commit is in def's history and it holds nothing not
// committed: an idle desk follows the default branch. A worktree on a branch
// stays as it is. moved says it did.
func (g *CLI) Freshen(ctx context.Context, worktree, def string) (moved bool, err error) {
	if !ValidBranch(ctx, def) {
		return false, ErrBadBranch
	}
	defer g.lock(ctx, worktree)()
	return g.freshen(ctx, worktree, def)
}

func (g *CLI) freshen(ctx context.Context, tree, def string) (bool, error) {
	if on, err := g.Branch(ctx, tree); err != nil || on != "" {
		return false, err
	}
	head, err := g.Resolve(ctx, tree, "HEAD")
	if err != nil {
		return false, err
	}
	target, err := g.Resolve(ctx, tree, "refs/heads/"+def)
	if err != nil || head == target {
		return false, err
	}
	if in, err := g.InHistory(ctx, tree, def, head); err != nil || !in {
		return false, err
	}
	if clean, err := g.Clean(ctx, tree); err != nil || !clean {
		return false, err
	}
	_, err = run(ctx, tree, "reset", "--quiet", "--keep", target)
	return err == nil, err
}

// Switch checks the worktree out on branch, creating it from base (a branch,
// else any commit git can name: origin/main, a hash) when the repo doesn't
// have it yet; created says it did. A worktree with changes is
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
	start, err := g.Resolve(ctx, dir, "refs/heads/"+base) // a branch first, over a tag of that name
	if err != nil {
		if start, err = g.Resolve(ctx, dir, base); err != nil {
			return false, fmt.Errorf("no branch or commit %q to start %s from", base, branch)
		}
	}
	// At the commit, so a remote's branch (origin/main) isn't set as upstream.
	_, err = run(ctx, dir, "checkout", "--quiet", "-b", branch, start)
	return err == nil, err
}

// CommitAll commits everything changed in the worktree on the branch checked
// out there, by the author, and returns the commit. After a merge with
// conflicts it concludes the merge, unless a submodule is still conflicted
// (*SubmoduleConflictError) or a conflicted file still holds markers
// (*ConflictsError). A repo inside the worktree that isn't a submodule is
// refused (*NestedReposError): git would commit a pointer to it, nothing more.
func (g *CLI) CommitAll(ctx context.Context, dir, message string, by Author) (string, error) {
	defer g.lock(ctx, dir)()
	if paths, err := unmergedSubmodules(ctx, dir); err != nil {
		return "", err
	} else if len(paths) > 0 {
		return "", &SubmoduleConflictError{Paths: paths}
	}
	if paths, err := nestedRepos(ctx, dir); err != nil {
		return "", err
	} else if len(paths) > 0 {
		return "", &NestedReposError{Paths: paths}
	}
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

// Name puts an idle worktree (detached, on no branch) on a new branch made
// where it stands, keeping what it holds not committed: an agent that started
// editing while idle names its topic (ErrNotDetached on a branch,
// ErrBranchExists when the repo has the branch).
func (g *CLI) Name(ctx context.Context, dir, branch string) error {
	if !ValidBranch(ctx, branch) {
		return ErrBadBranch
	}
	defer g.lock(ctx, dir)()
	if on, err := g.Branch(ctx, dir); err != nil {
		return err
	} else if on != "" {
		return ErrNotDetached
	}
	if g.HasBranch(ctx, dir, branch) {
		return ErrBranchExists
	}
	_, err := run(ctx, dir, "checkout", "--quiet", "-b", branch)
	return err
}

// Branch is the branch checked out in the worktree: "" when it is on none
// (detached: an idle worktree).
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
