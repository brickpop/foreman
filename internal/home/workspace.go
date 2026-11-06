package home

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// Workspaces (wiki/product/models/project-config.md#the-workspace): a
// project's folder of the CEO's, holding its project repo and its agents'
// sandboxes, so Foreman's home keeps only Foreman's own data. A project made
// before workspaces still lives in the home (projects/, sandboxes/) until it
// is migrated; legacy marks what goes once the CEO has migrated
// (wiki/backlog/remove-workspace-migration.md).

const (
	agentsDir  = "agents"   // in a workspace: one sandbox per agent
	filesDir   = "files"    // in a workspace: the files shared with its agents (Project.Files)
	harnessDir = "harness"  // in the home: each agent's private harness folder, <project>/<agent>
	legacyHarn = ".harness" // legacy: the harness folder inside a sandbox
)

// layout is where a project's files are: its project repo, its agents'
// sandboxes, and the folder of their harness folders ("" for legacy ones,
// inside each sandbox).
func (h *Home) layout(id, workspace string) (repo, agents, harness string) {
	if workspace == "" { // legacy
		return h.Path(projectsDir, id), h.Path(sandboxesDir, id), ""
	}
	return filepath.Join(workspace, projectRepo), filepath.Join(workspace, agentsDir), h.Path(harnessDir, id)
}

// Sandbox is an agent's sandbox on the project: its working directory,
// holding a worktree of each code repo.
func (p Project) Sandbox(agent string) string { return filepath.Join(p.agents, agent) }

const (
	FilesInbox  = "inbox"  // the CEO's files for the agents: read-only for them
	FilesShared = "shared" // the agents' data that outlives a task and a team: every agent writes it
)

// FileFolders are the folders of a project's files
// (wiki/product/models/project-config.md#the-workspace).
var FileFolders = []string{FilesInbox, FilesShared}

// MakeFiles makes the project's file folders that are missing; none for a
// legacy project.
func (p Project) MakeFiles() error {
	if p.Files == "" {
		return nil
	}
	for _, f := range FileFolders {
		if err := os.MkdirAll(filepath.Join(p.Files, f), 0o755); err != nil {
			return err
		}
	}
	return nil
}

// WorktreeAgent is the agent whose sandbox holds a worktree (its folder is
// <sandbox>/<repo>), sandbox giving an agent's sandbox; "" when it is no
// agent's (the CEO's checkout, another project's agent). Folders compare as
// the file system sees them: git reports worktrees with symlinks resolved
// (macOS's /var is /private/var).
func WorktreeAgent(tree string, sandbox func(agent string) string) string {
	dir := filepath.Dir(tree)
	agent := filepath.Base(dir)
	resolve := func(p string) string {
		if r, err := filepath.EvalSymlinks(p); err == nil {
			return r
		}
		return filepath.Clean(p)
	}
	if resolve(sandbox(agent)) == resolve(dir) {
		return agent
	}
	return ""
}

// HarnessDir is an agent's private harness folder: its config and
// conversations, in Foreman's home, which only the CEO can read.
func (p Project) HarnessDir(agent string) string {
	if p.harness == "" { // legacy
		return filepath.Join(p.Sandbox(agent), legacyHarn)
	}
	return filepath.Join(p.harness, agent)
}

// usable refuses a path Foreman may not use for a workspace or a repo
// (real: its real path): outside the CEO's home folder, or inside Foreman's
// home (the CEO, 2026-10-28).
func (h *Home) usable(path, real string) error {
	if !inside(real, h.allowed) {
		return fmt.Errorf("%w: %s is outside your home folder (%s)", ErrInvalid, path, h.allowed)
	}
	if home, err := filepath.EvalSymlinks(h.root); err == nil && inside(real, home) {
		return fmt.Errorf("%w: %s is inside Foreman's home (%s)", ErrInvalid, path, h.root)
	}
	return nil
}

// realPath is path's real path, resolving the folders of it that exist (a
// workspace may not exist yet).
func realPath(path string) (string, error) {
	var rest []string
	for dir := path; ; dir = filepath.Dir(dir) {
		if real, err := filepath.EvalSymlinks(dir); err == nil {
			return filepath.Join(append([]string{real}, rest...)...), nil
		}
		if filepath.Dir(dir) == dir {
			return "", fmt.Errorf("%w: %s can't be resolved", ErrInvalid, path)
		}
		rest = append([]string{filepath.Base(dir)}, rest...)
	}
}

// Suggestion is a new project's workspace as New project pre-fills it:
// <projects folder>/<name>.
type Suggestion struct {
	Path         string `json:"path"`         // the workspace; the projects folder when the name isn't a valid one (yet)
	Folder       string `json:"folder"`       // the projects folder, ~ expanded
	FolderExists bool   `json:"folderExists"` // false: creating the project creates it too
}

// SuggestWorkspace is where a project named name would go.
func (h *Home) SuggestWorkspace(name string) (Suggestion, error) {
	folder, err := expandHome(h.Settings().Settings.ProjectsFolder)
	if err != nil {
		return Suggestion{}, err
	}
	s := Suggestion{Path: folder, Folder: folder}
	if validProjectName(name) {
		s.Path = filepath.Join(folder, name)
	}
	info, err := os.Stat(folder)
	s.FolderExists = err == nil && info.IsDir()
	return s, nil
}

// expandHome turns a leading ~ into the CEO's home folder.
func expandHome(path string) (string, error) {
	if path != "~" && !strings.HasPrefix(path, "~/") {
		return path, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, strings.TrimPrefix(path, "~")), nil
}

// checkWorkspace checks a workspace for project id and returns it cleaned:
// an absolute path Foreman may use, existing or not, that isn't a file, nor
// another project's workspace, inside one or holding one, nor inside a code
// repo (its agents' worktrees would show in the repo). Called with h.mu held.
func (h *Home) checkWorkspace(id, path string) (string, error) {
	if !filepath.IsAbs(path) {
		return "", fmt.Errorf("%w: the workspace %s is not an absolute path", ErrInvalid, path)
	}
	path = filepath.Clean(path)
	real, err := realPath(path)
	if err != nil {
		return "", err
	}
	if err := h.usable(path, real); err != nil {
		return "", err
	}
	if info, err := os.Stat(path); err == nil && !info.IsDir() {
		return "", fmt.Errorf("%w: %s is a file", ErrExists, path)
	}
	for _, p := range h.snap.projects {
		if p.ID == id {
			continue
		}
		if other, err := realPath(p.Workspace); p.Workspace != "" && err == nil && (inside(real, other) || inside(other, real)) {
			return "", fmt.Errorf("%w: %s overlaps %s's workspace (%s)", ErrExists, path, p.ID, p.Workspace)
		}
		for _, r := range p.Repos {
			if repo, err := filepath.EvalSymlinks(r.Path); err == nil && inside(real, repo) {
				return "", fmt.Errorf("%w: %s is inside %s's repo %s", ErrInvalid, path, p.ID, r.Name)
			}
		}
	}
	return path, nil
}

// makeDir creates dir and its missing parents, and returns the outermost one
// it created ("" when dir existed), which undoes it whole.
func makeDir(dir string) (string, error) {
	made := ""
	for d := dir; !exists(d); d = filepath.Dir(d) {
		made = d
	}
	return made, os.MkdirAll(dir, 0o755)
}

// Migrate moves a project made before workspaces into workspace (the CEO,
// 2026-10-28): its project repo to <workspace>/project, its sandboxes (a
// stopped deployment's worktrees too) to <workspace>/agents, each agent's
// harness folder out of its sandbox into the home's harness/, its
// conversations carried over to the new sandbox path (Deps.Conversations),
// and each code repo's worktrees repaired. Refused while a team is deployed
// on it. A step that fails undoes those before it. Legacy: it goes once the
// CEO has migrated.
func (h *Home) Migrate(ctx context.Context, id, workspace string) (Project, error) {
	h.mu.Lock()
	defer h.unlock()
	p, ok := h.snap.projects[id]
	switch {
	case !ok:
		return Project{}, ErrNotFound
	case p.Workspace != "":
		return Project{}, fmt.Errorf("%w: %s is in its workspace already (%s)", ErrInvalid, id, p.Workspace)
	case h.deployed != nil && h.deployed(ctx, id):
		return Project{}, fmt.Errorf("%w: stop the team on %s before moving it", ErrDeployed, id)
	}
	ws, err := h.checkWorkspace(id, workspace)
	if err != nil {
		return Project{}, err
	}
	repo, agents, harness := h.layout(id, ws)
	for _, to := range []string{repo, agents} {
		if exists(to) {
			return Project{}, fmt.Errorf("%w: %s is there already", ErrExists, to)
		}
	}
	var undo []func()
	rollback := func(err error) (Project, error) {
		for _, u := range slices.Backward(undo) {
			u()
		}
		return Project{}, err
	}
	move := func(from, to string) error {
		if err := os.Rename(from, to); err != nil {
			return fmt.Errorf("moving %s to %s: %w", from, to, err)
		}
		undo = append(undo, func() { _ = os.Rename(to, from) })
		return nil
	}
	made, err := makeDir(ws)
	if err != nil {
		return Project{}, err
	}
	undo = append(undo, func() {
		if made != "" {
			_ = os.RemoveAll(made)
		}
	})
	// Repaired back once the folders are back (undone in reverse: this runs
	// after the moves are).
	repaired := map[string][]string{} // repo path: its worktrees where they were
	undo = append(undo, func() {
		for repo, trees := range repaired {
			_ = h.git.RepairWorktrees(ctx, repo, trees...)
		}
	})
	if err := move(p.Path, repo); err != nil {
		return rollback(err)
	}
	if exists(p.agents) {
		if err := move(p.agents, agents); err != nil {
			return rollback(err)
		}
	}
	entries, _ := os.ReadDir(agents)
	type worktree struct{ repo, from, to string }
	var worktrees []worktree
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		from, to := p.Sandbox(e.Name()), filepath.Join(agents, e.Name())
		if exists(filepath.Join(to, legacyHarn)) {
			if err := os.MkdirAll(harness, 0o700); err != nil {
				return rollback(err)
			}
			dir := filepath.Join(harness, e.Name())
			if err := move(filepath.Join(to, legacyHarn), dir); err != nil {
				return rollback(err)
			}
			if h.conversations != nil {
				if err := h.conversations(dir, from, to); err != nil {
					return rollback(fmt.Errorf("carrying %s's conversations over: %w", e.Name(), err))
				}
				undo = append(undo, func() { _ = h.conversations(dir, to, from) })
			}
		}
		for _, r := range p.Repos {
			if tree := filepath.Join(to, r.Name); exists(tree) {
				worktrees = append(worktrees, worktree{r.Path, filepath.Join(from, r.Name), tree})
			}
		}
	}
	// Each repo learns where its worktrees went; without it, a prune would
	// take them for deleted.
	for _, r := range p.Repos {
		var from, to []string
		for _, w := range worktrees {
			if w.repo == r.Path {
				from, to = append(from, w.from), append(to, w.to)
			}
		}
		if len(to) == 0 || !exists(r.Path) {
			continue
		}
		repaired[r.Path] = from
		if err := h.git.RepairWorktrees(ctx, r.Path, to...); err != nil {
			return rollback(err)
		}
	}
	if err := h.records.SetWorkspace(ctx, id, ws); err != nil {
		return rollback(h.recordError(err))
	}
	return h.rescan(id)
}
