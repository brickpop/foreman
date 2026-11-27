package home

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
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
	// Here says its HQ is on this machine. One that isn't (a project from
	// another machine's records) is brought here with BringHere.
	Here   bool   `json:"here"`
	Remote string `json:"remote"` // its HQ's origin as projects.toml keeps it, what BringHere clones; "" when it has none
	// Files is its folder of files for the agents, <folder>/projects/<id>/files:
	// the CEO's in inbox/, the agents' own in shared/ (FileFolders).
	Files     string        `json:"files"`
	Profile   string        `json:"profile"` // the profile its agents run on: its own, else the default
	Hidden    bool          `json:"hidden"`  // out of the popover's projects (in More) and the Dashboard
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
	ID      string `json:"id"`            // made from its name when added or renamed; unique in the project, in any case; agents see only this
	Name    string `json:"name"`          // its display name, for people only; "" shows the id readable
	Path    string `json:"path"`          // absolute
	Default string `json:"defaultBranch"` // agents start from it, merges go to it unless the lead names another branch
	Remote  string `json:"remote"`        // where it can be cloned from (its origin when added); "" when unknown
	Problem string `json:"problem"`       // why agents can't use it (missing, …); "" when they can
	root    string // its first commit, which Locate looks for
}

// Shown is how the repo reads to people: its display name, else its id made
// readable.
func (r Repo) Shown() string { return cmp.Or(r.Name, readable(r.ID)) }

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
		if r.ID == name && r.Ready() {
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
		if r.ID == name && name != hqRepo {
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
	File     string `json:"file"`     // in the HQ, e.g. backlog/index.md
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
	records      Records // projects.toml, which the projects are made from
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
	logsDir          = "logs"            // the session logs, one folder per instance (<agent>@<project>)
)

// Foreman's HQ is a wiki at its root, as a project's HQ is
// (wiki/backlog/wiki-at-root.md): what it starts with besides teams/ and
// instructions.md. Its board (outcomes/) comes with the Director.
const (
	// gitignore keeps the temp files of atomic writes, and the wiki's index,
	// out of it.
	gitignore = "*.tmp\n.wiki/\n"
	hqWiki    = `spec = "0.1"

# Not wiki entries: the CEO's instructions to every agent (the team files are TOML)
ignore = ["instructions.md"]
`
	hqIndex = `---
okf_version: "0.1"
---

# Foreman's HQ

What is yours across projects: your teams (` + "`teams/`" + `) and your standing instructions to every agent (` + "`instructions.md`" + `).
`
)

// Load finds Foreman's folder (foreman_folder), makes Foreman's HQ if it
// isn't there yet, then reads every config file and checks every project's
// repos.
func (h *Home) Load(ctx context.Context, d Deps) error {
	h.git, h.machine, h.deployed = d.Git, d.Projects, d.Deployed
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
// empty instructions.md, .gitignore and its wiki's wiki.toml and index.md
// (none overwritten), in a first commit with whatever was put there by hand.
func (h *Home) ensureHQ(ctx context.Context) error {
	// A repo with no commit is one whose making stopped halfway: made again.
	if _, err := h.git.Resolve(ctx, h.HQ(), "HEAD"); err == nil && topOfRepo(h.HQ()) {
		return nil
	}
	if err := os.MkdirAll(h.HQ(teamsDir), 0o755); err != nil {
		return err
	}
	starts := map[string]string{".gitignore": gitignore, instructionsFile: "", "wiki.toml": hqWiki, "index.md": hqIndex}
	for file, content := range starts {
		if !exists(h.HQ(file)) {
			if err := os.WriteFile(h.HQ(file), []byte(content), 0o644); err != nil {
				return err
			}
		}
	}
	if err := h.git.Init(ctx, h.HQ(), "main"); err != nil {
		return err
	}
	files := slices.Sorted(maps.Keys(starts))
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
	s.records = Records{Records: map[string]Record{}, Problems: []string{}}
	if data, err := os.ReadFile(h.HQ(projectsFile)); err == nil {
		s.records.Version = Version(data)
		s.records.Records, s.records.Problems = ParseRecords(data)
		if len(s.records.Problems) > 0 {
			s.records.Records = prev.records.Records
		}
	}
	machine, err := h.machine.List(ctx)
	if err != nil {
		slog.Error("listing the projects", "err", err)
		s.projects = prev.projects
	} else {
		here := map[string]store.ProjectRecord{}
		for _, m := range machine {
			here[strings.ToLower(m.ID)] = m
		}
		for id, r := range s.records.Records {
			p := h.project(ctx, id, r, here[id])
			if old, ok := prev.projects[p.ID]; ok && p.Version != "" && len(p.Problems) > 0 && len(old.Problems) == 0 {
				p.Config = old.Config
			}
			s.projects[p.ID] = p
		}
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

// project is a project's record, with this machine's part of it (here; the
// zero record when it has none yet), its project.toml read and its repos
// checked.
func (h *Home) project(ctx context.Context, id string, r Record, here store.ProjectRecord) Project {
	p := Project{
		ID: id, Name: r.DisplayName, Profile: here.Profile, Hidden: r.Hidden, Remote: r.HQ, Repos: []Repo{},
		Path: filepath.Join(h.projectDir(id), hqDir), Files: filepath.Join(h.projectDir(id), filesDir),
		worktrees: h.worktrees(id), harness: h.Path(harnessDir, id),
	}
	if p.Profile == "" {
		p.Profile = profiles.Default
	}
	p.Here = exists(p.Path)
	data, err := os.ReadFile(filepath.Join(p.Path, "project.toml"))
	if err != nil {
		p.Config, _ = ParseProject(nil)
		p.Problems = []string{fmt.Sprintf("project.toml: not found in %s", p.Path)}
	} else {
		p.Version, p.Source = Version(data), string(data)
		p.Config, p.Problems = ParseProject(data)
	}
	paths := map[string]string{}
	for _, m := range here.Repos {
		paths[strings.ToLower(m.Name)] = m.Path
	}
	followed := h.follow(ctx, p, r, here.Repos, paths)
	for _, id := range sortedKeys(r.Repos) {
		rec := r.Repos[id]
		path := paths[strings.ToLower(id)]
		repo := Repo{ID: id, Name: rec.DisplayName, Path: path, Default: rec.DefaultBranch, Remote: rec.Remote, root: rec.RootCommit}
		repo.Problem = cmp.Or(followed[id], h.checkRepo(ctx, path, rec.RootCommit))
		if !repo.Ready() {
			p.Problems = append(p.Problems, fmt.Sprintf("repos.%s: %s", repo.ID, repo.Problem))
		}
		p.Repos = append(p.Repos, repo)
	}
	p.Problems = normalize(p.Problems)
	return p
}

// follow carries this machine's part of the project's repos renamed on
// another machine over to their new ids (projects.toml travels between
// machines; the database rows and worktrees don't): a repo of the record with
// no row here, and exactly one row here whose id the record no longer has,
// holding the repo's first commit, is moved (moveRepo), and its path added
// to paths; so is a row of the repo's own id in another case only (Ledger
// renamed ledger), the same row. Several renames there are one here, from
// the first id to the last. Not while a team is deployed on the project: its
// agents work in the old worktrees. Why a rename couldn't follow is returned
// by new id, the repo's problem; more than one matching row is left to
// Locate. Called with h.mu held.
func (h *Home) follow(ctx context.Context, p Project, r Record, rows []store.RepoRecord, paths map[string]string) map[string]string {
	out := map[string]string{}
	// moved moves the row m to id, unless a team is deployed here.
	moved := func(id string, m store.RepoRecord) bool {
		if h.deployed != nil && h.deployed(ctx, p.ID) {
			out[id] = sentence(waitingRename(m.Name))
			return false
		}
		if err := h.moveRepo(ctx, p, m.Path, m.Name, id, "and Foreman follows."); err != nil {
			slog.Warn("following a repo renamed on another machine", "project", p.ID, "from", m.Name, "to", id, "err", err)
			out[id] = sentence(err)
			return false
		}
		slog.Info("followed a repo renamed on another machine", "project", p.ID, "from", m.Name, "to", id)
		paths[strings.ToLower(id)] = m.Path
		return true
	}
	rows = slices.Clone(rows)
	for _, id := range sortedKeys(r.Repos) {
		if m, ok := h.renamedRow(ctx, r, rows, id); ok && moved(id, m) {
			rows[slices.IndexFunc(rows, func(row store.RepoRecord) bool { return row.Name == m.Name })].Name = id
		}
	}
	return out
}

// renamedRow is this machine's row of repo id, renamed on another machine
// and not followed here yet: the row of its id in another case only (Ledger
// renamed ledger), or the one row whose id the record no longer has holding
// its first commit. Called with h.mu held.
func (h *Home) renamedRow(ctx context.Context, r Record, rows []store.RepoRecord, id string) (store.RepoRecord, bool) {
	var matches []store.RepoRecord
	for _, m := range rows {
		switch {
		case m.Name == id:
			return store.RepoRecord{}, false // its own row: nothing to follow
		case strings.EqualFold(m.Name, id):
			return m, true
		case !slices.ContainsFunc(sortedKeys(r.Repos), func(other string) bool { return strings.EqualFold(other, m.Name) }) &&
			h.checkRepo(ctx, m.Path, r.Repos[id].RootCommit) == "":
			matches = append(matches, m)
		}
	}
	if len(matches) != 1 {
		return store.RepoRecord{}, false
	}
	return matches[0], true
}

// waitingRename is a rename made on another machine that waits for the team
// here to stop: the repo's problem, and the refusal to locate it meanwhile.
func waitingRename(old string) error {
	return refuse(ErrDeployed, "Renamed from %s on another machine: stop the team here, and Foreman follows.", old)
}

// checkRepo says why agents can't use a repo, or "": it isn't on this
// machine yet, or its folder moved or went, isn't a git repo any more, is
// now another repo, or left the home folder.
func (h *Home) checkRepo(ctx context.Context, path, root string) string {
	if path == "" {
		return "not on this machine yet: Locate it"
	}
	real, err := h.repoFolder(path)
	if err != nil {
		return err.Error()
	}
	if _, err := h.git.Resolve(ctx, real, root); err != nil {
		return fmt.Sprintf("%s is another repo now (it hasn't the first commit, %.8s): Locate the right one", path, root)
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

// RecordsProblems is what is wrong with projects.toml: while it has any,
// the projects stay as they were last read, and nothing changes them.
func (h *Home) RecordsProblems() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return slices.Clone(h.snap.records.Problems)
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

// WikiDir is where a project's wiki bundle lives: its HQ, the wiki at its
// root (wiki/backlog/wiki-at-root.md).
func (h *Home) WikiDir(id string) (string, error) {
	p, err := h.Project(id)
	if err != nil {
		return "", err
	}
	return p.Path, nil
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
	if !slices.Equal(old.records.Problems, cur.records.Problems) {
		projectsChanged = true
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
