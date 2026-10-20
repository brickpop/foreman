package home

import (
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

	"github.com/brickpop/foreman/internal/profiles"
	"github.com/brickpop/foreman/internal/store"
)

// Adding projects and their repos (wiki/product/features/add-project.md). A
// project is a record in the database and a project repo under projects/; its
// code repos are the CEO's, anywhere in their home folder. Foreman creates a
// repo only new and empty, with a first commit so its main branch exists
// (agents' branches start from it).

var (
	// ErrInvalid means a request can't be carried out as asked; the error says why.
	ErrInvalid = errors.New("home: invalid")
	// ErrExists means the name or the folder is taken.
	ErrExists = errors.New("home: exists")
	// ErrDeployed means a team is deployed on the project: stop it first.
	ErrDeployed = errors.New("home: a team is deployed on the project")
)

// NewProject creates a project with no repos yet: its record, and its project
// repo, new (project.toml and a starter wiki, in one first commit) or cloned
// from clone (a URL or a local path, as the CEO's git can reach it). The
// name follows a repo's rules and must be new, in any case, unless a
// removed project left its project repo there: that is taken over. Nothing
// is left behind if a step fails.
func (h *Home) NewProject(ctx context.Context, name, clone string) (Project, error) {
	h.mu.Lock()
	defer h.unlock()
	if !validName(name) {
		return Project{}, fmt.Errorf("%w: a project name is %s", ErrInvalid, nameRules)
	}
	if existing, ok := h.projectNamed(name); ok {
		return Project{}, fmt.Errorf("%w: project %s already exists", ErrExists, existing)
	}
	dir := h.Path(projectsDir, name)
	create := h.createProjectRepo
	if clone != "" {
		create = func(ctx context.Context, _, dir string) error {
			if err := h.git.Clone(ctx, clone, dir); err != nil {
				return fmt.Errorf("%w: %s", ErrInvalid, err.Error())
			}
			return nil
		}
	}
	// A project removed with its project repo kept left its folder: a project
	// of that name takes it over, as it is (the CEO, 2026-10-20).
	existing, takeOver := taken(h.Path(projectsDir), name)
	if takeOver {
		if err := canTakeOver(h.Path(projectsDir, existing), name, existing, clone); err != nil {
			return Project{}, err
		}
		create = func(context.Context, string, string) error { return nil }
	}
	// Only a folder this call made goes when a step fails.
	drop := func() {
		if !takeOver {
			_ = os.RemoveAll(dir)
		}
	}
	if err := os.MkdirAll(h.Path(projectsDir), 0o755); err != nil {
		return Project{}, err
	}
	if err := create(ctx, name, dir); err != nil {
		drop()
		return Project{}, err
	}
	if err := h.records.Create(ctx, name); err != nil {
		drop()
		if errors.Is(err, store.ErrExists) {
			return Project{}, fmt.Errorf("%w: project %s already exists", ErrExists, name)
		}
		return Project{}, err
	}
	return h.rescan(name)
}

// canTakeOver says why the folder left at dir (named existing) can't become
// project name's repo: another case of the name, a clone asked for, or not a
// git repo's top folder of its own (a symlink would put the project repo
// outside the home).
func canTakeOver(dir, name, existing, clone string) error {
	info, err := os.Lstat(dir)
	switch {
	case err != nil:
		return err
	case existing != name:
		return fmt.Errorf("%w: %s is there already: name the project %s to take it over, or move it away", ErrExists, dir, existing)
	case clone != "":
		return fmt.Errorf("%w: %s is there already: create the project without Clone to take it over, or move it away", ErrExists, dir)
	case !info.IsDir() || !topOfRepo(dir):
		return fmt.Errorf("%w: %s is there already and isn't a git repo: move it away, or pick another name", ErrExists, dir)
	}
	return nil
}

// projectNamed finds a project named name in any case. Called with h.mu held.
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
// not in the project already. A repo may be in several projects.
func (h *Home) AddRepo(ctx context.Context, project, name, path string) (Project, error) {
	h.mu.Lock()
	defer h.unlock()
	p, ok := h.snap.projects[project]
	if !ok {
		return Project{}, ErrNotFound
	}
	if err := checkRepoName(p, name); err != nil {
		return Project{}, err
	}
	root, err := h.usableRepo(ctx, p, "", path)
	if err != nil {
		return Project{}, err
	}
	if err := h.records.AddRepo(ctx, project, store.RepoRecord{Name: name, Path: filepath.Clean(path), RootCommit: root}); err != nil {
		return Project{}, h.recordError(err)
	}
	return h.rescan(project)
}

// NewRepo creates an empty code repo at path, a folder that doesn't exist yet
// in one that does, on the project's main branch, and adds it to the project.
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
	if !inside(parent, h.start) {
		return Project{}, fmt.Errorf("%w: %s", ErrInvalid, h.outsideStart(path))
	}
	if home, err := filepath.EvalSymlinks(h.root); err == nil && inside(parent, home) {
		return Project{}, fmt.Errorf("%w: %s is inside Foreman's home (%s)", ErrInvalid, path, h.root)
	}
	if err := h.createRepo(ctx, path, p.Config.Main); err != nil {
		_ = os.RemoveAll(path)
		return Project{}, err
	}
	root, err := h.usableRepo(ctx, p, "", path)
	if err == nil {
		err = h.recordError(h.records.AddRepo(ctx, project, store.RepoRecord{Name: name, Path: path, RootCommit: root}))
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

// usableRepo checks a folder to add as a repo of p (Home.usableFolder)
// and returns its root commit: a repo without a commit has no main branch
// for agents to start from.
func (h *Home) usableRepo(ctx context.Context, p Project, name, path string) (string, error) {
	real, err := h.usableFolder(p, name, path)
	if err != nil {
		return "", err
	}
	root, err := h.git.RootCommit(ctx, real)
	if err != nil {
		return "", fmt.Errorf("%w: %s has no commit yet: commit once, then add it", ErrInvalid, path)
	}
	return root, nil
}

// usableFolder is the real path of a folder agents can use as one of p's
// repos, inside the folder repos are added from, not one p has already,
// except the repo named name (Locate).
func (h *Home) usableFolder(p Project, name, path string) (string, error) {
	real, err := h.repoFolder(path)
	if err != nil {
		return "", fmt.Errorf("%w: %s", ErrInvalid, err.Error())
	}
	if !inside(real, h.start) {
		return "", fmt.Errorf("%w: %s", ErrInvalid, h.outsideStart(path))
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

// validName is the rule for names that become folders: a project's (its
// project repo, its sandboxes) and a repo's (its worktree in a sandbox).
func validName(name string) bool {
	return nameRule.MatchString(name) && !strings.Contains(name, "..")
}

// taken finds an entry of dir named name in any case: a Mac's filesystem
// takes them for one, and elsewhere they would be two names hard to tell
// apart.
func taken(dir, name string) (string, bool) {
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.EqualFold(e.Name(), name) {
			return e.Name(), true
		}
	}
	return "", false
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

// outsideStart is the refusal of a path outside the folder repos are added
// from.
func (h *Home) outsideStart(path string) string {
	return fmt.Sprintf("%s is outside %s, where repos are added from (the folder Foreman was started in): start Foreman from a folder that holds it", path, h.start)
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
	agents, _ := os.ReadDir(h.Path(sandboxesDir, p.ID))
	for _, a := range agents {
		from, to := h.Path(sandboxesDir, p.ID, a.Name(), name), h.Path(sandboxesDir, p.ID, a.Name(), newName)
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

// RemoveProject removes a project: its sandboxes (its code repos forget
// those worktrees; their branches stay), its session logs, its record and
// its whole history in the database, and its project repo only when asked
// (one never pushed has no other copy). Refused while a team is deployed on
// it.
func (h *Home) RemoveProject(ctx context.Context, id string, deleteRepo bool) error {
	h.mu.Lock()
	defer h.unlock()
	p, ok := h.snap.projects[id]
	if !ok {
		return ErrNotFound
	}
	if h.deployed != nil && h.deployed(ctx, id) {
		return fmt.Errorf("%w: stop the team on %s before removing it", ErrDeployed, id)
	}
	if err := os.RemoveAll(h.Path(sandboxesDir, p.ID)); err != nil {
		return err
	}
	for _, r := range p.Repos {
		if exists(r.Path) {
			if err := h.git.PruneWorktrees(ctx, r.Path); err != nil {
				return err
			}
		}
	}
	// Session logs are per instance, <agent>@<project>.
	logs, _ := filepath.Glob(h.Path(logsDir, "*@"+p.ID))
	for _, dir := range logs {
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
	old := h.snap
	h.snap = h.scan(old)
	h.publish(diff(old, h.snap))
	return nil
}
