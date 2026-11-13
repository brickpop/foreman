package home

import (
	"cmp"
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/brickpop/foreman/internal/git"
	"github.com/brickpop/foreman/internal/profiles"
	"github.com/brickpop/foreman/internal/store"
)

// Adding projects and their repos (wiki/product/features/add-project.md). A
// project is a record in the database and a workspace, with its project repo;
// code repos are the CEO's, anywhere in their home folder. Foreman creates a
// repo only new and empty, with a first commit so its default branch exists
// (agents' branches start from it).

var (
	// ErrInvalid means a request can't be carried out as asked; the error says why.
	ErrInvalid = errors.New("home: invalid")
	// ErrExists means the name or the folder is taken.
	ErrExists = errors.New("home: exists")
	// ErrDeployed means a team is deployed on the project: stop it first.
	ErrDeployed = errors.New("home: a team is deployed on the project")
)

// NewProject creates a project with no repos yet, in workspace (a folder of
// the CEO's, made if missing, Home.checkWorkspace): its record, and its
// project repo at <workspace>/project, new (project.toml and a starter wiki,
// in one first commit) or cloned from clone (a URL or a local path, as the
// CEO's git can reach it). The name is a slug (validProjectName) and must be
// new, in any case. A project repo already at <workspace>/project (a removed
// project's, kept) is taken over as it is. Nothing this call made is left
// behind if a step fails. created is false for a takeover.
func (h *Home) NewProject(ctx context.Context, name, clone, workspace string) (p Project, created bool, err error) {
	h.mu.Lock()
	defer h.unlock()
	if !validProjectName(name) {
		return Project{}, false, fmt.Errorf("%w: a project name is lowercase letters, digits and single hyphens (my-portfolio), at most 63", ErrInvalid)
	}
	if existing, ok := h.projectNamed(name); ok {
		return Project{}, false, fmt.Errorf("%w: project %s already exists", ErrExists, existing)
	}
	if other, ok := h.shownAs(name, readable(name)); ok {
		return Project{}, false, fmt.Errorf("%w: project %s shows as %s already: pick another name, or rename it first", ErrExists, other, readable(name))
	}
	ws, err := h.checkWorkspace(name, workspace)
	if err != nil {
		return Project{}, false, err
	}
	dir, _, _ := h.layout(name, ws)
	create := h.createProjectRepo
	if clone != "" {
		create = func(ctx context.Context, _, dir string) error {
			if err := h.git.Clone(ctx, clone, dir); err != nil {
				return fmt.Errorf("%w: %s", ErrInvalid, err.Error())
			}
			return nil
		}
	}
	takeOver := exists(dir)
	if takeOver {
		if err := canTakeOver(dir, clone); err != nil {
			return Project{}, false, err
		}
		create = func(context.Context, string, string) error { return nil }
	}
	made, err := makeDir(ws)
	if err != nil {
		return Project{}, false, err
	}
	// Only folders this call made go when a step fails.
	drop := func() {
		switch {
		case made != "":
			_ = os.RemoveAll(made)
		case !takeOver:
			_ = os.RemoveAll(dir)
		}
	}
	if err := create(ctx, name, dir); err != nil {
		drop()
		return Project{}, false, err
	}
	if err := h.records.Create(ctx, name, ws); err != nil {
		drop()
		if errors.Is(err, store.ErrExists) {
			return Project{}, false, fmt.Errorf("%w: project %s already exists", ErrExists, name)
		}
		return Project{}, false, err
	}
	p, err = h.rescan(name)
	return p, !takeOver, err
}

// canTakeOver says why the folder at dir can't become a new project's repo
// as it is: a clone asked for, or not a git repo's top folder of its own.
func canTakeOver(dir, clone string) error {
	info, err := os.Lstat(dir)
	switch {
	case err != nil:
		return err
	case clone != "":
		return fmt.Errorf("%w: %s is there already: create the project without Clone to take it over, or move it away", ErrExists, dir)
	case !info.IsDir() || !topOfRepo(dir):
		return fmt.Errorf("%w: %s is there already and isn't a git repo: move it away, or pick another workspace", ErrExists, dir)
	}
	return nil
}

// projectNamed finds a project named name in any case. Called with h.mu held.
// shownAs finds the project, other than id, that shows as shown (its
// display name, else its id made readable), in any case: two projects never
// read the same (the CEO, 2026-11-04).
func (h *Home) shownAs(id, shown string) (string, bool) {
	for _, p := range h.snap.projects {
		if p.ID != id && strings.EqualFold(p.Shown(), shown) {
			return p.ID, true
		}
	}
	return "", false
}

func (h *Home) projectNamed(name string) (string, bool) {
	for id := range h.snap.projects {
		if strings.EqualFold(id, name) {
			return id, true
		}
	}
	return "", false
}

// createProjectRepo makes dir the project repo: the scaffold (project.toml
// and a starter wiki), in one first commit.
func (h *Home) createProjectRepo(ctx context.Context, project, dir string) error {
	created, err := writeScaffold(project, dir)
	if err != nil {
		return err
	}
	if err := h.git.Init(ctx, dir, "main"); err != nil {
		return err
	}
	return h.git.Commit(ctx, dir, "Project repo", created...)
}

// AddRepo adds an existing repo to a project, under name: path is its top
// folder, which agents can use (Home.repoFolder), with a commit, and
// not in the project already, with def as its default branch (usableRepo). A
// repo may be in several projects.
func (h *Home) AddRepo(ctx context.Context, project, name, path, def string) (Project, error) {
	h.mu.Lock()
	defer h.unlock()
	p, ok := h.snap.projects[project]
	if !ok {
		return Project{}, ErrNotFound
	}
	if err := checkRepoName(p, name); err != nil {
		return Project{}, err
	}
	root, def, err := h.usableRepo(ctx, p, "", path, def)
	if err != nil {
		return Project{}, err
	}
	if err := h.records.AddRepo(ctx, project, store.RepoRecord{Name: name, Path: filepath.Clean(path), RootCommit: root, Default: def}); err != nil {
		return Project{}, h.recordError(err)
	}
	return h.rescan(project)
}

// NewRepo creates an empty code repo at path, a folder that doesn't exist yet
// in one that does, on main, and adds it to the project.
func (h *Home) NewRepo(ctx context.Context, project, name, path string) (Project, error) {
	h.mu.Lock()
	defer h.unlock()
	p, ok := h.snap.projects[project]
	if !ok {
		return Project{}, ErrNotFound
	}
	if err := checkRepoName(p, name); err != nil {
		return Project{}, err
	}
	if !filepath.IsAbs(path) {
		return Project{}, fmt.Errorf("%w: %s is not an absolute path", ErrInvalid, path)
	}
	path = filepath.Clean(path)
	if exists(path) {
		return Project{}, fmt.Errorf("%w: %s already exists", ErrExists, path)
	}
	parent, err := filepath.EvalSymlinks(filepath.Dir(path))
	if err != nil {
		return Project{}, fmt.Errorf("%w: %s must be a new folder in an existing one", ErrInvalid, path)
	}
	if err := h.usable(path, filepath.Join(parent, filepath.Base(path))); err != nil {
		return Project{}, err
	}
	if err := h.outsideForemans(filepath.Join(parent, filepath.Base(path))); err != nil {
		return Project{}, err
	}
	if err := h.createRepo(ctx, path, "main"); err != nil {
		_ = os.RemoveAll(path)
		return Project{}, err
	}
	root, def, err := h.usableRepo(ctx, p, "", path, "main")
	if err == nil {
		err = h.recordError(h.records.AddRepo(ctx, project, store.RepoRecord{Name: name, Path: path, RootCommit: root, Default: def}))
	}
	if err != nil {
		_ = os.RemoveAll(path)
		return Project{}, err
	}
	return h.rescan(project)
}

// LocateRepo points a project's repo at its new folder: the same repo (it
// holds the repo's first commit), which agents can use.
func (h *Home) LocateRepo(ctx context.Context, project, name, path string) (Project, error) {
	h.mu.Lock()
	defer h.unlock()
	p, ok := h.snap.projects[project]
	if !ok {
		return Project{}, ErrNotFound
	}
	i := slices.IndexFunc(p.Repos, func(r Repo) bool { return r.Name == name })
	if i < 0 {
		return Project{}, ErrNotFound
	}
	want := p.Repos[i].root
	real, err := h.usableFolder(p, name, path)
	if err != nil {
		return Project{}, err
	}
	if _, err := h.git.Resolve(ctx, real, want); err != nil {
		return Project{}, fmt.Errorf("%w: %s is another repo (it hasn't %s's first commit, %.8s)", ErrInvalid, path, name, want)
	}
	if err := h.records.MoveRepo(ctx, project, name, filepath.Clean(path)); err != nil {
		return Project{}, h.recordError(err)
	}
	return h.rescan(project)
}

// RemoveRepo takes a repo out of a project; its folder and branches stay.
// Refused while a team is deployed on the project: its agents work in it.
func (h *Home) RemoveRepo(ctx context.Context, project, name string) (Project, error) {
	h.mu.Lock()
	defer h.unlock()
	if _, ok := h.snap.projects[project]; !ok {
		return Project{}, ErrNotFound
	}
	if h.deployed != nil && h.deployed(ctx, project) {
		return Project{}, fmt.Errorf("%w: stop the team on %s before removing %s", ErrDeployed, project, name)
	}
	if err := h.records.RemoveRepo(ctx, project, name); err != nil {
		return Project{}, h.recordError(err)
	}
	return h.rescan(project)
}

// SetRepoDefault sets the branch a repo's agents start from and its merges
// go to unless the lead names another: one the repo has, not an agent's.
// Merge requests keep the branch they go into, and worktrees made before stay
// on their branches.
func (h *Home) SetRepoDefault(ctx context.Context, project, name, branch string) (Project, error) {
	h.mu.Lock()
	defer h.unlock()
	p, ok := h.snap.projects[project]
	if !ok {
		return Project{}, ErrNotFound
	}
	i := slices.IndexFunc(p.Repos, func(r Repo) bool { return r.Name == name })
	if i < 0 {
		return Project{}, ErrNotFound
	}
	if git.AgentBranch(branch) {
		return Project{}, fmt.Errorf("%w: %s is an agent's branch (wt/*, lead/*): agents write it, so it can't be the default", ErrInvalid, branch)
	}
	if !h.git.HasBranch(ctx, p.Repos[i].Path, branch) {
		return Project{}, fmt.Errorf("%w: %s has no branch %q", ErrInvalid, name, branch)
	}
	if err := h.records.SetRepoDefault(ctx, project, name, branch); err != nil {
		return Project{}, h.recordError(err)
	}
	return h.rescan(project)
}

// SetProfile names the profile a project's agents run on: a profile's name,
// or the default.
func (h *Home) SetProfile(ctx context.Context, project, profile string) (Project, error) {
	h.mu.Lock()
	defer h.unlock()
	if _, ok := h.snap.projects[project]; !ok {
		return Project{}, ErrNotFound
	}
	if !profiles.Valid(profile) {
		return Project{}, fmt.Errorf("%w: %q is not a profile name", ErrInvalid, profile)
	}
	if profile == profiles.Default {
		profile = ""
	}
	if err := h.records.SetProfile(ctx, project, profile); err != nil {
		return Project{}, h.recordError(err)
	}
	return h.rescan(project)
}

// SetHidden hides a project, or shows it again: where it is listed, nothing
// else (a team on it keeps working).
func (h *Home) SetHidden(ctx context.Context, project string, hidden bool) (Project, error) {
	h.mu.Lock()
	defer h.unlock()
	if _, ok := h.snap.projects[project]; !ok {
		return Project{}, ErrNotFound
	}
	if err := h.records.SetHidden(ctx, project, hidden); err != nil {
		return Project{}, h.recordError(err)
	}
	return h.rescan(project)
}

// maxProjectName is the most characters a project's display name has.
const maxProjectName = 80

// SetName gives a project its display name, any words (trimmed), "" to show
// it by its id again. Only how it shows changes, so a team may be deployed.
func (h *Home) SetName(ctx context.Context, project, name string) (Project, error) {
	name = strings.TrimSpace(name)
	if utf8.RuneCountInString(name) > maxProjectName {
		return Project{}, fmt.Errorf("%w: a project's name has at most %d characters", ErrInvalid, maxProjectName)
	}
	if strings.ContainsFunc(name, unicode.IsControl) {
		return Project{}, fmt.Errorf("%w: a project's name is one line of text", ErrInvalid)
	}
	h.mu.Lock()
	defer h.unlock()
	if _, ok := h.snap.projects[project]; !ok {
		return Project{}, ErrNotFound
	}
	shown := cmp.Or(name, readable(project))
	if other, ok := h.shownAs(project, shown); ok {
		return Project{}, fmt.Errorf("%w: project %s shows as %s already", ErrExists, other, shown)
	}
	if err := h.records.SetName(ctx, project, name); err != nil {
		return Project{}, h.recordError(err)
	}
	return h.rescan(project)
}

// usableRepo checks a folder to add as a repo of p (Home.usableFolder)
// and returns its root commit: a repo without a commit has no branch for
// agents to start from, and its default branch: def, a branch it has; for
// "", main when it has that branch, else the CEO picks one (RepoBranches).
func (h *Home) usableRepo(ctx context.Context, p Project, name, path, def string) (string, string, error) {
	real, err := h.usableFolder(p, name, path)
	if err != nil {
		return "", "", err
	}
	root, err := h.git.RootCommit(ctx, real)
	if err != nil {
		return "", "", fmt.Errorf("%w: %s has no commit yet: commit once, then add it", ErrInvalid, path)
	}
	switch {
	case def == "" && h.git.HasBranch(ctx, real, "main"):
		def = "main"
	case def == "":
		return "", "", fmt.Errorf("%w: %s has no branch main: pick its default branch", ErrInvalid, path)
	case git.AgentBranch(def):
		return "", "", fmt.Errorf("%w: %s is an agent's branch (wt/*, lead/*): agents write it, so it can't be the default", ErrInvalid, def)
	case !h.git.HasBranch(ctx, real, def):
		return "", "", fmt.Errorf("%w: %s has no branch %q", ErrInvalid, path, def)
	}
	return root, def, nil
}

// usableFolder is the real path of a folder agents can use as one of p's
// repos (Home.usable), not one p has already,
// except the repo named name (Locate).
func (h *Home) usableFolder(p Project, name, path string) (string, error) {
	real, err := h.repoFolder(path)
	if err != nil {
		return "", fmt.Errorf("%w: %s", ErrInvalid, err.Error())
	}
	if err := h.usable(path, real); err != nil {
		return "", err
	}
	if err := h.outsideForemans(real); err != nil {
		return "", err
	}
	for _, r := range p.Repos {
		if other, err := filepath.EvalSymlinks(r.Path); err == nil && other == real && r.Name != name {
			return "", fmt.Errorf("%w: %s is %s's repo %s already", ErrExists, path, p.ID, r.Name)
		}
	}
	return real, nil
}

// recordError turns the database's refusals into the home's.
func (h *Home) recordError(err error) error {
	switch {
	case errors.Is(err, store.ErrExists):
		return fmt.Errorf("%w: the project has that repo already", ErrExists)
	case errors.Is(err, store.ErrNotFound):
		return ErrNotFound
	}
	return err
}

// starter is the scaffold: what a project repo holds besides project.toml,
// laid out as in the repo, "{project}" standing for the project's name
// (wiki/product/features/add-project.md#effect). The wiki follows the
// conventions its AGENTS.md describes.
//
//go:embed all:starter
var starter embed.FS

// scaffold is the starter's files in order, each with what goes wrong
// without it; the wiki can't be read without the required ones.
var scaffold = []Missing{
	{"project.toml", "Foreman can't start the project's agents without it", false},
	{wikiDir + "/wiki.toml", "Foreman can't read the wiki or its boards without it", true},
	{wikiDir + "/index.md", "the wiki has no home page", true},
	{"AGENTS.md", "the agents don't know how this project's wiki and tasks work", false},
	{".gitignore", "the wiki's index and editor files can be committed", false},
	{wikiDir + "/product/index.md", "the specs have no map", false},
	{wikiDir + "/backlog/index.md", "the backlog has no entry point", false},
	{wikiDir + "/technical-debt.md", "technical debt has no page", false},
}

// starterFile is a scaffold file's content for the project.
func starterFile(project, file string) (string, error) {
	data, err := starter.ReadFile("starter/" + file)
	return strings.ReplaceAll(string(data), "{project}", project), err
}

// writeScaffold writes the scaffold files dir lacks, never overwriting one,
// and returns those it wrote.
func writeScaffold(project, dir string) ([]string, error) {
	var created []string
	for _, f := range scaffold {
		if exists(filepath.Join(dir, f.File)) {
			continue
		}
		content, err := starterFile(project, f.File)
		if err != nil {
			return created, err
		}
		if err := makeFile(filepath.Join(dir, f.File), content); err != nil {
			return created, err
		}
		created = append(created, f.File)
	}
	return created, nil
}

func exists(path string) bool {
	_, err := os.Lstat(path)
	return !errors.Is(err, fs.ErrNotExist)
}

func makeFile(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

// Missing lists the scaffold files the project's repo lacks: deleted, or
// never there in a project repo made elsewhere.
func (h *Home) Missing(id string) ([]Missing, error) {
	p, err := h.Project(id)
	if err != nil {
		return nil, err
	}
	out := []Missing{}
	for _, f := range scaffold {
		if !exists(filepath.Join(p.Path, f.File)) {
			out = append(out, f)
		}
	}
	return out, nil
}

// Scaffold writes the scaffold files the project's repo lacks and commits
// them, by the CEO's identity. It never overwrites a file.
func (h *Home) Scaffold(ctx context.Context, id string) error {
	p, err := h.Project(id)
	if err != nil {
		return err
	}
	created, err := writeScaffold(id, p.Path)
	if err != nil || len(created) == 0 {
		return err
	}
	return h.git.Commit(ctx, p.Path, "Project repo: add "+strings.Join(created, ", "), created...)
}

var nameRule = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,62}$`)

// nameRules says what validName accepts, for refusals.
const nameRules = "letters, digits, dots, hyphens and underscores, starting with a letter or digit"

// validName is the rule for a repo's name, which becomes a folder: its
// worktree in a sandbox. Looser than a project's: a repo is often an existing
// folder, named as it is.
func validName(name string) bool {
	return nameRule.MatchString(name) && !strings.Contains(name, "..")
}

// readable is a project's id as people read it, as the UI shows it:
// hyphens and underscores as spaces, the first letter capital (my-portfolio
// is "My portfolio").
func readable(id string) string {
	words := strings.TrimSpace(strings.Join(strings.FieldsFunc(id, func(r rune) bool { return r == '-' || r == '_' }), " "))
	if words == "" {
		return ""
	}
	r, size := utf8.DecodeRuneInString(words)
	return string(unicode.ToUpper(r)) + words[size:]
}

// validProjectName is the rule for a new project's name: a slug, since it is
// its workspace's folder and how it reads is made from it (the UI's readable;
// the CEO, 2026-11-02). A project named before keeps its name.
func validProjectName(name string) bool {
	return len(name) <= 63 && slug.MatchString(name)
}

// checkRepoName refuses a name a repo of p can't have: one Foreman couldn't
// create as a folder, the project repo's, or one p has, in any case.
func checkRepoName(p Project, name string) error {
	switch {
	case !validName(name):
		return fmt.Errorf("%w: a repo name is %s", ErrInvalid, nameRules)
	case strings.EqualFold(name, projectRepo):
		return fmt.Errorf("%w: %q is the project repo's name", ErrInvalid, name)
	}
	for _, r := range p.Repos {
		if strings.EqualFold(r.Name, name) {
			return fmt.Errorf("%w: %s has a repo %s already", ErrExists, p.ID, r.Name)
		}
	}
	return nil
}

func (h *Home) createRepo(ctx context.Context, dir, branch string) error {
	if err := os.Mkdir(dir, 0o755); err != nil {
		return err
	}
	readme := "# " + filepath.Base(dir) + "\n"
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte(readme), 0o644); err != nil {
		return err
	}
	if err := h.git.Init(ctx, dir, branch); err != nil {
		return err
	}
	return h.git.Commit(ctx, dir, "First commit", "README.md")
}

// rescan reloads the snapshot after a change on disk, queues the changes,
// and returns the project. Called with h.mu held.
func (h *Home) rescan(project string) (Project, error) {
	old := h.snap
	h.snap = h.scan(old)
	h.publish(diff(old, h.snap))
	p, ok := h.snap.projects[project]
	if !ok {
		return Project{}, ErrNotFound
	}
	return p, nil
}

// RenameRepo renames one of a project's repos, and moves each agent's
// worktree of it to the new name, so stopped deployments keep their work.
// Refused while a team is deployed on the project, and while the repo is
// missing (its worktrees can only be moved by the repo itself).
func (h *Home) RenameRepo(ctx context.Context, project, name, newName string) (Project, error) {
	h.mu.Lock()
	defer h.unlock()
	p, ok := h.snap.projects[project]
	if !ok {
		return Project{}, ErrNotFound
	}
	i := slices.IndexFunc(p.Repos, func(r Repo) bool { return r.Name == name })
	if i < 0 {
		return Project{}, ErrNotFound
	}
	if h.deployed != nil && h.deployed(ctx, project) {
		return Project{}, fmt.Errorf("%w: stop the team on %s before renaming %s", ErrDeployed, project, name)
	}
	if err := checkRepoName(p, newName); err != nil {
		return Project{}, err
	}
	repo := p.Repos[i]
	if repo.Problem != "" {
		return Project{}, fmt.Errorf("%w: %s can't be renamed while it has a problem (%s): locate it first", ErrInvalid, name, repo.Problem)
	}
	type move struct{ from, to string }
	var moved []move
	undo := func() {
		for _, m := range slices.Backward(moved) {
			_ = h.git.MoveWorktree(ctx, repo.Path, m.to, m.from)
		}
	}
	agents, _ := os.ReadDir(p.agents)
	for _, a := range agents {
		from, to := filepath.Join(p.Sandbox(a.Name()), name), filepath.Join(p.Sandbox(a.Name()), newName)
		if !exists(from) {
			continue
		}
		if exists(to) {
			undo()
			return Project{}, fmt.Errorf("%w: %s is in the way", ErrExists, to)
		}
		if err := h.git.MoveWorktree(ctx, repo.Path, from, to); err != nil {
			undo()
			return Project{}, err
		}
		moved = append(moved, move{from, to})
	}
	if err := h.records.RenameRepo(ctx, project, name, newName); err != nil {
		undo()
		return Project{}, h.recordError(err)
	}
	return h.rescan(project)
}

// RemoveProject removes a project: its record and its whole history in the
// database, its session logs and its agents' harness folders. Its workspace
// stays, the CEO's files (the CEO, 2026-10-28): its agents' worktrees go
// only when asked (its code repos forget them; their branches stay), and its
// project repo only when asked (one never pushed has no other copy); an
// empty workspace goes too. Never while a team is deployed on it.
func (h *Home) RemoveProject(ctx context.Context, id string, deleteRepo, deleteWorktrees bool) error {
	h.mu.Lock()
	defer h.unlock()
	p, ok := h.snap.projects[id]
	if !ok {
		return ErrNotFound
	}
	if h.deployed != nil && h.deployed(ctx, id) {
		return fmt.Errorf("%w: stop the team on %s before removing it", ErrDeployed, id)
	}
	if deleteWorktrees {
		if err := os.RemoveAll(p.agents); err != nil {
			return err
		}
		for _, r := range p.Repos {
			if exists(r.Path) {
				if err := h.git.PruneWorktrees(ctx, r.Path); err != nil {
					return err
				}
			}
		}
	}
	// Private to Foreman: session logs are per instance, <agent>@<project>.
	private, _ := filepath.Glob(h.Path(logsDir, "*@"+p.ID))
	if p.harness != "" {
		private = append(private, p.harness)
	}
	for _, dir := range private {
		if err := os.RemoveAll(dir); err != nil {
			return err
		}
	}
	if err := h.records.Remove(ctx, p.ID); err != nil {
		return h.recordError(err)
	}
	if deleteRepo {
		if err := os.RemoveAll(p.Path); err != nil {
			return err
		}
	}
	if p.Workspace != "" {
		_ = os.Remove(p.Workspace) // only when empty
	}
	old := h.snap
	h.snap = h.scan(old)
	h.publish(diff(old, h.snap))
	return nil
}
