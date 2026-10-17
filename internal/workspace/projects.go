package workspace

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
)

// Adding projects and code repos (wiki/product/features/add-project.md).
// Foreman never clones: a repo it creates is new and empty, with a first
// commit so its main branch exists (agents' branches start from it).

var (
	// ErrInvalid means a request can't be carried out as asked; the error says why.
	ErrInvalid = errors.New("workspace: invalid")
	// ErrExists means the folder is already there.
	ErrExists = errors.New("workspace: already exists")
)

// Candidates are the workspace's folders that aren't projects yet, with the
// git repos in each.
func (w *Workspace) Candidates() []Candidate {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.candidates()
}

// candidates is Candidates, called with w.mu held.
func (w *Workspace) candidates() []Candidate {
	candidates := []Candidate{}
	entries, _ := os.ReadDir(w.root)
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		if _, ok := w.snap.projects[e.Name()]; ok {
			continue
		}
		if _, err := os.Stat(filepath.Join(w.root, e.Name(), projectRepo, "project.toml")); err == nil {
			continue // a project the snapshot hasn't seen yet
		}
		repos, _ := w.codeRepos(e.Name(), nil, nil)
		candidates = append(candidates, Candidate{Folder: e.Name(), Repos: repos})
	}
	return candidates
}

// starterProject is a new project.toml: the defaults, with the gates as a
// commented-out example so the CEO sees the option. It lists no repos, so
// every git repo in the folder is one, including those added later.
const starterProject = `project = %q
main = "main"
merge_to_main = "ceo"
checks = ""

# Ask the CEO before any of these:
# always_ask_ceo = ["new dependency", "touches shared staging"]

[forge]
kind = "none"
`

// AddProject turns a candidate folder into a project: its project repo with
// project.toml and a starter wiki, in one first commit. With repo, it also
// creates that empty code repo. Nothing is left behind if a step fails.
func (w *Workspace) AddProject(ctx context.Context, folder, repo string) (Project, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !slices.ContainsFunc(w.candidates(), func(c Candidate) bool { return c.Folder == folder }) {
		return Project{}, fmt.Errorf("%w: %q isn't a folder of the workspace that could become a project", ErrInvalid, folder)
	}
	if err := w.makeProject(ctx, folder, repo); err != nil {
		return Project{}, err
	}
	return w.rescan(folder)
}

// NewProject creates the folder name in the workspace and makes it a project,
// as AddProject does. The name follows a repo's rules and must be new, in
// any case.
func (w *Workspace) NewProject(ctx context.Context, name, repo string) (Project, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !validName(name) {
		return Project{}, fmt.Errorf("%w: a project name is %s", ErrInvalid, nameRules)
	}
	if existing, ok := taken(w.root, name); ok {
		return Project{}, fmt.Errorf("%w: %s", ErrExists, existing)
	}
	dir := filepath.Join(w.root, name)
	if err := os.Mkdir(dir, 0o755); err != nil {
		return Project{}, err
	}
	if err := w.makeProject(ctx, name, repo); err != nil {
		_ = os.RemoveAll(dir)
		return Project{}, err
	}
	return w.rescan(name)
}

// makeProject creates folder's project repo and, with repo, its first code
// repo, removing what it made if a step fails. Called with w.mu held.
func (w *Workspace) makeProject(ctx context.Context, folder, repo string) error {
	if repo != "" {
		if err := w.checkRepoName(folder, repo); err != nil {
			return err
		}
	}
	dir := filepath.Join(w.root, folder, projectRepo)
	if err := w.createProjectRepo(ctx, folder, dir); err != nil {
		_ = os.RemoveAll(dir)
		return err
	}
	if repo == "" {
		return nil
	}
	if err := w.createRepo(ctx, filepath.Join(w.root, folder, repo), "main"); err != nil {
		_ = os.RemoveAll(filepath.Join(w.root, folder, repo))
		_ = os.RemoveAll(dir)
		return err
	}
	return nil
}

// createProjectRepo makes dir the project repo: project.toml and the
// scaffold, in one first commit.
func (w *Workspace) createProjectRepo(ctx context.Context, folder, dir string) error {
	if err := makeFile(filepath.Join(dir, "project.toml"), fmt.Sprintf(starterProject, folder)); err != nil {
		return err
	}
	created, err := writeScaffold(folder, dir)
	if err != nil {
		return err
	}
	if err := w.git.Init(ctx, dir, "main"); err != nil {
		return err
	}
	return w.git.Commit(ctx, dir, "Project repo", append([]string{"project.toml"}, created...)...)
}

// starter is the scaffold: what a project repo holds besides project.toml,
// laid out as in the repo, "{project}" standing for the project's name
// (wiki/product/features/add-project.md#effect). The wiki follows the
// conventions its AGENTS.md describes.
//
//go:embed starter
var starter embed.FS

// scaffold is the starter's files in order, each with what goes wrong
// without it; the wiki can't be read without the required ones.
var scaffold = []Missing{
	{wikiDir + "/wiki.toml", "Foreman can't read the wiki or its boards without it", true},
	{wikiDir + "/index.md", "the wiki has no home page", true},
	{"AGENTS.md", "the agents don't know how this project's wiki and tasks work", false},
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

// Missing lists the scaffold files the project's repo lacks: a project made
// before the scaffold had them, or one whose files were deleted.
func (w *Workspace) Missing(id string) ([]Missing, error) {
	if _, err := w.Project(id); err != nil {
		return nil, err
	}
	out := []Missing{}
	for _, f := range scaffold {
		if !exists(filepath.Join(w.root, id, projectRepo, f.File)) {
			out = append(out, f)
		}
	}
	return out, nil
}

// Scaffold writes the scaffold files the project's repo lacks and commits
// them, by the CEO's identity. It never overwrites a file.
func (w *Workspace) Scaffold(ctx context.Context, id string) error {
	if _, err := w.Project(id); err != nil {
		return err
	}
	dir := filepath.Join(w.root, id, projectRepo)
	created, err := writeScaffold(id, dir)
	if err != nil || len(created) == 0 {
		return err
	}
	return w.git.Commit(ctx, dir, "Project repo: add "+strings.Join(created, ", "), created...)
}

// AddRepo creates an empty code repo in a project, on the project's main
// branch, with a first commit (a README) so agents' branches can start from
// it.
func (w *Workspace) AddRepo(ctx context.Context, project, name string) (Project, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	p, ok := w.snap.projects[project]
	if !ok {
		return Project{}, ErrNotFound
	}
	if len(p.Config.Repos) > 0 {
		return Project{}, fmt.Errorf("%w: project.toml lists its repos; create %s there by hand and add it to repos", ErrInvalid, name)
	}
	if err := w.checkRepoName(project, name); err != nil {
		return Project{}, err
	}
	dir := filepath.Join(w.root, project, name)
	if err := w.createRepo(ctx, dir, p.Config.Main); err != nil {
		_ = os.RemoveAll(dir)
		return Project{}, err
	}
	return w.rescan(project)
}

var nameRule = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,62}$`)

// nameRules says what validName accepts, for refusals.
const nameRules = "letters, digits, dots, hyphens and underscores, starting with a letter or digit"

// validName is the rule for the folders Foreman creates: code repos and
// projects.
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

func (w *Workspace) checkRepoName(folder, name string) error {
	switch {
	case !validName(name):
		return fmt.Errorf("%w: a repo name is %s", ErrInvalid, nameRules)
	case strings.EqualFold(name, projectRepo):
		return fmt.Errorf("%w: %q is the project repo's name", ErrInvalid, name)
	}
	if existing, ok := taken(filepath.Join(w.root, folder), name); ok {
		return fmt.Errorf("%w: %s/%s", ErrExists, folder, existing)
	}
	return nil
}

func (w *Workspace) createRepo(ctx context.Context, dir, branch string) error {
	if err := os.Mkdir(dir, 0o755); err != nil {
		return err
	}
	readme := "# " + filepath.Base(dir) + "\n"
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte(readme), 0o644); err != nil {
		return err
	}
	if err := w.git.Init(ctx, dir, branch); err != nil {
		return err
	}
	return w.git.Commit(ctx, dir, "First commit", "README.md")
}

// rescan reloads the snapshot after a change on disk, tells the subscriber,
// and returns the project. Called with w.mu held.
func (w *Workspace) rescan(project string) (Project, error) {
	old := w.snap
	w.snap = w.scan(old)
	w.publish(diff(old, w.snap))
	p, ok := w.snap.projects[project]
	if !ok {
		return Project{}, ErrNotFound
	}
	return p, nil
}
