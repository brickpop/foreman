package home

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// Cloning code repos (wiki/backlog/portable-projects.md): besides a folder
// the CEO picks, a repo can be cloned from its remote, with the CEO's own
// git, into repos/<path> of Foreman's folder, <path> being the URL's path
// without .git (me/ledger-api; a GitLab subgroup keeps its groups). Foreman
// clones only there, and never moves or deletes a clone once added: it is
// the CEO's, like any repo of theirs.

// cloneDir is where a repo cloned from remote goes: repos/<its URL's path>.
// Only a URL with a host (ssh, scp-like, http(s), git): a local repo is
// picked as its folder, never copied.
func (h *Home) cloneDir(remote string) (string, error) {
	var p string
	switch u, err := url.Parse(remote); {
	case err == nil && slices.Contains([]string{"ssh", "git", "http", "https"}, u.Scheme) && u.Host != "":
		p = u.Path
	case err == nil && u.Scheme != "":
		return "", fmt.Errorf("%w: %s: Foreman clones from an ssh, https or git URL", ErrInvalid, remote)
	default:
		// scp-like, [user@]host:path, as long as no slash comes before the colon
		// (that is a local path).
		host, rest, ok := strings.Cut(remote, ":")
		if !ok || host == "" || strings.Contains(host, "/") {
			return "", fmt.Errorf("%w: %s is a local path: pick its folder instead", ErrInvalid, remote)
		}
		p = rest
	}
	p = strings.TrimSuffix(strings.Trim(p, "/"), ".git")
	p = strings.TrimPrefix(p, "~") // ssh://host/~user/repo
	parts := strings.Split(strings.Trim(p, "/"), "/")
	for _, part := range parts {
		if !validName(part) {
			return "", fmt.Errorf("%w: %s: its path (%s) can't be a folder of Foreman's (each part %s)", ErrInvalid, remote, p, nameRules)
		}
	}
	return filepath.Join(append([]string{h.folder, reposDir}, parts...)...), nil
}

// clone makes dir a clone of remote, unless dir holds it already: a repo
// with root as its first commit, or, when root is "" (a repo being added),
// an origin that matches remote. Anything else there is refused, never touched.
// undo removes what this call made.
func (h *Home) clone(ctx context.Context, remote, dir, root string) (undo func(), err error) {
	if exists(dir) {
		switch {
		case !topOfRepo(dir):
			return nil, fmt.Errorf("%w: %s is there already and isn't a git repo: move it away, or pick a folder instead", ErrExists, dir)
		case root != "":
			if _, err := h.git.Resolve(ctx, dir, root); err != nil {
				return nil, fmt.Errorf("%w: %s holds another repo (it hasn't the first commit, %.8s): move it away, or pick a folder instead", ErrExists, dir, root)
			}
		default:
			if err := h.git.MatchOrigin(ctx, dir, remote); err != nil {
				return nil, fmt.Errorf("%w: %s %s. To use a separate checkout, clone into another folder and select A folder", ErrExists, dir, err)
			}
		}
		return func() {}, nil
	}
	made, err := makeDir(filepath.Dir(dir))
	if err != nil {
		return nil, err
	}
	undo = func() {
		_ = os.RemoveAll(dir)
		if made != "" {
			_ = os.RemoveAll(made)
		}
	}
	if err := h.git.Clone(ctx, remote, dir); err != nil {
		undo()
		return nil, fmt.Errorf("%w: %s", ErrInvalid, err.Error())
	}
	return undo, nil
}

// CloneRepo adds a repo to a project, named name (repoName), cloned from remote into
// repos/ (or the clone already there), with def as its default branch: for
// "", main when it has one, else the branch the remote's checkout is on.
func (h *Home) CloneRepo(ctx context.Context, project, name, remote, def string) (Project, error) {
	h.mu.Lock()
	defer h.unlock()
	p, ok := h.snap.projects[project]
	if !ok {
		return Project{}, ErrNotFound
	}
	id, name, err := repoName(p, "", name)
	if err != nil {
		return Project{}, err
	}
	dir, err := h.cloneDir(remote)
	if err != nil {
		return Project{}, err
	}
	undo, err := h.clone(ctx, remote, dir, "")
	if err != nil {
		return Project{}, err
	}
	if def == "" && !h.git.HasBranch(ctx, dir, "main") {
		def, _ = h.git.Branch(ctx, dir)
	}
	root, def, err := h.usableRepo(ctx, p, "", dir, def)
	if err == nil {
		p, err = h.addRepo(ctx, project, id, name, dir, root, def)
	}
	if err != nil {
		undo()
		return Project{}, err
	}
	return p, nil
}

// CloneMissingRepo locates a project's repo by cloning it from the remote
// its record keeps (a repo not on this machine yet): into repos/, or the
// clone already there when it is the same repo. Refused while a rename made
// on another machine waits (renameWaits).
func (h *Home) CloneMissingRepo(ctx context.Context, project, name string) (Project, error) {
	h.mu.Lock()
	defer h.unlock()
	p, ok := h.snap.projects[project]
	if !ok {
		return Project{}, ErrNotFound
	}
	i := slices.IndexFunc(p.Repos, func(r Repo) bool { return r.ID == name })
	if i < 0 {
		return Project{}, ErrNotFound
	}
	if err := h.renameWaits(ctx, project, name); err != nil {
		return Project{}, err
	}
	remote := h.snap.records.Records[project].Repos[name].Remote
	if remote == "" {
		return Project{}, fmt.Errorf("%w: %s has no remote recorded to clone it from: pick its folder", ErrInvalid, name)
	}
	dir, err := h.cloneDir(remote)
	if err != nil {
		return Project{}, err
	}
	root := p.Repos[i].root
	undo, err := h.clone(ctx, remote, dir, root)
	if err != nil {
		return Project{}, err
	}
	if _, err := h.usableFolder(p, name, dir); err != nil {
		undo()
		return Project{}, err
	}
	if _, err := h.git.Resolve(ctx, dir, root); err != nil {
		undo()
		return Project{}, fmt.Errorf("%w: %s is another repo now (it hasn't %s's first commit, %.8s)", ErrInvalid, remote, name, root)
	}
	if err := h.machine.SetRepoPath(ctx, project, name, dir); err != nil {
		undo()
		return Project{}, h.recordError(err)
	}
	return h.rescan(project)
}
