package workspace

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// ErrNotFound means no project or team has that id.
var ErrNotFound = errors.New("workspace: not found")

// Project is a workspace subfolder with project/project.toml.
type Project struct {
	ID       string        `json:"id"` // the folder name
	Config   ProjectConfig `json:"config"`
	Repos    []string      `json:"repos"`    // the code repos agents get worktrees of
	Version  string        `json:"version"`  // of project.toml
	Problems []string      `json:"problems"` // what is wrong with project.toml; Config is then the last good one
	Source   string        `json:"source"`   // project.toml as it is on disk, shown read-only
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

// Change says what changed on disk, for the UI to refetch.
type Change struct {
	Kind     string   `json:"object"`             // "projects" (a project or repo appeared or went), "project", "team" or "settings"
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
	teamsDir     = "teams"
	settingsFile = "settings.toml"
	projectRepo  = "project"
	wikiDir      = "wiki" // the wiki bundle, inside the project repo
	sandboxes    = ".sandboxes"
)

// gitignore keeps Foreman's runtime files out of the workspace repo; temp
// files are those of atomic writes.
const gitignore = "state.db*\nlogs/\nsecrets\ncredentials/\naccess-token\n*.tmp\n"

// Load makes .foreman/ the workspace repo if it isn't yet, then reads every
// config file.
func (w *Workspace) Load(ctx context.Context, g Git) error {
	w.git = g
	if err := w.ensureRepo(ctx); err != nil {
		return fmt.Errorf("workspace repo: %w", err)
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	w.snap = w.scan(w.snap)
	return nil
}

func (w *Workspace) ensureRepo(ctx context.Context) error {
	if _, err := os.Stat(w.Path(".git")); err == nil {
		return nil
	}
	if err := w.git.Init(ctx, w.Path(), "main"); err != nil {
		return err
	}
	if _, err := os.Stat(w.Path(".gitignore")); errors.Is(err, os.ErrNotExist) {
		if err := os.WriteFile(w.Path(".gitignore"), []byte(gitignore), 0o644); err != nil {
			return err
		}
	}
	// An existing workspace brings its team files and settings into the first commit.
	files := []string{".gitignore"}
	if _, err := os.Stat(w.Path(settingsFile)); err == nil {
		files = append(files, settingsFile)
	}
	if _, err := os.Stat(w.Path(teamsDir)); err == nil {
		files = append(files, teamsDir)
	}
	return w.git.Commit(ctx, w.Path(), "Workspace repo", files...)
}

// scan reads every config file. A file that no longer parses keeps the last
// good config from prev, with its problems.
func (w *Workspace) scan(prev snapshot) snapshot {
	s := snapshot{projects: map[string]Project{}, teams: map[string]Team{}}
	entries, _ := os.ReadDir(w.root)
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(w.root, e.Name(), projectRepo, "project.toml"))
		if err != nil {
			continue // not a project (yet): a candidate for Add a project
		}
		p := Project{ID: e.Name(), Version: Version(data), Source: string(data)}
		p.Config, p.Problems = ParseProject(p.ID, data)
		if old, ok := prev.projects[p.ID]; ok && len(p.Problems) > 0 && len(old.Problems) == 0 {
			p.Config = old.Config
		}
		p.Repos, p.Problems = w.codeRepos(p.ID, p.Config.Repos, p.Problems)
		p.Problems = normalize(p.Problems)
		s.projects[p.ID] = p
	}

	files, _ := filepath.Glob(w.Path(teamsDir, "*.toml"))
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
	if data, err := os.ReadFile(w.Path(settingsFile)); err == nil {
		s.settings.Version = Version(data)
		s.settings.Settings, s.settings.Problems = ParseSettings(data)
		s.settings.Problems = normalize(s.settings.Problems)
		if len(s.settings.Problems) > 0 && len(prev.settings.Problems) == 0 && prev.settings.Version != "" {
			s.settings.Settings = prev.settings.Settings
		}
	}
	return s
}

// codeRepos are the git repos of a project: the ones project.toml lists, or
// else every git repo in its folder, except the project repo and sandboxes.
func (w *Workspace) codeRepos(project string, listed []string, p []string) ([]string, []string) {
	dir := filepath.Join(w.root, project)
	isRepo := func(name string) bool {
		_, err := os.Stat(filepath.Join(dir, name, ".git"))
		return err == nil
	}
	if len(listed) > 0 {
		repos := []string{}
		for _, r := range listed {
			if isRepo(r) {
				repos = append(repos, r)
			} else {
				p = append(p, fmt.Sprintf("repos: %q is not a git repo in %s", r, project))
			}
		}
		return repos, p
	}
	repos := []string{}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if e.IsDir() && e.Name() != projectRepo && e.Name() != sandboxes && isRepo(e.Name()) {
			repos = append(repos, e.Name())
		}
	}
	return repos, p
}

// crossTeamProblems adds the rules that span team files: agent ids and display
// names are unique across the workspace.
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
func (w *Workspace) Projects() []Project {
	w.mu.Lock()
	defer w.mu.Unlock()
	return values(w.snap.projects)
}

// Project returns one project.
func (w *Workspace) Project(id string) (Project, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	p, ok := w.snap.projects[id]
	if !ok {
		return Project{}, ErrNotFound
	}
	return p, nil
}

// WikiDir is where a project's wiki bundle lives: <project>/project/wiki.
func (w *Workspace) WikiDir(id string) (string, error) {
	if _, err := w.Project(id); err != nil {
		return "", err
	}
	return filepath.Join(w.root, id, projectRepo, wikiDir), nil
}

// Teams returns every team, by id.
func (w *Workspace) Teams() []Team {
	w.mu.Lock()
	defer w.mu.Unlock()
	return values(w.snap.teams)
}

// Team returns one team.
func (w *Workspace) Team(id string) (Team, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	t, ok := w.snap.teams[id]
	if !ok {
		return Team{}, ErrNotFound
	}
	return t, nil
}

// Settings returns the workspace settings.
func (w *Workspace) Settings() SettingsFile {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.snap.settings
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
		case inOld != inCur || !slices.Equal(o.Repos, c.Repos):
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

// Candidate is a workspace folder that isn't a project yet: Add a project
// turns it into one (wiki/product/features/add-project.md).
type Candidate struct {
	Folder string   `json:"folder"`
	Repos  []string `json:"repos"` // the git repos found in it
}
