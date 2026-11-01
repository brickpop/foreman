// Package repos serves a project's Repositories tab
// (wiki/product/features/repositories.md): its code repos and its project
// repo, read-only, as committed.
package repos

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"slices"

	"github.com/brickpop/foreman/internal/git"
	"github.com/brickpop/foreman/internal/home"
)

// FileLimit is the largest file shown; a larger one says so.
const FileLimit = 1 << 20

// logPage is how many commits a page of history holds.
const logPage = 50

// projectRepo is the project repo's folder, listed among the repos.
const projectRepo = "project"

// ErrNoRepo means the project has no such repo.
var ErrNoRepo = errors.New("repos: no such repo")

// Config is what the browser needs of the home.
type Config interface {
	Project(id string) (home.Project, error)
	// Sandbox is an agent's folder of worktrees in the project.
	Sandbox(project, agent string) string
}

// Git is what the browser needs of the Git block.
type Git interface {
	RecentBranches(ctx context.Context, dir string) ([]string, error)
	Ahead(ctx context.Context, dir, branch, base string) (int, error)
	Worktrees(ctx context.Context, dir string) (map[string]string, error)
	Clean(ctx context.Context, dir string) (bool, error)
	Resolve(ctx context.Context, dir, ref string) (string, error)
	Tree(ctx context.Context, dir, commit, path string) ([]git.TreeEntry, error)
	File(ctx context.Context, dir, commit, path string, limit int64) ([]byte, int64, error)
	Log(ctx context.Context, dir, commit string, skip, n int) ([]git.Commit, error)
}

// Browser reads a project's repos.
type Browser struct {
	Config Config
	Git    Git
}

// dir is the folder of one of the project's repos, with its main branch.
func (b Browser) dir(project, repo string) (string, string, error) {
	p, err := b.Config.Project(project)
	if err != nil {
		return "", "", err
	}
	dir, ok := p.RepoDir(repo)
	if !ok {
		return "", "", ErrNoRepo
	}
	return dir, p.Main(repo), nil
}

// Repos lists the project's code repos agents can use (a missing one is a
// problem shown on the project), then its project repo, each with its
// branches: main first, then the others by name.
func (b Browser) Repos(ctx context.Context, project string) ([]Repo, error) {
	p, err := b.Config.Project(project)
	if err != nil {
		return nil, err
	}
	list := []Repo{}
	var names []string
	for _, r := range p.Repos {
		if r.Ready() {
			names = append(names, r.Name)
		}
	}
	for _, name := range append(names, projectRepo) {
		dir, main, err := b.dir(project, name)
		if err != nil {
			return nil, err
		}
		branches, err := b.branches(ctx, project, dir, main)
		if err != nil {
			return nil, err
		}
		list = append(list, Repo{Name: name, Project: name == projectRepo, Main: main, Branches: branches})
	}
	return list, nil
}

func (b Browser) branches(ctx context.Context, project, dir, main string) ([]Branch, error) {
	names, err := b.Git.RecentBranches(ctx, dir)
	if err != nil {
		return nil, err
	}
	trees, err := b.Git.Worktrees(ctx, dir)
	if err != nil {
		return nil, err
	}
	// A repo cloned with another default branch (master) has no main to
	// count from: its branches show no ahead count.
	hasMain := slices.Contains(names, main)
	list := []Branch{}
	for _, name := range names {
		br := Branch{Name: name}
		if hasMain && name != main {
			if br.Ahead, err = b.Git.Ahead(ctx, dir, name, main); err != nil {
				return nil, err
			}
		}
		if tree, ok := trees[name]; ok {
			clean, err := b.Git.Clean(ctx, tree)
			br.Uncommitted = err == nil && !clean
			br.Agent = b.agentOf(project, tree)
		}
		list = append(list, br)
	}
	// main first, then the most recently committed.
	slices.SortStableFunc(list, func(x, y Branch) int {
		switch {
		case x.Name == main && y.Name != main:
			return -1
		case y.Name == main && x.Name != main:
			return 1
		}
		return 0
	})
	return list, nil
}

// agentOf is the agent whose sandbox holds the worktree (its folder is
// <sandbox>/<repo>), or "" when it is no agent's (the CEO's checkout).
func (b Browser) agentOf(project, tree string) string {
	sandbox := filepath.Dir(tree)
	agent := filepath.Base(sandbox)
	if samePath(b.Config.Sandbox(project, agent), sandbox) {
		return agent
	}
	return ""
}

// samePath compares two folders as the file system sees them: git reports
// worktrees with symlinks resolved (macOS's /var is /private/var).
func samePath(a, b string) bool {
	resolve := func(p string) string {
		if r, err := filepath.EvalSymlinks(p); err == nil {
			return r
		}
		return filepath.Clean(p)
	}
	return resolve(a) == resolve(b)
}

// at is the repo's folder and the commit ref names.
func (b Browser) at(ctx context.Context, project, repo, ref string) (string, string, error) {
	dir, _, err := b.dir(project, repo)
	if err != nil {
		return "", "", err
	}
	commit, err := b.Git.Resolve(ctx, dir, ref)
	return dir, commit, err
}

// Tree lists the folder at path on ref.
func (b Browser) Tree(ctx context.Context, project, repo, ref, path string) ([]git.TreeEntry, error) {
	dir, commit, err := b.at(ctx, project, repo, ref)
	if err != nil {
		return nil, err
	}
	return b.Git.Tree(ctx, dir, commit, path)
}

// File returns the file at path on ref: its text, unless it is binary or
// larger than FileLimit.
func (b Browser) File(ctx context.Context, project, repo, ref, path string) (File, error) {
	dir, commit, err := b.at(ctx, project, repo, ref)
	if err != nil {
		return File{}, err
	}
	content, size, err := b.Git.File(ctx, dir, commit, path, FileLimit)
	if err != nil {
		return File{}, err
	}
	f := File{Path: path, Size: size}
	switch {
	case content == nil && size > FileLimit:
		f.TooLarge = true
	case bytes.IndexByte(content[:min(len(content), 8000)], 0) >= 0:
		f.Binary = true
	default:
		f.Text = string(content)
	}
	return f, nil
}

// Log returns a page of ref's history, or with all every branch's (ref
// unread), after skip commits.
func (b Browser) Log(ctx context.Context, project, repo, ref string, all bool, skip int) (LogPage, error) {
	var dir, commit string
	var err error
	if all {
		dir, _, err = b.dir(project, repo)
	} else {
		dir, commit, err = b.at(ctx, project, repo, ref)
	}
	if err != nil {
		return LogPage{}, err
	}
	commits, err := b.Git.Log(ctx, dir, commit, max(skip, 0), logPage)
	if err != nil {
		return LogPage{}, err
	}
	page := LogPage{Commits: commits}
	if len(commits) == logPage {
		page.Next = max(skip, 0) + logPage
	}
	return page, nil
}
