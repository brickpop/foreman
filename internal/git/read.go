package git

import (
	"context"
	"errors"
	"fmt"
	"path"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Reads for the CEO's Repositories tab (wiki/product/features/repositories.md):
// what is committed, at a ref, never a worktree's files. A ref and a path
// come from the browser, so both are checked before git sees them.

var (
	// ErrNoRef means the ref names no commit.
	ErrNoRef = errors.New("git: no such branch or commit")
	// ErrNoPath means the path isn't in the commit, or isn't what was asked
	// (a folder for a file, or the other way round).
	ErrNoPath = errors.New("git: no such file or folder")
)

// Resolve returns the commit ref names (a branch or a hash).
func (g *CLI) Resolve(ctx context.Context, dir, ref string) (string, error) {
	if ref == "" || strings.HasPrefix(ref, "-") || strings.ContainsAny(ref, ": \x00") {
		return "", ErrNoRef
	}
	out, err := run(ctx, dir, "rev-parse", "--verify", "--quiet", ref+"^{commit}")
	if err != nil {
		return "", ErrNoRef
	}
	return strings.TrimSpace(out), nil
}

// cleanPath is a path inside a commit's tree: relative, with no way out of
// it. "" is the root.
func cleanPath(p string) (string, error) {
	if strings.ContainsRune(p, 0) || strings.HasPrefix(p, "/") {
		return "", ErrNoPath
	}
	c := path.Clean(p)
	if c == "." {
		return "", nil
	}
	if c == ".." || strings.HasPrefix(c, "../") {
		return "", ErrNoPath
	}
	return c, nil
}

// Tree lists the folder at p in commit, folders first, then by name.
func (g *CLI) Tree(ctx context.Context, dir, commit, p string) ([]TreeEntry, error) {
	p, err := cleanPath(p)
	if err != nil {
		return nil, err
	}
	if t, err := g.kind(ctx, dir, commit, p); err != nil || t != "tree" {
		return nil, ErrNoPath
	}
	out, err := run(ctx, dir, "ls-tree", "-z", "--long", commit+":"+p)
	if err != nil {
		return nil, err
	}
	var dirs, files []TreeEntry
	for _, rec := range strings.Split(strings.TrimSuffix(out, "\x00"), "\x00") {
		if rec == "" {
			continue
		}
		// <mode> <type> <object> <size>\t<name>
		meta, name, ok := strings.Cut(rec, "\t")
		f := strings.Fields(meta)
		if !ok || len(f) != 4 {
			return nil, fmt.Errorf("git ls-tree: unexpected %q", rec)
		}
		e := TreeEntry{Name: name}
		switch {
		case f[1] == "tree":
			e.Kind = "dir"
		case f[1] == "commit":
			e.Kind = "module"
		case f[0] == "120000":
			e.Kind = "link"
		default:
			e.Kind = "file"
			e.Size, _ = strconv.ParseInt(f[3], 10, 64)
		}
		if e.Kind == "dir" {
			dirs = append(dirs, e)
		} else {
			files = append(files, e)
		}
	}
	return append(append([]TreeEntry{}, dirs...), files...), nil
}

// File returns the file at p in commit and its size; content is nil when the
// file is over limit bytes.
func (g *CLI) File(ctx context.Context, dir, commit, p string, limit int64) (content []byte, size int64, err error) {
	p, err = cleanPath(p)
	if err != nil || p == "" {
		return nil, 0, ErrNoPath
	}
	if t, err := g.kind(ctx, dir, commit, p); err != nil || t != "blob" {
		return nil, 0, ErrNoPath
	}
	out, err := run(ctx, dir, "cat-file", "-s", commit+":"+p)
	if err != nil {
		return nil, 0, err
	}
	if size, err = strconv.ParseInt(strings.TrimSpace(out), 10, 64); err != nil {
		return nil, 0, err
	}
	if size > limit {
		return nil, size, nil
	}
	out, err = run(ctx, dir, "cat-file", "blob", commit+":"+p)
	return []byte(out), size, err
}

// kind is the object type at p in commit: tree, blob or commit; an error when
// there is nothing there.
func (g *CLI) kind(ctx context.Context, dir, commit, p string) (string, error) {
	out, err := run(ctx, dir, "cat-file", "-t", commit+":"+p)
	return strings.TrimSpace(out), err
}

// Log returns up to n commits of commit's history, newest first, after
// skipping skip.
func (g *CLI) Log(ctx context.Context, dir, commit string, skip, n int) ([]Commit, error) {
	// Children before their parents, also within one second; decorated with
	// the branches only (no HEAD, no tags).
	out, err := run(ctx, dir, "log", "--topo-order", "--decorate-refs=refs/heads/", "--format=%H%x1f%an%x1f%aI%x1f%P%x1f%D%x1f%s%x1e",
		"--skip="+strconv.Itoa(skip), "-n", strconv.Itoa(n), commit, "--")
	if err != nil {
		return nil, err
	}
	list := []Commit{}
	for _, rec := range strings.Split(out, "\x1e") {
		rec = strings.TrimSpace(rec)
		if rec == "" {
			continue
		}
		f := strings.SplitN(rec, "\x1f", 6)
		if len(f) != 6 {
			return nil, fmt.Errorf("git log: unexpected %q", rec)
		}
		date, err := time.Parse(time.RFC3339, f[2])
		if err != nil {
			return nil, err
		}
		branches := []string{}
		if f[4] != "" {
			branches = strings.Split(f[4], ", ") // branch names have no spaces
			slices.Sort(branches)
		}
		list = append(list, Commit{Hash: f[0], Author: f[1], Date: date, Subject: f[5], Merge: len(strings.Fields(f[3])) > 1, Branches: branches})
	}
	return list, nil
}

// Worktrees maps each branch checked out in one of the repo's worktrees
// (the repo's own checkout included) to that worktree's folder.
func (g *CLI) Worktrees(ctx context.Context, dir string) (map[string]string, error) {
	out, err := run(ctx, dir, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}
	trees := map[string]string{}
	var at string
	for _, line := range strings.Split(out, "\n") {
		switch {
		case strings.HasPrefix(line, "worktree "):
			at = strings.TrimPrefix(line, "worktree ")
		case strings.HasPrefix(line, "branch refs/heads/"):
			trees[strings.TrimPrefix(line, "branch refs/heads/")] = at
		}
	}
	return trees, nil
}

// RecentBranches lists the repo's branches by their last commit, newest
// first (by name when two were committed at the same second).
func (g *CLI) RecentBranches(ctx context.Context, dir string) ([]string, error) {
	out, err := run(ctx, dir, "for-each-ref", "--sort=-committerdate", "--format=%(refname:short)", "refs/heads/")
	if err != nil {
		return nil, err
	}
	return strings.Fields(out), nil
}

// RootCommit is a first commit of dir's history (of several, joined by a
// merge, the lowest hash: commit dates tie): it names the repo wherever its
// folder goes, so a moved repo is recognised. ErrNoRef with no commit yet.
func (g *CLI) RootCommit(ctx context.Context, dir string) (string, error) {
	out, err := run(ctx, dir, "rev-list", "--max-parents=0", "HEAD")
	roots := strings.Fields(out)
	if err != nil || len(roots) == 0 {
		return "", ErrNoRef
	}
	return slices.Min(roots), nil
}
