// Package repos serves a project's Repos section
// (wiki/product/features/repos.md): its code repos and its project
// repo, read-only, as committed.
package repos

import (
	"bytes"
	"context"
	"errors"
	"regexp"
	"slices"
	"strings"

	"github.com/brickpop/foreman/internal/git"
	"github.com/brickpop/foreman/internal/home"
	"github.com/brickpop/foreman/internal/wiki"
)

// FileLimit is the largest file shown; a larger one says so.
const FileLimit = 1 << 20

// logPage is how many commits a page of history holds.
const logPage = 50

// hq is the project's HQ's name, listed among the repos.
const hq = "hq"

// markdown names the files shown rendered, their frontmatter apart.
var markdown = regexp.MustCompile(`(?i)\.(md|markdown)$`)

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
	Detached(ctx context.Context, dir string) ([]string, error)
	Clean(ctx context.Context, dir string) (bool, error)
	Resolve(ctx context.Context, dir, ref string) (string, error)
	Tree(ctx context.Context, dir, commit, path string) ([]git.TreeEntry, error)
	File(ctx context.Context, dir, commit, path string, limit int64) ([]byte, int64, error)
	Log(ctx context.Context, dir, commit string, skip, n int) ([]git.Commit, error)
	Count(ctx context.Context, dir, commit string) (int, error)
}

// Browser reads a project's repos.
type Browser struct {
	Config Config
	Git    Git
}

// dir is the folder of one of the project's repos, with its default branch.
func (b Browser) dir(project, repo string) (string, string, error) {
	p, err := b.Config.Project(project)
	if err != nil {
		return "", "", err
	}
	dir, ok := p.RepoDir(repo)
	if !ok {
		return "", "", ErrNoRepo
	}
	return dir, p.Default(repo), nil
}

// Repos lists the project's code repos agents can use (a missing one is a
// problem shown on the project), then its HQ, each with its
// branches: the default first, then the others by name.
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
	for _, name := range append(names, hq) {
		dir, def, err := b.dir(project, name)
		if err != nil {
			return nil, err
		}
		branches, err := b.branches(ctx, project, dir, def)
		if err != nil {
			return nil, err
		}
		idle, err := b.idle(ctx, project, dir)
		if err != nil {
			return nil, err
		}
		list = append(list, Repo{Name: name, HQ: name == hq, Default: def, Branches: branches, Idle: idle})
	}
	return list, nil
}

func (b Browser) branches(ctx context.Context, project, dir, def string) ([]Branch, error) {
	names, err := b.Git.RecentBranches(ctx, dir)
	if err != nil {
		return nil, err
	}
	trees, err := b.Git.Worktrees(ctx, dir)
	if err != nil {
		return nil, err
	}
	// A default branch deleted outside Foreman leaves nothing to count from:
	// its branches show no ahead count.
	hasDefault := slices.Contains(names, def)
	list := []Branch{}
	for _, name := range names {
		br := Branch{Name: name}
		if hasDefault && name != def {
			if br.Ahead, err = b.Git.Ahead(ctx, dir, name, def); err != nil {
				return nil, err
			}
		}
		if tree, ok := trees[name]; ok {
			clean, err := b.Git.Clean(ctx, tree)
			br.Uncommitted = err == nil && !clean
			br.Agent = home.WorktreeAgent(tree, func(agent string) string { return b.Config.Sandbox(project, agent) })
		}
		list = append(list, br)
	}
	// The default first, then the most recently committed.
	slices.SortStableFunc(list, func(x, y Branch) int {
		switch {
		case x.Name == def && y.Name != def:
			return -1
		case y.Name == def && x.Name != def:
			return 1
		}
		return 0
	})
	return list, nil
}

// idle lists the agents' worktrees of the repo on no branch; any other
// detached worktree (the CEO's own, say) is nobody's and left out.
func (b Browser) idle(ctx context.Context, project, dir string) ([]Idle, error) {
	trees, err := b.Git.Detached(ctx, dir)
	if err != nil {
		return nil, err
	}
	list := []Idle{}
	for _, tree := range trees {
		agent := home.WorktreeAgent(tree, func(agent string) string { return b.Config.Sandbox(project, agent) })
		if agent == "" {
			continue
		}
		clean, err := b.Git.Clean(ctx, tree)
		list = append(list, Idle{Agent: agent, Uncommitted: err == nil && !clean})
	}
	slices.SortFunc(list, func(x, y Idle) int { return strings.Compare(x.Agent, y.Agent) })
	return list, nil
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
		if f.Markdown = markdown.MatchString(path); f.Markdown {
			f.Frontmatter, f.Body, f.Headings, f.FrontmatterError = wiki.Markdown(path, f.Text)
			t := &trees{ctx: ctx, git: b.Git, dir: dir, commit: commit, read: map[string][]git.TreeEntry{}}
			f.Links = t.links(path, f.Text)
		}
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
	total, err := b.Git.Count(ctx, dir, commit)
	if err != nil {
		return LogPage{}, err
	}
	page := LogPage{Commits: commits, Total: total}
	if len(commits) == logPage {
		page.Next = max(skip, 0) + logPage
	}
	return page, nil
}
