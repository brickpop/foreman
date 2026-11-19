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

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"

	"github.com/brickpop/foreman/internal/git"
	"github.com/brickpop/foreman/internal/profiles"
	"github.com/brickpop/foreman/internal/store"
)

// Adding projects and their repos (wiki/product/features/add-project.md). A
// project is a record in projects.toml (records.go), this machine's part of
// it in the database, and its folder in Foreman's folder, with its HQ; code repos are the CEO's, anywhere in their home folder. Foreman creates a
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

// NewProject creates a project with no repos yet: its record, and its HQ at
// <folder>/projects/<name>/hq, new (project.toml and a starter wiki, in one
// first commit) or cloned from clone (a URL or a local path, as the CEO's git
// can reach it). The id is a slug (validProjectName) and must be new, in
// any case. name is the display name, as SetName takes it: kept only when it
// reads otherwise than the id made readable. An HQ already there (a removed
// project's, kept) is taken over as it is. Nothing this call made is left
// behind if a step fails. created is false for a takeover.
func (h *Home) NewProject(ctx context.Context, id, name, clone string) (p Project, created bool, err error) {
	h.mu.Lock()
	defer h.unlock()
	if !validProjectName(id) {
		return Project{}, false, refuse(ErrInvalid, "A project id is lowercase letters, digits and single hyphens, at most 63 characters (my-portfolio).")
	}
	name, err = displayName(id, name)
	if err != nil {
		return Project{}, false, err
	}
	if existing, ok := h.projectNamed(id); ok {
		return Project{}, false, refuse(ErrExists, "Another project has the id %s already: pick another name.", existing)
	}
	shown := cmp.Or(name, readable(id))
	if h.shownAs(id, shown) {
		return Project{}, false, projectShownAs(shown)
	}
	folder := h.projectDir(id)
	dir := filepath.Join(folder, hqDir)
	create := h.createHQ
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
	made, err := makeDir(folder)
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
	if err := create(ctx, id, dir); err != nil {
		drop()
		return Project{}, false, err
	}
	// A row with no record is a project whose record the CEO took out of
	// projects.toml by hand: its history isn't this new project's.
	if err := h.machine.Remove(ctx, id); err != nil && !errors.Is(err, store.ErrNotFound) {
		drop()
		return Project{}, false, err
	}
	if err := h.machine.Create(ctx, id); err != nil {
		drop()
		return Project{}, false, err
	}
	origin := h.git.Origin(ctx, dir)
	p, err = h.changeRecord(ctx, id, "Projects: add "+id, func(r *Record) {
		r.HQ = origin
		r.DisplayName = name
	})
	if err != nil {
		_ = h.machine.Remove(ctx, id)
		drop()
		return Project{}, false, err
	}
	return p, !takeOver, nil
}

// refuse is a refusal the CEO reads as it is, in the API's error and the
// UI: one full sentence, saying what to do when there is something to do
// (wiki/product/architecture/api.md#error-format). kind is ErrInvalid,
// ErrExists or ErrDeployed.
func refuse(kind error, format string, args ...any) error {
	return fmt.Errorf("%w: %s", kind, fmt.Sprintf(format, args...))
}

// sentence is an error's text as people read it, without its kind
// (ErrInvalid, ErrExists, ErrDeployed).
func sentence(err error) string {
	for _, kind := range []error{ErrInvalid, ErrExists, ErrDeployed} {
		if errors.Is(err, kind) {
			return strings.TrimPrefix(err.Error(), kind.Error()+": ")
		}
	}
	return err.Error()
}

// canTakeOver says why the folder at dir can't become a new project's HQ as
// it is: a clone asked for, or not a git repo's top folder of its own.
func canTakeOver(dir, clone string) error {
	info, err := os.Lstat(dir)
	switch {
	case err != nil:
		return err
	case clone != "":
		return fmt.Errorf("%w: %s is there already: create the project without Clone to take it over, or move it away", ErrExists, dir)
	case !info.IsDir() || !topOfRepo(dir):
		return fmt.Errorf("%w: %s is there already and isn't a git repo: move it away, or pick another name", ErrExists, dir)
	}
	return nil
}

// projectNamed finds a project named name in any case. Called with h.mu held.
// shownAs says whether a project other than id shows as shown (its display
// name, else its id made readable), in any case: two projects never read the
// same (the CEO, 2026-11-04).
func (h *Home) shownAs(id, shown string) bool {
	for _, p := range h.snap.projects {
		if p.ID != id && strings.EqualFold(p.Shown(), shown) {
			return true
		}
	}
	return false
}

// projectShownAs is the refusal of a project that would show as another.
func projectShownAs(shown string) error {
	return refuse(ErrExists, "Another project shows as “%s” already: pick another name.", shown)
}

func (h *Home) projectNamed(name string) (string, bool) {
	for id := range h.snap.projects {
		if strings.EqualFold(id, name) {
			return id, true
		}
	}
	return "", false
}

// createHQ makes dir a project's HQ: the scaffold (project.toml and a
// starter wiki), in one first commit.
func (h *Home) createHQ(ctx context.Context, project, dir string) error {
	created, err := writeScaffold(project, dir)
	if err != nil {
		return err
	}
	if err := h.git.Init(ctx, dir, "main"); err != nil {
		return err
	}
	return h.git.Commit(ctx, dir, "Project HQ", created...)
}

// AddRepo adds an existing repo to a project, named name (its id made from
// it, repoName): path is its top folder, which agents can use
// (Home.repoFolder), with a commit, and not in the project already, with def
// as its default branch (usableRepo). A repo may be in several projects.
func (h *Home) AddRepo(ctx context.Context, project, name, path, def string) (Project, error) {
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
	root, def, err := h.usableRepo(ctx, p, "", path, def)
	if err != nil {
		return Project{}, err
	}
	return h.addRepo(ctx, project, id, name, filepath.Clean(path), root, def)
}

// addRepo records a repo checked usable, id and display name checked by
// repoName: where it is on this machine, then the rest in projects.toml.
// Called with h.mu held.
func (h *Home) addRepo(ctx context.Context, project, id, name, path, root, def string) (Project, error) {
	if err := h.machine.SetRepoPath(ctx, project, id, path); err != nil {
		return Project{}, h.recordError(err)
	}
	repo := RepoRecord{DisplayName: name, Remote: h.git.Origin(ctx, path), RootCommit: root, DefaultBranch: def}
	p, err := h.changeRecord(ctx, project, fmt.Sprintf("Projects: add %s to %s", id, project), func(r *Record) {
		if r.Repos == nil {
			r.Repos = map[string]RepoRecord{}
		}
		r.Repos[id] = repo
	})
	if err != nil {
		_ = h.machine.RemoveRepo(ctx, project, id)
	}
	return p, err
}

// NewRepo creates an empty code repo at path, a folder that doesn't exist yet
// in one that does, on main, and adds it to the project, named name.
func (h *Home) NewRepo(ctx context.Context, project, name, path string) (Project, error) {
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
		p, err = h.addRepo(ctx, project, id, name, path, root, def)
	}
	if err != nil {
		_ = os.RemoveAll(path)
		return Project{}, err
	}
	return p, nil
}

// renameWaits refuses to locate the project's repo id (LocateRepo,
// CloneMissingRepo) while its rename made on another machine waits for the
// team here to stop: located, the rename would never be followed (follow).
// Called with h.mu held.
func (h *Home) renameWaits(ctx context.Context, project, id string) error {
	if h.deployed == nil || !h.deployed(ctx, project) {
		return nil
	}
	rows, err := h.machine.List(ctx)
	if err != nil {
		return err
	}
	for _, row := range rows {
		if row.ID != project {
			continue
		}
		if m, ok := h.renamedRow(ctx, h.snap.records.Records[project], row.Repos, id); ok {
			return waitingRename(m.Name)
		}
	}
	return nil
}

// LocateRepo points a project's repo at its new folder: the same repo (it
// holds the repo's first commit), which agents can use. Refused while a rename
// made on another machine waits for the team here to stop.
func (h *Home) LocateRepo(ctx context.Context, project, name, path string) (Project, error) {
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
	want := p.Repos[i].root
	real, err := h.usableFolder(p, name, path)
	if err != nil {
		return Project{}, err
	}
	if _, err := h.git.Resolve(ctx, real, want); err != nil {
		return Project{}, fmt.Errorf("%w: %s is another repo (it hasn't %s's first commit, %.8s)", ErrInvalid, path, name, want)
	}
	if err := h.machine.SetRepoPath(ctx, project, name, filepath.Clean(path)); err != nil {
		return Project{}, h.recordError(err)
	}
	return h.rescan(project)
}

// RemoveRepo takes a repo out of a project; its folder and branches stay.
// Refused while a team is deployed on the project: its agents work in it.
func (h *Home) RemoveRepo(ctx context.Context, project, name string) (Project, error) {
	h.mu.Lock()
	defer h.unlock()
	p, ok := h.snap.projects[project]
	if !ok || !slices.ContainsFunc(p.Repos, func(r Repo) bool { return r.ID == name }) {
		return Project{}, ErrNotFound
	}
	if h.deployed != nil && h.deployed(ctx, project) {
		return Project{}, refuse(ErrDeployed, "Stop the team on this project first to remove a repo.")
	}
	p, err := h.changeRecord(ctx, project, fmt.Sprintf("Projects: remove %s from %s", name, project), func(r *Record) {
		delete(r.Repos, name)
	})
	if err != nil {
		return Project{}, err
	}
	if err := h.machine.RemoveRepo(ctx, project, name); err != nil {
		return Project{}, err
	}
	return p, nil
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
	i := slices.IndexFunc(p.Repos, func(r Repo) bool { return r.ID == name })
	if i < 0 {
		return Project{}, ErrNotFound
	}
	if git.AgentBranch(branch) {
		return Project{}, fmt.Errorf("%w: %s is an agent's branch (wt/*, lead/*): agents write it, so it can't be the default", ErrInvalid, branch)
	}
	if !p.Repos[i].Ready() {
		return Project{}, fmt.Errorf("%w: %s has a problem (%s): locate it first", ErrInvalid, name, p.Repos[i].Problem)
	}
	if !h.git.HasBranch(ctx, p.Repos[i].Path, branch) {
		return Project{}, fmt.Errorf("%w: %s has no branch %q", ErrInvalid, name, branch)
	}
	return h.changeRecord(ctx, project, fmt.Sprintf("Projects: %s's %s defaults to %s", project, name, branch), func(r *Record) {
		repo := r.Repos[name]
		repo.DefaultBranch = branch
		r.Repos[name] = repo
	})
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
	if err := h.machine.SetProfile(ctx, project, profile); err != nil {
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
	verb := map[bool]string{true: "hide", false: "show"}[hidden]
	return h.changeRecord(ctx, project, fmt.Sprintf("Projects: %s %s", verb, project), func(r *Record) { r.Hidden = hidden })
}

// maxDisplayName is the most characters a display name has, a project's or a
// repo's.
const maxDisplayName = 80

// SetName gives a project its display name, any words (displayName), "" to
// show it by its id again. Only how it shows changes, so a team may be deployed.
func (h *Home) SetName(ctx context.Context, project, name string) (Project, error) {
	name, err := displayName(project, name)
	if err != nil {
		return Project{}, err
	}
	h.mu.Lock()
	defer h.unlock()
	if _, ok := h.snap.projects[project]; !ok {
		return Project{}, ErrNotFound
	}
	shown := cmp.Or(name, readable(project))
	if h.shownAs(project, shown) {
		return Project{}, projectShownAs(shown)
	}
	return h.changeRecord(ctx, project, fmt.Sprintf("Projects: name %s %q", project, name), func(r *Record) { r.DisplayName = name })
}

// displayName is the one rule for display names, a project's or a repo's:
// it checks the name the CEO typed for id, one line of at most
// maxDisplayName characters, and returns it trimmed, or "" when it reads as
// the id made readable, so the record keeps no copy of it.
func displayName(id, name string) (string, error) {
	name = strings.TrimSpace(name)
	if utf8.RuneCountInString(name) > maxDisplayName {
		return "", refuse(ErrInvalid, "A name has at most %d characters.", maxDisplayName)
	}
	if strings.ContainsFunc(name, unicode.IsControl) {
		return "", refuse(ErrInvalid, "A name is one line.")
	}
	if name == readable(id) {
		return "", nil
	}
	return name, nil
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
		if other, err := filepath.EvalSymlinks(r.Path); err == nil && other == real && r.ID != name {
			return "", fmt.Errorf("%w: %s is %s's repo %s already", ErrExists, path, p.ID, r.ID)
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

// starter is the scaffold: what a project's HQ holds besides project.toml,
// laid out as in the repo, "{project}" standing for the project's name
// (wiki/product/features/add-project.md#effect). The wiki follows the
// conventions its AGENTS.md describes.
//
//go:embed all:starter
var starter embed.FS

// scaffold is the starter's files in order, each with what goes wrong
// without it; the wiki can't be read without the required ones.
var scaffold = []Missing{
	{File: "project.toml", Why: "Foreman can't start the project's agents without it", Required: false},
	{File: "wiki.toml", Why: "Foreman can't read the wiki or its boards without it", Required: true},
	{File: "index.md", Why: "the wiki has no home page", Required: true},
	{File: "AGENTS.md", Why: "the agents don't know how this project's wiki and tasks work", Required: false},
	{File: ".gitignore", Why: "the wiki's index and editor files can be committed", Required: false},
	{File: "spec/index.md", Why: "the specs have no map", Required: false},
	{File: "backlog/index.md", Why: "the backlog has no entry point", Required: false},
	{File: "technical-debt.md", Why: "technical debt has no page", Required: false},
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

// Missing lists the scaffold files the project's HQ lacks: deleted, or
// never there in an HQ made elsewhere.
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

// Scaffold writes the scaffold files the project's HQ lacks and commits
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
	return h.git.Commit(ctx, p.Path, "HQ: add "+strings.Join(created, ", "), created...)
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

// slug makes an id from any words, the one rule Foreman has for it (the UI's
// slug is its twin; testdata/slugs.json keeps both in step): accents
// dropped, lowercase, each run of anything but a-z and 0-9 one hyphen, at
// most 63 characters. Letters that don't decompose (æ, ø, ß) are dropped,
// not transliterated. "" when no letter or digit is left.
func slug(words string) string {
	words, _, _ = transform.String(transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn))), words)
	var b strings.Builder
	for _, r := range strings.ToLower(words) {
		switch {
		case 'a' <= r && r <= 'z' || '0' <= r && r <= '9':
			b.WriteRune(r)
		case !strings.HasSuffix(b.String(), "-"):
			b.WriteByte('-')
		}
	}
	id := strings.Trim(b.String(), "-")
	if len(id) > 63 {
		id = strings.TrimRight(id[:63], "-")
	}
	return id
}

// validProjectName is the rule for a new project's name: a slug, since it
// names its folders and how it reads is made from it (the UI's readable;
// the CEO, 2026-11-02). A project named before keeps its name.
func validProjectName(name string) bool {
	return len(name) <= 63 && slugRule.MatchString(name)
}

// repoName checks the name the CEO gives a repo of p, self being the repo
// renamed ("" for a new one), and returns the repo's id, made from it (slug),
// and its display name (displayName). Two repos of a project never have the
// same id, nor show the same, in any case: the id check alone misses a repo
// added before ids were made from names (Ledger_API shows as Ledger API).
func repoName(p Project, self, name string) (id, display string, err error) {
	id = slug(name)
	if display, err = displayName(id, name); err != nil {
		return "", "", err
	}
	if err := checkRepoID(p, self, id); err != nil {
		return "", "", err
	}
	shown := cmp.Or(display, readable(id))
	for _, r := range p.Repos {
		if r.ID != self && strings.EqualFold(r.Shown(), shown) {
			return "", "", refuse(ErrExists, "Another repo shows as “%s” already: pick another name.", r.Shown())
		}
	}
	return id, display, nil
}

// checkRepoID refuses an id made from a repo's name that a repo of p can't
// have: none (no letter or digit), the HQ's, or another repo's (not self's),
// in any case.
func checkRepoID(p Project, self, id string) error {
	switch id {
	case "":
		return refuse(ErrInvalid, "A repo's name needs a letter or digit (a–z, 0–9): its id is made from them.")
	case hqRepo:
		return refuse(ErrInvalid, "“HQ” is the project’s HQ: pick another name.")
	}
	for _, r := range p.Repos {
		if r.ID != self && strings.EqualFold(r.ID, id) {
			return refuse(ErrExists, "Another repo has the id %s already: pick another name.", id)
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
	return h.found(project)
}

// found is the project as last scanned. Called with h.mu held.
func (h *Home) found(project string) (Project, error) {
	p, ok := h.snap.projects[project]
	if !ok {
		return Project{}, ErrNotFound
	}
	return p, nil
}

// RenameRepo gives one of a project's repos a new name, and the id made from
// it (repoName). When the id changes, each agent's worktree of it moves and
// open merge requests follow (moveRepo), so stopped deployments keep their
// work; otherwise only its display name changes. Refused while a team is
// deployed on the project, and, when the id changes, while the repo is
// missing (its worktrees can only be moved by the repo itself).
func (h *Home) RenameRepo(ctx context.Context, project, repo, name string) (Project, error) {
	h.mu.Lock()
	defer h.unlock()
	p, ok := h.snap.projects[project]
	if !ok {
		return Project{}, ErrNotFound
	}
	i := slices.IndexFunc(p.Repos, func(r Repo) bool { return r.ID == repo })
	if i < 0 {
		return Project{}, ErrNotFound
	}
	if h.deployed != nil && h.deployed(ctx, project) {
		return Project{}, refuse(ErrDeployed, "Stop the team on this project first to rename a repo.")
	}
	id, name, err := repoName(p, repo, name)
	if err != nil {
		return Project{}, err
	}
	rename := func(r *Record) {
		rec := r.Repos[repo]
		rec.DisplayName = name
		delete(r.Repos, repo)
		r.Repos[id] = rec
	}
	if id == repo {
		return h.changeRecord(ctx, project, fmt.Sprintf("Projects: name %s's %s %q", project, repo, name), rename)
	}
	path := p.Repos[i].Path
	if problem := p.Repos[i].Problem; problem != "" {
		return Project{}, refuse(ErrInvalid, "%s has a problem (%s): locate it first, then rename it.", repo, problem)
	}
	if err := h.moveRepo(ctx, p, path, repo, id, "then rename again."); err != nil {
		return Project{}, err
	}
	renamed, err := h.changeRecord(ctx, project, fmt.Sprintf("Projects: rename %s's %s to %s", project, repo, id), rename)
	if err != nil {
		_ = h.moveRepo(ctx, p, path, id, repo, "then rename again.")
		return Project{}, err
	}
	return renamed, nil
}

// moveRepo moves this machine's part of p's repo at path from id from to id
// to: each agent's worktree of it, then its row in the database, with the
// open merge requests on it. A worktree in the way refuses it, saying what
// to do then. On a failure, what moved goes back. It never
// touches projects.toml: a rename here writes it (RenameRepo), and a rename
// made on another machine brought it (follow). Called with h.mu held.
func (h *Home) moveRepo(ctx context.Context, p Project, path, from, to, then string) error {
	type move struct{ from, to string }
	var moved []move
	// A change of case only is two moves, as on a Mac's filesystem the
	// new name is the old folder.
	mv := func(from, to string) error {
		if !strings.EqualFold(from, to) {
			return h.git.MoveWorktree(ctx, path, from, to)
		}
		if err := h.git.MoveWorktree(ctx, path, from, to+".renaming"); err != nil {
			return err
		}
		return h.git.MoveWorktree(ctx, path, to+".renaming", to)
	}
	undo := func() {
		for _, m := range slices.Backward(moved) {
			_ = mv(m.to, m.from)
		}
	}
	agents, _ := os.ReadDir(p.worktrees)
	for _, a := range agents {
		src, dst := filepath.Join(p.Sandbox(a.Name()), from), filepath.Join(p.Sandbox(a.Name()), to)
		if !exists(src) {
			continue
		}
		if exists(dst) && !strings.EqualFold(src, dst) {
			undo()
			return refuse(ErrExists, "%s is in the way: move it, %s", dst, then)
		}
		if err := mv(src, dst); err != nil {
			undo()
			return err
		}
		moved = append(moved, move{src, dst})
	}
	if err := h.machine.RenameRepo(ctx, p.ID, from, to); err != nil {
		undo()
		return h.recordError(err)
	}
	return nil
}

// RemoveProject removes a project: its record in projects.toml (on every
// machine, then), its whole history in this machine's database, its session logs and its agents' harness folders. Its folder
// stays, the CEO's files (the CEO, 2026-10-28): its HQ goes only when asked
// (one never pushed has no other copy), and so do its agents' worktrees (its
// code repos forget them; their branches stay); its files stay, and a folder
// left empty goes. Never while a team is deployed on it.
func (h *Home) RemoveProject(ctx context.Context, id string, deleteHQ, deleteWorktrees bool) error {
	h.mu.Lock()
	defer h.unlock()
	p, ok := h.snap.projects[id]
	if !ok {
		return ErrNotFound
	}
	if h.deployed != nil && h.deployed(ctx, id) {
		return fmt.Errorf("%w: stop the team on %s before removing it", ErrDeployed, id)
	}
	// Its record first: refused while projects.toml has problems, before
	// anything else goes.
	if err := h.changeRecords(ctx, "Projects: remove "+id, func(records map[string]Record) { delete(records, id) }); err != nil {
		return err
	}
	if err := h.machine.Remove(ctx, p.ID); err != nil && !errors.Is(err, store.ErrNotFound) {
		return err
	}
	delete(h.synced, p.ID) // a project made again under its id is synced afresh
	if deleteWorktrees {
		if err := os.RemoveAll(p.worktrees); err != nil {
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
	for _, dir := range append(private, p.harness) {
		if err := os.RemoveAll(dir); err != nil {
			return err
		}
	}
	if deleteHQ {
		if err := os.RemoveAll(p.Path); err != nil {
			return err
		}
	}
	// Each only when empty.
	for _, f := range FileFolders {
		_ = os.Remove(filepath.Join(p.Files, f))
	}
	for _, dir := range []string{p.worktrees, p.Files, h.projectDir(p.ID)} {
		_ = os.Remove(dir)
	}
	old := h.snap
	h.snap = h.scan(old)
	h.publish(diff(old, h.snap))
	return nil
}
