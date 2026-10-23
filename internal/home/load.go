package home

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/brickpop/foreman/internal/profiles"
	"github.com/brickpop/foreman/internal/store"
)

// ErrNotFound means no project or team has that id.
var ErrNotFound = errors.New("home: not found")

// Project is a project of this machine: its record (its name, its repos, its
// profile, whether it is archived) and its project repo's project.toml.
type Project struct {
	ID       string        `json:"id"`
	Path     string        `json:"path"`     // its project repo, <home>/projects/<id>
	Profile  string        `json:"profile"`  // the profile its agents run on: its own, else the default
	Archived bool          `json:"archived"` // put away: out of the sidebar's projects and the Dashboard, not deployable
	Config   ProjectConfig `json:"config"`
	Repos    []Repo        `json:"repos"`    // the code repos agents get worktrees of, by name
	Version  string        `json:"version"`  // of project.toml
	Problems []string      `json:"problems"` // what is wrong with project.toml or a repo; Config is then the last good one
	Source   string        `json:"source"`   // project.toml as it is on disk, shown read-only
}

// Repo is one of a project's code repos, anywhere on disk.
type Repo struct {
	Name    string `json:"name"`    // unique in the project
	Path    string `json:"path"`    // absolute
	Main    string `json:"main"`    // the branch agents start from and merges go to
	Problem string `json:"problem"` // why agents can't use it (missing, …); "" when they can
	root    string // its first commit, which Locate looks for
}

// Ready says whether agents can use the repo.
func (r Repo) Ready() bool { return r.Problem == "" }

// Gone says why the project is gone, as found on disk now, or "" while it
// isn't: its project repo's folder deleted, or every code repo's folder
// missing (a repo moved elsewhere counts; one holding another repo doesn't).
// A gone project's deployments stop (instances.Manager.ProjectsChanged).
func (p Project) Gone() string {
	if !exists(p.Path) {
		return "its project repo, " + p.Path + ", is gone"
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
// agents can use, or its project repo ("project").
func (p Project) RepoDir(name string) (string, bool) {
	if name == projectRepo {
		return p.Path, true
	}
	for _, r := range p.Repos {
		if r.Name == name && r.Ready() {
			return r.Path, true
		}
	}
	return "", false
}

// Main is the main branch of the repo named name: the code repo's own, or
// main for the project repo (Foreman makes it on main) and any other name.
func (p Project) Main(name string) string {
	for _, r := range p.Repos {
		if r.Name == name && name != projectRepo {
			return r.Main
		}
	}
	return "main"
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

// Missing is a scaffold file a project repo lacks.
type Missing struct {
	File     string `json:"file"`     // in the project repo, e.g. wiki/wiki.toml
	Why      string `json:"why"`      // what goes wrong without it
	Required bool   `json:"required"` // the wiki can't be read without it
}

// Change says what changed, for the UI to refetch.
type Change struct {
	Kind     string   `json:"object"`             // "projects" (a project or repo appeared, went or changed), "project", "team" or "settings"
	ID       string   `json:"id,omitempty"`       // the project or team, for "project" and "team"
	Version  string   `json:"version,omitempty"`  // the file's new version ("" once it is gone)
	Problems []string `json:"problems,omitempty"` // what is wrong with it now
}

type snapshot struct {
	projects map[string]Project
	teams    map[string]Team
	settings SettingsFile
}

const (
	configDir    = "config" // the global repo: teams and settings
	teamsDir     = "teams"
	settingsFile = "settings.toml"
	projectsDir  = "projects"  // the project repos, one folder per project
	sandboxesDir = "sandboxes" // the agents' worktrees, <project>/<agent>/<repo>
	projectRepo  = "project"   // the project repo's name among the repos, and its folder in a sandbox
	wikiDir      = "wiki"      // the wiki bundle, inside the project repo
	logsDir      = "logs"      // the session logs, one folder per instance (<agent>@<project>)
)

// gitignore keeps the temp files of atomic writes out of the global repo,
// which holds nothing else of this machine's: the home's runtime state,
// project repos and sandboxes are outside it, and so out of reach of a git
// command run in a sandbox folder.
const gitignore = "*.tmp\n"

// Load makes the home the global repo if it isn't yet, then reads every
// config file and checks every project's repos.
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
	if err := h.ensureRepo(ctx); err != nil {
		return fmt.Errorf("global repo: %w", err)
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.snap = h.scan(h.snap)
	return nil
}

// config is a path inside the global repo, <home>/config.
func (h *Home) config(elem ...string) string {
	return h.Path(append([]string{configDir}, elem...)...)
}

// TeamsDir is the team files' folder.
func (h *Home) TeamsDir() string { return h.config(teamsDir) }

// LogsDir is where the session logs are, one folder per instance.
func (h *Home) LogsDir() string { return h.Path(logsDir) }

// Start is the folder repos are added from: where Foreman was started, or the
// CEO's home folder (a real path).
func (h *Home) Start() string { return h.start }

// homeFolder is the CEO's home folder, its real path: repos must be inside it
// (on macOS, what the Podman machine shares).
func homeFolder() (string, error) {
	dir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(dir)
}

func (h *Home) ensureRepo(ctx context.Context) error {
	if _, err := os.Stat(h.config(".git")); err == nil {
		return h.ensureIgnored(ctx)
	}
	if err := os.MkdirAll(h.config(), 0o755); err != nil {
		return err
	}
	if err := h.git.Init(ctx, h.config(), "main"); err != nil {
		return err
	}
	if _, err := os.Stat(h.config(".gitignore")); errors.Is(err, os.ErrNotExist) {
		if err := os.WriteFile(h.config(".gitignore"), []byte(gitignore), 0o644); err != nil {
			return err
		}
	}
	// Teams and settings put there by hand come in with the first commit.
	files := []string{".gitignore"}
	if _, err := os.Stat(h.config(settingsFile)); err == nil {
		files = append(files, settingsFile)
	}
	if _, err := os.Stat(h.config(teamsDir)); err == nil {
		files = append(files, teamsDir)
	}
	return h.git.Commit(ctx, h.config(), "Global repo", files...)
}

// ensureIgnored adds to an existing global repo's .gitignore the lines a
// newer Foreman ignores, and commits it; lines the CEO wrote stay.
func (h *Home) ensureIgnored(ctx context.Context) error {
	data, err := os.ReadFile(h.config(".gitignore"))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	have := strings.Split(string(data), "\n")
	var missing []string
	for _, line := range strings.Split(strings.TrimSpace(gitignore), "\n") {
		if !slices.Contains(have, line) {
			missing = append(missing, line)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	if len(data) > 0 && !strings.HasSuffix(string(data), "\n") {
		data = append(data, '\n')
	}
	data = append(data, strings.Join(missing, "\n")+"\n"...)
	if err := os.WriteFile(h.config(".gitignore"), data, 0o644); err != nil {
		return err
	}
	return h.git.Commit(ctx, h.config(), "Ignore Foreman's runtime files", ".gitignore")
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

	files, _ := filepath.Glob(h.config(teamsDir, "*.toml"))
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
	crossTeamProblems(s.teams)

	s.settings = SettingsFile{Settings: DefaultSettings(), Problems: []string{}}
	if data, err := os.ReadFile(h.config(settingsFile)); err == nil {
		s.settings.Version = Version(data)
		s.settings.Settings, s.settings.Problems = ParseSettings(data)
		s.settings.Problems = normalize(s.settings.Problems)
		if len(s.settings.Problems) > 0 && len(prev.settings.Problems) == 0 && prev.settings.Version != "" {
			s.settings.Settings = prev.settings.Settings
		}
	}
	return s
}

// project is a record with its project.toml read and its repos checked.
func (h *Home) project(ctx context.Context, r store.ProjectRecord) Project {
	p := Project{ID: r.ID, Path: h.Path(projectsDir, r.ID), Profile: r.Profile, Archived: r.Archived, Repos: []Repo{}}
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
		repo := Repo{Name: rec.Name, Path: rec.Path, Main: rec.Main, Problem: h.checkRepo(ctx, rec), root: rec.RootCommit}
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

// crossTeamProblems adds the rules that span team files: agent ids and display
// names are unique across the home.
func crossTeamProblems(teams map[string]Team) {
	ids, names := map[string][]string{}, map[string][]string{}
	for _, tid := range sortedKeys(teams) {
		for aid, soul := range teams[tid].Config.Agents {
			ids[aid] = append(ids[aid], tid)
			if soul.Name != "" {
				names[soul.Name] = append(names[soul.Name], tid)
			}
		}
	}
	for _, tid := range sortedKeys(teams) {
		t := teams[tid]
		for _, aid := range sortedKeys(t.Config.Agents) {
			if others := without(ids[aid], tid); len(others) > 0 {
				t.Problems = append(t.Problems, fmt.Sprintf("agents.%s: the id is also used in team %s", aid, strings.Join(others, ", ")))
			}
			if name := t.Config.Agents[aid].Name; name != "" {
				if others := without(names[name], tid); len(others) > 0 || count(names[name], tid) > 1 {
					t.Problems = append(t.Problems, fmt.Sprintf("agents.%s.name: %q is also used by another agent", aid, name))
				}
			}
		}
		teams[tid] = t
	}
}

func without(list []string, v string) []string {
	return slices.DeleteFunc(slices.Clone(list), func(s string) bool { return s == v })
}

func count(list []string, v string) int {
	n := 0
	for _, s := range list {
		if s == v {
			n++
		}
	}
	return n
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

// WikiDir is where a project's wiki bundle lives: <project repo>/wiki.
func (h *Home) WikiDir(id string) (string, error) {
	p, err := h.Project(id)
	if err != nil {
		return "", err
	}
	return filepath.Join(p.Path, wikiDir), nil
}

// Sandbox is the folder of an agent's worktrees on a project:
// <home>/sandboxes/<project>/<agent>.
func (h *Home) Sandbox(project, agent string) string {
	return h.Path(sandboxesDir, project, agent)
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

// Settings returns the global settings (config/settings.toml).
func (h *Home) Settings() SettingsFile {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.snap.settings
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
		case inOld != inCur || !slices.Equal(o.Repos, c.Repos) || o.Profile != c.Profile || o.Archived != c.Archived:
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
