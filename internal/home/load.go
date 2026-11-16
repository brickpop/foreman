package home

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"

	"github.com/brickpop/foreman/internal/profiles"
	"github.com/brickpop/foreman/internal/store"
)

// ErrNotFound means no project or team has that id.
var ErrNotFound = errors.New("home: not found")

// Project is a project of this machine: its record (its name, its repos, its
// profile, whether it is hidden) and its HQ's project.toml.
type Project struct {
	ID string `json:"id"`
	// Name is how it shows (the CEO's, any words); "" shows it by its id, made
	// readable. The id stays: ids, branches and folders are built from it.
	Name string `json:"name"`
	Path string `json:"path"` // its HQ, <folder>/projects/<id>/hq
	// Files is its folder of files for the agents, <folder>/projects/<id>/files:
	// the CEO's in inbox/, the agents' own in shared/ (FileFolders).
	Files     string        `json:"files"`
	Profile   string        `json:"profile"` // the profile its agents run on: its own, else the default
	Hidden    bool          `json:"hidden"`  // out of the sidebar's projects (in More) and the Dashboard
	Config    ProjectConfig `json:"config"`
	Repos     []Repo        `json:"repos"`    // the code repos agents get worktrees of, by name
	Version   string        `json:"version"`  // of project.toml
	Problems  []string      `json:"problems"` // what is wrong with project.toml or a repo; Config is then the last good one
	Source    string        `json:"source"`   // project.toml as it is on disk, shown read-only
	worktrees string        // its agents' worktrees, one folder each (Sandbox)
	harness   string        // its agents' harness folders, one each (HarnessDir)
}

// Shown is how the project reads: its display name, else its id made readable.
func (p Project) Shown() string { return cmp.Or(p.Name, readable(p.ID)) }

// Repo is one of a project's code repos, anywhere on disk.
type Repo struct {
	Name    string `json:"name"`          // unique in the project
	Path    string `json:"path"`          // absolute
	Default string `json:"defaultBranch"` // agents start from it, merges go to it unless the lead names another branch
	Problem string `json:"problem"`       // why agents can't use it (missing, …); "" when they can
	root    string // its first commit, which Locate looks for
}

// Ready says whether agents can use the repo.
func (r Repo) Ready() bool { return r.Problem == "" }

// Gone says why the project is gone, as found on disk now, or "" while it
// isn't: its HQ's folder deleted, or every code repo's folder
// missing (a repo moved elsewhere counts; one holding another repo doesn't).
// A gone project's deployments stop (instances.Manager.ProjectsChanged).
func (p Project) Gone() string {
	if !exists(p.Path) {
		return "its HQ, " + p.Path + ", is gone"
	}
	if len(p.Repos) == 0 {
		return ""
	}
	for _, r := range p.Repos {
		if exists(r.Path) {
			return ""
		}
	}
	return "all its code repos are missing"
}

// RepoDir is the folder of one of the project's repos, by name: a code repo
// agents can use, or its HQ ("hq").
func (p Project) RepoDir(name string) (string, bool) {
	if name == hqRepo {
		return p.Path, true
	}
	for _, r := range p.Repos {
		if r.Name == name && r.Ready() {
			return r.Path, true
		}
	}
	return "", false
}

// Default is the default branch of the repo named name: the code repo's
// own, or main for the HQ (Foreman makes it on main) and any other
// name.
func (p Project) Default(name string) string {
	if r, ok := p.repo(name); ok {
		return r.Default
	}
	return "main"
}

func (p Project) repo(name string) (Repo, bool) {
	for _, r := range p.Repos {
		if r.Name == name && name != hqRepo {
			return r, true
		}
	}
	return Repo{}, false
}

// Team is a team file.
type Team struct {
	ID       string     `json:"id"` // the file name without .toml
	Config   TeamConfig `json:"config"`
	Version  string     `json:"version"`
	Problems []string   `json:"problems"`
	Source   string     `json:"source"` // the team file as it is on disk, shown read-only
}

// SettingsFile is settings.toml, or the defaults while it doesn't exist
// (version "").
type SettingsFile struct {
	Settings Settings `json:"settings"`
	Version  string   `json:"version"`
	Problems []string `json:"problems"`
}

// Missing is a scaffold file a project's HQ lacks.
type Missing struct {
	File     string `json:"file"`     // in the HQ, e.g. wiki/wiki.toml
	Why      string `json:"why"`      // what goes wrong without it
	Required bool   `json:"required"` // the wiki can't be read without it
}

// Change says what changed, for the UI to refetch.
type Change struct {
	Kind     string   `json:"object"`             // "projects" (a project or repo appeared, went or changed), "project", "team", "settings" or "instructions"
	ID       string   `json:"id,omitempty"`       // the project or team, for "project" and "team"
	Version  string   `json:"version,omitempty"`  // the file's new version ("" once it is gone)
	Problems []string `json:"problems,omitempty"` // what is wrong with it now
}

type snapshot struct {
	projects     map[string]Project
	teams        map[string]Team
	settings     SettingsFile
	instructions Instructions
}

// Instructions is Foreman's HQ's instructions.md: the CEO's standing
// instructions for every agent, added to each session prompt
// (wiki/product/architecture/session-prompt.md).
type Instructions struct {
	Text    string `json:"text"`
	Version string `json:"version"` // "" while the file doesn't exist
}

const (
	teamsDir         = "teams"           // in Foreman's HQ: the team files
	instructionsFile = "instructions.md" // in Foreman's HQ
	settingsFile     = "settings.toml"   // in the home: this machine's settings
	hqRepo           = "hq"              // a project's HQ's name among its repos
	wikiDir          = "wiki"            // the wiki bundle, inside a project's HQ
	logsDir          = "logs"            // the session logs, one folder per instance (<agent>@<project>)
)

// gitignore keeps the temp files of atomic writes out of Foreman's HQ.
const gitignore = "*.tmp\n"

// Load finds Foreman's folder (foreman_folder), makes Foreman's HQ if it
// isn't there yet, then reads every config file and checks every project's
// repos.
func (h *Home) Load(ctx context.Context, d Deps) error {
	h.git, h.records, h.deployed = d.Git, d.Projects, d.Deployed
	allowed, err := homeFolder()
	if err != nil {
		return err
	}
	h.allowed, h.start = allowed, allowed
	if real, err := filepath.EvalSymlinks(d.Start); d.Start != "" && err == nil && inside(real, allowed) {
		h.start = real
	}
	if h.folder, err = h.foremanFolder(); err != nil {
		return err
	}
	if err := h.ensureHQ(ctx); err != nil {
		return fmt.Errorf("making Foreman's HQ at %s: %w", h.HQ(), err)
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.snap = h.scan(h.snap)
	return nil
}

// TeamsDir is the team files' folder.
func (h *Home) TeamsDir() string { return h.HQ(teamsDir) }

// LogsDir is where the session logs are, one folder per instance.
func (h *Home) LogsDir() string { return h.Path(logsDir) }

// Start is where the folder browser opens: where Foreman was started, or the
// CEO's home folder (a real path).
func (h *Home) Start() string { return h.start }

// homeFolder is the CEO's home folder, its real path: Foreman's folder and
// repos must be inside it (on macOS, what the Podman machine shares).
func homeFolder() (string, error) {
	dir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(dir)
}

// ensureHQ makes Foreman's HQ a git repo if it isn't one yet: teams/, an
// empty instructions.md and .gitignore, in a first commit with whatever was
// put there by hand.
func (h *Home) ensureHQ(ctx context.Context) error {
	if topOfRepo(h.HQ()) {
		return nil
	}
	if err := os.MkdirAll(h.HQ(teamsDir), 0o755); err != nil {
		return err
	}
	for file, content := range map[string]string{".gitignore": gitignore, instructionsFile: ""} {
		if !exists(h.HQ(file)) {
			if err := os.WriteFile(h.HQ(file), []byte(content), 0o644); err != nil {
				return err
			}
		}
	}
	if err := h.git.Init(ctx, h.HQ(), "main"); err != nil {
		return err
	}
	files := []string{".gitignore", instructionsFile}
	if teams, _ := filepath.Glob(h.HQ(teamsDir, "*")); len(teams) > 0 {
		files = append(files, teamsDir)
	}
	return h.git.Commit(ctx, h.HQ(), "Foreman's HQ", files...)
}

// scan reads every config file and checks every repo. A file that no longer
// parses keeps the last good config from prev, with its problems; projects
// the database can't list stay as they were.
func (h *Home) scan(prev snapshot) snapshot {
	s := snapshot{projects: map[string]Project{}, teams: map[string]Team{}}
	ctx := context.Background()
	records, err := h.records.List(ctx)
	if err != nil {
		slog.Error("listing the projects", "err", err)
		s.projects = prev.projects
	}
	for _, r := range records {
		p := h.project(ctx, r)
		if old, ok := prev.projects[p.ID]; ok && p.Version != "" && len(p.Problems) > 0 && len(old.Problems) == 0 {
			p.Config = old.Config
		}
		s.projects[p.ID] = p
	}

	files, _ := filepath.Glob(h.HQ(teamsDir, "*.toml"))
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		t := Team{ID: strings.TrimSuffix(filepath.Base(f), ".toml"), Version: Version(data), Source: string(data)}
		t.Config, t.Problems = ParseTeam(t.ID, data)
		t.Problems = normalize(t.Problems)
		if old, ok := prev.teams[t.ID]; ok && len(t.Problems) > 0 && len(old.Problems) == 0 {
			t.Config = old.Config
		}
		s.teams[t.ID] = t
	}

	s.settings = SettingsFile{Settings: DefaultSettings(), Problems: []string{}}
	if data, err := os.ReadFile(h.Path(settingsFile)); err == nil {
		s.settings.Version = Version(data)
		s.settings.Settings, s.settings.Problems = ParseSettings(data)
		s.settings.Problems = normalize(s.settings.Problems)
		if len(s.settings.Problems) > 0 && len(prev.settings.Problems) == 0 && prev.settings.Version != "" {
			s.settings.Settings = prev.settings.Settings
		}
	}
	if data, err := os.ReadFile(h.HQ(instructionsFile)); err == nil {
		s.instructions = Instructions{Text: string(data), Version: Version(data)}
	}
	return s
}

// project is a record with its project.toml read and its repos checked.
func (h *Home) project(ctx context.Context, r store.ProjectRecord) Project {
	p := Project{
		ID: r.ID, Name: r.Name, Profile: r.Profile, Hidden: r.Hidden, Repos: []Repo{},
		Path: filepath.Join(h.projectDir(r.ID), hqDir), Files: filepath.Join(h.projectDir(r.ID), filesDir),
		worktrees: h.worktrees(r.ID), harness: h.Path(harnessDir, r.ID),
	}
	if p.Profile == "" {
		p.Profile = profiles.Default
	}
	data, err := os.ReadFile(filepath.Join(p.Path, "project.toml"))
	if err != nil {
		p.Config, _ = ParseProject(nil)
		p.Problems = []string{fmt.Sprintf("project.toml: not found in %s", p.Path)}
	} else {
		p.Version, p.Source = Version(data), string(data)
		p.Config, p.Problems = ParseProject(data)
	}
	for _, rec := range r.Repos {
		repo := Repo{Name: rec.Name, Path: rec.Path, Default: rec.Default, Problem: h.checkRepo(ctx, rec), root: rec.RootCommit}
		if !repo.Ready() {
			p.Problems = append(p.Problems, fmt.Sprintf("repos.%s: %s", repo.Name, repo.Problem))
		}
		p.Repos = append(p.Repos, repo)
	}
	p.Problems = normalize(p.Problems)
	return p
}

// checkRepo says why agents can't use a repo, or "": its folder moved or went,
// isn't a git repo any more, is now another repo, or left the home folder.
func (h *Home) checkRepo(ctx context.Context, r store.RepoRecord) string {
	real, err := h.repoFolder(r.Path)
	if err != nil {
		return err.Error()
	}
	if _, err := h.git.Resolve(ctx, real, r.RootCommit); err != nil {
		return fmt.Sprintf("%s is another repo now (it hasn't the first commit, %.8s): Locate the right one", r.Path, r.RootCommit)
	}
	return ""
}

// repoFolder checks that path is a repo's top folder agents can use, and
// returns its real path: absolute, a folder with a .git folder (mounted
// read-only at its own path; a .git file points elsewhere), inside the CEO's
// home folder and outside Foreman's home. The error says why not, for the CEO.
func (h *Home) repoFolder(path string) (string, error) {
	if !filepath.IsAbs(path) {
		return "", fmt.Errorf("%s is not an absolute path", path)
	}
	real, err := filepath.EvalSymlinks(path)
	if errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("%s is missing: Locate it where it went, or remove it from the project", path)
	}
	if err != nil {
		return "", err
	}
	if !topOfRepo(real) {
		return "", fmt.Errorf("%s is not a git repo's top folder (with its own .git folder)", path)
	}
	if !inside(real, h.allowed) {
		return "", fmt.Errorf("%s is outside your home folder (%s), where agents can't reach it", path, h.allowed)
	}
	if home, err := filepath.EvalSymlinks(h.root); err == nil && inside(real, home) {
		return "", fmt.Errorf("%s is inside Foreman's home (%s)", path, h.root)
	}
	return real, nil
}

// topOfRepo says whether dir is a git repo's top folder, with its own .git
// folder (not a worktree's .git file).
func topOfRepo(dir string) bool {
	info, err := os.Stat(filepath.Join(dir, ".git"))
	return err == nil && info.IsDir()
}

// inside says whether path is dir or under it (both real paths).
func inside(path, dir string) bool {
	rel, err := filepath.Rel(dir, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// Projects returns every project, by id.
func (h *Home) Projects() []Project {
	h.mu.Lock()
	defer h.mu.Unlock()
	return values(h.snap.projects)
}

// Project returns one project.
func (h *Home) Project(id string) (Project, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	p, ok := h.snap.projects[id]
	if !ok {
		return Project{}, ErrNotFound
	}
	return p, nil
}

// Checked is a project with its repos checked now, for an agent about to
// start: the home doesn't watch them, so one may have moved since the last
// reload. The other changes found are told as usual.
func (h *Home) Checked(id string) (Project, error) {
	h.mu.Lock()
	defer h.unlock()
	return h.rescan(id)
}

// WikiDir is where a project's wiki bundle lives: <its HQ>/wiki.
func (h *Home) WikiDir(id string) (string, error) {
	p, err := h.Project(id)
	if err != nil {
		return "", err
	}
	return filepath.Join(p.Path, wikiDir), nil
}

// Sandbox is the folder of an agent's worktrees on a project
// (Project.Sandbox); "" for no such project.
func (h *Home) Sandbox(project, agent string) string {
	p, err := h.Project(project)
	if err != nil {
		return ""
	}
	return p.Sandbox(agent)
}

// Teams returns every team, by id.
func (h *Home) Teams() []Team {
	h.mu.Lock()
	defer h.mu.Unlock()
	return values(h.snap.teams)
}

// Team returns one team.
func (h *Home) Team(id string) (Team, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	t, ok := h.snap.teams[id]
	if !ok {
		return Team{}, ErrNotFound
	}
	return t, nil
}

// Settings returns this machine's settings (settings.toml in the home).
func (h *Home) Settings() SettingsFile {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.snap.settings
}

// Instructions returns the CEO's instructions for every agent (Foreman's
// HQ's instructions.md).
func (h *Home) Instructions() Instructions {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.snap.instructions
}

func values[V any](m map[string]V) []V {
	list := make([]V, 0, len(m))
	for _, k := range sortedKeys(m) {
		list = append(list, m[k])
	}
	return list
}

// diff lists what changed between two snapshots.
func diff(old, cur snapshot) []Change {
	var changes []Change
	projectsChanged := false
	for _, id := range sortedKeys(union(old.projects, cur.projects)) {
		o, inOld := old.projects[id]
		c, inCur := cur.projects[id]
		switch {
		case inOld != inCur || !reflect.DeepEqual(o.Repos, c.Repos) || o.Profile != c.Profile || o.Hidden != c.Hidden || o.Name != c.Name:
			projectsChanged = true
			if inOld && inCur && o.Version != c.Version {
				changes = append(changes, Change{Kind: "project", ID: id, Version: c.Version, Problems: c.Problems})
			}
		case o.Version != c.Version || !slices.Equal(o.Problems, c.Problems):
			changes = append(changes, Change{Kind: "project", ID: id, Version: c.Version, Problems: c.Problems})
		}
	}
	if projectsChanged {
		changes = append(changes, Change{Kind: "projects"})
	}
	for _, id := range sortedKeys(union(old.teams, cur.teams)) {
		o, inOld := old.teams[id]
		c, inCur := cur.teams[id]
		if inOld != inCur || o.Version != c.Version || !slices.Equal(o.Problems, c.Problems) {
			changes = append(changes, Change{Kind: "team", ID: id, Version: c.Version, Problems: c.Problems})
		}
	}
	if old.settings.Version != cur.settings.Version {
		changes = append(changes, Change{Kind: "settings", Version: cur.settings.Version, Problems: cur.settings.Problems})
	}
	if old.instructions.Version != cur.instructions.Version {
		changes = append(changes, Change{Kind: "instructions", Version: cur.instructions.Version})
	}
	return changes
}

func union[V any](a, b map[string]V) map[string]V {
	m := make(map[string]V, len(a)+len(b))
	for k, v := range a {
		m[k] = v
	}
	for k, v := range b {
		m[k] = v
	}
	return m
}
