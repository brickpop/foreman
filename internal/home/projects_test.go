package home

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/brickpop/foreman/internal/profiles"
)

func entries(t *testing.T, dir string) []string {
	t.Helper()
	es, err := os.ReadDir(dir)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	var names []string
	for _, e := range es {
		names = append(names, e.Name())
	}
	return names
}

// A new project is a record and its HQ at <folder>/projects/<id>/hq,
// committed, with no repos yet and the default profile; its files, its
// agents' worktrees and harness folders follow from its id. A name is a
// slug, and taken in any case (one from before slugs may have capitals).
// Nothing but its harness folders goes in Foreman's home.
func TestNewProject(t *testing.T) {
	h := loaded(t, nil)
	p, _, err := h.NewProject(ctx, "dev-project", "")
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(h.Folder(), "projects", "dev-project")
	if p.ID != "dev-project" || p.Path != filepath.Join(dir, "hq") || p.Files != filepath.Join(dir, "files") || p.Profile != "default" || len(p.Repos) != 0 || len(p.Problems) != 0 {
		t.Errorf("project %+v", p)
	}
	if p.Sandbox("mara") != filepath.Join(h.Folder(), "worktrees", "dev-project", "mara") || p.HarnessDir("mara") != h.Path("harness", "dev-project", "mara") {
		t.Errorf("sandbox %s, harness %s", p.Sandbox("mara"), p.HarnessDir("mara"))
	}
	if want, _ := starterFile("dev-project", "project.toml"); p.Source != want {
		t.Errorf("project.toml %q %+v", p.Source, p.Config)
	}
	if log := gitOut(t, p.Path, "log", "--format=%s"); log != "Project HQ" {
		t.Errorf("HQ log %q", log)
	}
	if status := gitOut(t, p.Path, "status", "--porcelain"); status != "" {
		t.Errorf("left uncommitted: %q", status)
	}
	if list := h.Projects(); len(list) != 1 || list[0].ID != "dev-project" {
		t.Errorf("projects %+v", list)
	}
	if got := entries(t, h.Root()); slices.Contains(got, "projects") || slices.Contains(got, "worktrees") {
		t.Errorf("the home holds a project's files: %v", got)
	}
	// A restart finds it: it is a record, its folder found by its id.
	reload(t, h)
	if got, err := h.Project("dev-project"); err != nil || got.Path != p.Path {
		t.Errorf("after a restart: %+v %v", got, err)
	}
	for name, want := range map[string]error{
		"dev-project": ErrExists,
		// A project's name is its folder's: a slug (the CEO, 2026-11-02).
		"Dev-Project":           ErrInvalid,
		"my_app":                ErrInvalid,
		"app.v2":                ErrInvalid,
		"-lead":                 ErrInvalid,
		"trail-":                ErrInvalid,
		"a--b":                  ErrInvalid,
		"../escape":             ErrInvalid,
		"a b":                   ErrInvalid,
		"":                      ErrInvalid,
		".hidden":               ErrInvalid,
		"with/slash":            ErrInvalid,
		strings.Repeat("x", 64): ErrInvalid,
	} {
		if _, _, err := h.NewProject(ctx, name, ""); !errors.Is(err, want) {
			t.Errorf("%q: %v, want %v", name, err, want)
		}
	}
	if got := entries(t, filepath.Dir(dir)); !slices.Equal(got, []string{"dev-project"}) {
		t.Errorf("projects/ holds %v", got)
	}
}

// A project removed with its HQ kept leaves it there; a new project of that
// name takes it over as it is, its history and edits included. A clone, or
// a folder there that isn't a git repo of its own, is refused, and a refused
// takeover never deletes the folder.
func TestNewProjectTakesOverAKeptHQ(t *testing.T) {
	h := loaded(t, nil)
	p, _, err := h.NewProject(ctx, "ledger", "")
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(p.Path, "notes.md"), "kept\n")
	gitOut(t, p.Path, "add", "-A")
	gitOut(t, p.Path, "commit", "--quiet", "-m", "Notes")
	if err := h.RemoveProject(ctx, "ledger", false, false); err != nil {
		t.Fatal(err)
	}
	if _, _, err := h.NewProject(ctx, "ledger", p.Path); !errors.Is(err, ErrExists) || !strings.Contains(err.Error(), "without Clone") {
		t.Errorf("a clone over it: %v", err)
	}
	if !exists(filepath.Join(p.Path, "notes.md")) {
		t.Fatal("a refused takeover deleted the kept HQ")
	}
	// A row left in the database with no record in projects.toml (the CEO
	// took it out by hand) is another project's history: it goes.
	if err := h.machine.SetProfile(ctx, "ledger", "work"); err != nil {
		t.Fatal(err)
	}
	got, created, err := h.NewProject(ctx, "ledger", "")
	if err != nil {
		t.Fatal(err)
	}
	if created || got.Path != p.Path || got.Source != p.Source || len(got.Problems) != 0 || got.Profile != profiles.Default {
		t.Errorf("taken over (created %v) %+v", created, got)
	}
	if log := gitOut(t, got.Path, "log", "--format=%s"); log != "Notes\nProject HQ" {
		t.Errorf("its history changed: %q", log)
	}

	// An hq/ that isn't a repo's top, or a symlink to one elsewhere, is refused.
	loose := filepath.Join(h.Folder(), "projects", "loose", "hq")
	writeFile(t, filepath.Join(loose, "project.toml"), "")
	linked := filepath.Join(h.Folder(), "projects", "linked", "hq")
	if err := os.MkdirAll(filepath.Dir(linked), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(codeRepo(t, "dev/elsewhere"), linked); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"loose", "linked"} {
		if _, _, err := h.NewProject(ctx, name, ""); !errors.Is(err, ErrExists) || !strings.Contains(err.Error(), "isn't a git repo") {
			t.Errorf("%s: %v", name, err)
		}
	}
	if !exists(filepath.Join(loose, "project.toml")) || !exists(linked) {
		t.Error("a refused takeover removed the folder")
	}
}

// failingGit fails the commit named fail.
type failingGit struct {
	Git
	fail string
}

func (g failingGit) Commit(ctx context.Context, dir, message string, files ...string) error {
	if message == g.fail {
		return errors.New("disk full")
	}
	return g.Git.Commit(ctx, dir, message, files...)
}

// A failed step leaves no folder and no record behind.
func TestAFailedStepLeavesNothingBehind(t *testing.T) {
	h := loaded(t, nil)
	real := h.git
	h.git = failingGit{real, "Project HQ"}
	if _, _, err := h.NewProject(ctx, "ledger", ""); err == nil {
		t.Error("new project: no error")
	}
	if exists(filepath.Join(h.Folder(), "projects", "ledger")) || len(h.Projects()) != 0 {
		t.Errorf("left behind: %v %+v", exists(filepath.Join(h.Folder(), "projects", "ledger")), h.Projects())
	}
	h.git = real
	if _, _, err := h.NewProject(ctx, "ledger", ""); err != nil {
		t.Fatal(err)
	}
	h.git = failingGit{real, "First commit"}
	path := filepath.Join(os.Getenv("HOME"), "dev", "api")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := h.NewRepo(ctx, "ledger", "api", path); err == nil {
		t.Error("new repo: no error")
	}
	if p, _ := h.Project("ledger"); exists(path) || len(p.Repos) != 0 {
		t.Errorf("left behind: %v %+v", exists(path), p.Repos)
	}
}

// A repo is any git repo's top folder in the CEO's home folder, with a commit:
// in several projects at once, but once per project, under a name unique in
// it.
func TestAddRepo(t *testing.T) {
	h := loaded(t, nil)
	for _, id := range []string{"aragon", "app-only"} {
		if _, _, err := h.NewProject(ctx, id, ""); err != nil {
			t.Fatal(err)
		}
	}
	app, sdk := codeRepo(t, "dev/aragon/app"), codeRepo(t, "dev/aragon/sdk")
	if _, err := h.AddRepo(ctx, "aragon", "app", app, ""); err != nil {
		t.Fatal(err)
	}
	p, err := h.AddRepo(ctx, "aragon", "sdk", sdk+"/", "")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(p.Repos, []Repo{{Name: "app", Path: app, Default: "main", root: gitOut(t, app, "rev-parse", "HEAD")}, {Name: "sdk", Path: sdk, Default: "main", root: gitOut(t, sdk, "rev-parse", "HEAD")}}) || len(p.Problems) != 0 {
		t.Errorf("aragon %+v", p)
	}
	if p, err := h.AddRepo(ctx, "app-only", "app", app, ""); err != nil || len(p.Repos) != 1 {
		t.Errorf("one repo in two projects: %+v %v", p, err)
	}

	// What can't be one of its repos.
	link := filepath.Join(os.Getenv("HOME"), "app-link")
	if err := os.Symlink(app, link); err != nil {
		t.Fatal(err)
	}
	empty := filepath.Join(os.Getenv("HOME"), "empty")
	gitOut(t, filepath.Dir(empty), "init", "--quiet", empty)
	worktree := filepath.Join(os.Getenv("HOME"), "wt")
	gitOut(t, app, "worktree", "add", "--quiet", worktree)
	outside := codeRepo(t, filepath.Join(t.TempDir(), "outside"))
	inHome := codeRepo(t, h.Path("sandboxes", "aragon", "mara", "app"))
	plain := filepath.Join(os.Getenv("HOME"), "plain")
	writeFile(t, filepath.Join(plain, "x"), "")
	for _, tc := range []struct {
		name, repo, path string
		want             error
		says             string
	}{
		{"the same name", "app", codeRepo(t, "dev/other"), ErrExists, "aragon has a repo app"},
		{"in another case", "App", codeRepo(t, "dev/other2"), ErrExists, "aragon has a repo app"},
		{"the HQ's name", "HQ", codeRepo(t, "dev/p"), ErrInvalid, "the HQ's name"},
		{"a name that isn't a folder", "a/b", codeRepo(t, "dev/q"), ErrInvalid, "a repo name is"},
		{"the same folder", "app2", app, ErrExists, "aragon's repo app"},
		{"the same folder by a symlink", "app2", link, ErrExists, "aragon's repo app"},
		{"a relative path", "rel", "dev/aragon/app", ErrInvalid, "not an absolute path"},
		{"a missing folder", "gone", filepath.Join(os.Getenv("HOME"), "gone"), ErrInvalid, "is missing"},
		{"not a repo", "plain", plain, ErrInvalid, "not a git repo's top folder"},
		{"a repo's subfolder", "sub", filepath.Join(app, "."+string(filepath.Separator)+"..", "app", ".git"), ErrInvalid, "not a git repo's top folder"},
		{"a linked worktree (.git is a file)", "wt", worktree, ErrInvalid, "not a git repo's top folder"},
		{"no commit yet", "empty", empty, ErrInvalid, "has no commit yet"},
		{"outside the CEO's home folder", "out", outside, ErrInvalid, "outside your home folder"},
		{"inside Foreman's home", "in", inHome, ErrInvalid, "inside Foreman's home"},
	} {
		_, err := h.AddRepo(ctx, "aragon", tc.repo, tc.path, "")
		if !errors.Is(err, tc.want) || !strings.Contains(err.Error(), tc.says) {
			t.Errorf("%s: %v, want %v saying %q", tc.name, err, tc.want, tc.says)
		}
	}
	if p, _ := h.Project("aragon"); len(p.Repos) != 2 {
		t.Errorf("a refusal added a repo: %+v", p.Repos)
	}
	if _, err := h.AddRepo(ctx, "nope", "app", app, ""); !errors.Is(err, ErrNotFound) {
		t.Errorf("no such project: %v", err)
	}
}

// A repo that moves, goes or is replaced is a problem on its project (which
// blocks its agents), until it is located again: only the same repo is
// accepted. Removing one is refused while a team is deployed on the project.
func TestMissingReposAreLocatedOrRemoved(t *testing.T) {
	h := loaded(t, nil)
	if _, _, err := h.NewProject(ctx, "ledger", ""); err != nil {
		t.Fatal(err)
	}
	api := codeRepo(t, "dev/api")
	if _, err := h.AddRepo(ctx, "ledger", "api", api, ""); err != nil {
		t.Fatal(err)
	}
	moved := filepath.Join(os.Getenv("HOME"), "src", "api")
	if err := os.MkdirAll(filepath.Dir(moved), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(api, moved); err != nil {
		t.Fatal(err)
	}
	h.reload()
	p, _ := h.Project("ledger")
	if p.Repos[0].Ready() || !strings.Contains(p.Repos[0].Problem, api+" is missing") || !hasProblem(p.Problems, "repos.api: "+api+" is missing") {
		t.Errorf("a moved repo: %+v", p)
	}

	// Another repo isn't it, even with the same name.
	other := codeRepo(t, "elsewhere/api")
	if _, err := h.LocateRepo(ctx, "ledger", "api", other); !errors.Is(err, ErrInvalid) || !strings.Contains(err.Error(), "is another repo") {
		t.Errorf("locating another repo: %v", err)
	}
	clone := filepath.Join(os.Getenv("HOME"), "clone")
	gitOut(t, os.Getenv("HOME"), "clone", "--quiet", moved, clone)
	for _, where := range []string{clone, moved} { // a clone holds its first commit too
		p, err := h.LocateRepo(ctx, "ledger", "api", where)
		if err != nil || p.Repos[0].Path != where || !p.Repos[0].Ready() || len(p.Problems) != 0 {
			t.Errorf("located at %s: %+v %v", where, p, err)
		}
	}
	if _, err := h.LocateRepo(ctx, "ledger", "web", moved); !errors.Is(err, ErrNotFound) {
		t.Errorf("locating a repo the project hasn't: %v", err)
	}

	// The folder now holds another repo.
	if err := os.RemoveAll(moved); err != nil {
		t.Fatal(err)
	}
	codeRepo(t, moved)
	h.reload()
	if p, _ := h.Project("ledger"); !strings.Contains(p.Repos[0].Problem, "is another repo now") {
		t.Errorf("replaced: %+v", p.Repos)
	}

	deployed := true
	h.deployed = func(_ context.Context, project string) bool { return project == "ledger" && deployed }
	if _, err := h.RemoveRepo(ctx, "ledger", "api"); !errors.Is(err, ErrDeployed) {
		t.Errorf("removing under a deployed team: %v", err)
	}
	deployed = false
	p, err := h.RemoveRepo(ctx, "ledger", "api")
	if err != nil || len(p.Repos) != 0 || len(p.Problems) != 0 {
		t.Errorf("removed: %+v %v", p, err)
	}
	if !exists(moved) {
		t.Error("removing a repo deleted its folder")
	}
	if _, err := h.RemoveRepo(ctx, "ledger", "api"); !errors.Is(err, ErrNotFound) {
		t.Errorf("removing it again: %v", err)
	}
}

// A new repo is made empty, on the project's main branch, at a new folder
// in an existing one inside the folder repos are added from.
func TestNewRepo(t *testing.T) {
	h := loaded(t, nil)
	p, _, err := h.NewProject(ctx, "ledger", "")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(os.Getenv("HOME"), "dev", "ledger-api")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	p, err = h.NewRepo(ctx, "ledger", "api", path)
	if err != nil || len(p.Repos) != 1 || p.Repos[0].Path != path || !p.Repos[0].Ready() {
		t.Fatalf("%+v %v", p, err)
	}
	if branch := gitOut(t, path, "branch", "--show-current"); branch != "main" || p.Repos[0].Default != "main" {
		t.Errorf("branch %q", branch)
	}
	if log := gitOut(t, path, "log", "--format=%s", "--name-only"); log != "First commit\n\nREADME.md" {
		t.Errorf("log %q", log)
	}
	for _, tc := range []struct{ name, path, says string }{
		{"a folder that exists", path, "already exists"},
		{"a relative path", "dev/x", "not an absolute path"},
		{"in a folder that doesn't exist", filepath.Join(os.Getenv("HOME"), "nope", "x"), "a new folder in an existing one"},
		{"outside the CEO's home folder", filepath.Join(t.TempDir(), "x"), "outside your home folder"},
		{"inside Foreman's home", h.Path("x"), "inside Foreman's home"},
	} {
		if _, err := h.NewRepo(ctx, "ledger", "web", tc.path); err == nil || !strings.Contains(err.Error(), tc.says) {
			t.Errorf("%s: %v", tc.name, err)
		}
	}
}

// A project's profile is its own, else the default; a profile is a name,
// never a path.
func TestSetProfile(t *testing.T) {
	h := loaded(t, nil)
	if _, _, err := h.NewProject(ctx, "ledger", ""); err != nil {
		t.Fatal(err)
	}
	var changes []Change
	h.notify = func(c Change) { changes = append(changes, c) }
	if p, err := h.SetProfile(ctx, "ledger", "work"); err != nil || p.Profile != "work" {
		t.Errorf("%+v %v", p, err)
	}
	if len(changes) != 1 || changes[0].Kind != "projects" {
		t.Errorf("changes %+v", changes)
	}
	reload(t, h)
	if p, _ := h.Project("ledger"); p.Profile != "work" {
		t.Errorf("after a restart: %q", p.Profile)
	}
	if p, err := h.SetProfile(ctx, "ledger", "default"); err != nil || p.Profile != "default" {
		t.Errorf("back to the default: %+v %v", p, err)
	}
	for _, bad := range []string{"../../etc", "Work", ""} {
		if _, err := h.SetProfile(ctx, "ledger", bad); !errors.Is(err, ErrInvalid) {
			t.Errorf("%q: %v", bad, err)
		}
	}
	if _, err := h.SetProfile(ctx, "nope", "work"); !errors.Is(err, ErrNotFound) {
		t.Errorf("no such project: %v", err)
	}
}

// Hiding changes only where a project is listed: never refused, a team on it
// or not. Each is announced, and survives a restart.
func TestSetHidden(t *testing.T) {
	h := loaded(t, nil)
	if _, _, err := h.NewProject(ctx, "ledger", ""); err != nil {
		t.Fatal(err)
	}
	h.deployed = func(_ context.Context, project string) bool { return project == "ledger" }
	var changes []Change
	h.notify = func(c Change) { changes = append(changes, c) }
	if p, err := h.SetHidden(ctx, "ledger", true); err != nil || !p.Hidden {
		t.Errorf("hidden with a team on it: %+v %v", p, err)
	}
	if len(changes) != 1 || changes[0].Kind != "projects" {
		t.Errorf("changes %+v", changes)
	}
	reload(t, h)
	if p, _ := h.Project("ledger"); !p.Hidden || !exists(p.Path) {
		t.Errorf("after a restart: %+v", p)
	}
	if p, err := h.SetHidden(ctx, "ledger", false); err != nil || p.Hidden {
		t.Errorf("shown: %+v %v", p, err)
	}
	if _, err := h.SetHidden(ctx, "nope", true); !errors.Is(err, ErrNotFound) {
		t.Errorf("no such project: %v", err)
	}
}

// An HQ without the scaffold (made elsewhere, or its files deleted)
// gets only what it lacks, committed on its own.
func TestScaffoldFillsWhatIsMissing(t *testing.T) {
	h := loaded(t, nil)
	p, _, err := h.NewProject(ctx, "old", "")
	if err != nil {
		t.Fatal(err)
	}
	repo := p.Path
	gitOut(t, repo, "rm", "--quiet", "-r", "wiki.toml", "index.md", "spec", "backlog", "technical-debt.md", "AGENTS.md", ".gitignore")
	writeFile(t, filepath.Join(repo, "spec.md"), "# Spec\n")
	gitOut(t, repo, "add", ".")
	gitOut(t, repo, "commit", "--quiet", "-m", "agents")
	got, err := h.Missing("old")
	if err != nil || len(got) != 7 {
		t.Fatalf("missing %+v %v", got, err)
	}
	// The wiki can't be read without its files; it can without AGENTS.md.
	for i, want := range []struct {
		file     string
		required bool
	}{{"wiki.toml", true}, {"index.md", true}, {"AGENTS.md", false}, {".gitignore", false}, {"spec/index.md", false}, {"backlog/index.md", false}, {"technical-debt.md", false}} {
		if got[i].File != want.file || got[i].Required != want.required || got[i].Why == "" {
			t.Errorf("missing[%d] = %+v, want %+v", i, got[i], want)
		}
	}
	// The CEO's own draft index stays: never overwritten.
	writeFile(t, filepath.Join(repo, "index.md"), "# Mine\n")
	if err := h.Scaffold(ctx, "old"); err != nil {
		t.Fatal(err)
	}
	if got, _ := h.Missing("old"); len(got) != 0 {
		t.Errorf("still missing %v", got)
	}
	if data, _ := os.ReadFile(filepath.Join(repo, "index.md")); string(data) != "# Mine\n" {
		t.Errorf("index.md overwritten: %q", data)
	}
	if data, _ := os.ReadFile(filepath.Join(repo, "AGENTS.md")); !strings.Contains(string(data), "type: task") {
		t.Errorf("AGENTS.md = %q", data)
	}
	if log := gitOut(t, repo, "log", "-1", "--format=%s", "--name-only"); log != "HQ: add wiki.toml, AGENTS.md, .gitignore, spec/index.md, backlog/index.md, technical-debt.md\n\n.gitignore\nAGENTS.md\nbacklog/index.md\nspec/index.md\ntechnical-debt.md\nwiki.toml" {
		t.Errorf("commit %q", log)
	}
	// Nothing missing: nothing written, nothing committed.
	head := gitOut(t, repo, "rev-parse", "HEAD")
	if err := h.Scaffold(ctx, "old"); err != nil || gitOut(t, repo, "rev-parse", "HEAD") != head {
		t.Errorf("a second scaffold: %v", err)
	}
	for _, id := range []string{"nope", "../old"} {
		if _, err := h.Missing(id); !errors.Is(err, ErrNotFound) {
			t.Errorf("Missing(%q): %v", id, err)
		}
		if err := h.Scaffold(ctx, id); !errors.Is(err, ErrNotFound) {
			t.Errorf("Scaffold(%q): %v", id, err)
		}
	}
}

// The scaffold list and the starter folder are one thing: neither has a file
// the other lacks.
func TestScaffoldIsTheStarterFolder(t *testing.T) {
	var files []string
	_ = fs.WalkDir(starter, "starter", func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			files = append(files, strings.TrimPrefix(p, "starter/"))
		}
		return err
	})
	var listed []string
	for _, f := range scaffold {
		listed = append(listed, f.File)
	}
	slices.Sort(files)
	slices.Sort(listed)
	if !slices.Equal(files, listed) {
		t.Errorf("starter folder %v, scaffold %v", files, listed)
	}
}

// A new project's wiki is named after it and passes the engine's check.
func TestNewProjectWikiChecks(t *testing.T) {
	h := loaded(t, nil)
	p, _, err := h.NewProject(ctx, "ledger", "")
	if err != nil {
		t.Fatal(err)
	}
	repo := p.Path
	for _, f := range scaffold {
		data, err := os.ReadFile(filepath.Join(repo, f.File))
		if err != nil || strings.Contains(string(data), "{project}") {
			t.Errorf("%s: %v, unreplaced: %v", f.File, err, strings.Contains(string(data), "{project}"))
		}
	}
	if data, _ := os.ReadFile(filepath.Join(repo, "index.md")); !strings.Contains(string(data), "# ledger\n") {
		t.Errorf("index.md = %q", data)
	}
	// The wiki's index, secrets and clutter never show as changes to commit.
	for _, f := range []string{".wiki/index.db", ".env", ".DS_Store", "backlog/task.md.swp", ".obsidian/workspace.json"} {
		writeFile(t, filepath.Join(repo, f), "x")
	}
	writeFile(t, filepath.Join(repo, "spec", "flow.png"), "x")
	if status := gitOut(t, repo, "status", "--porcelain", "--untracked-files=all"); status != "?? spec/flow.png" {
		t.Errorf("status %q", status)
	}
	bin, err := exec.LookPath("wiki")
	if err != nil {
		t.Skip("the wiki CLI isn't installed")
	}
	if out, err := exec.Command(bin, "--root", repo, "check").CombinedOutput(); err != nil || !strings.Contains(string(out), "no issues") {
		t.Errorf("wiki check: %v\n%s", err, out)
	}
}

// A project's HQ can be cloned, from a local path here; a clone that fails
// leaves nothing behind, and a URL can't run a command.
func TestCloneAnHQ(t *testing.T) {
	h := loaded(t, nil)
	src, _, err := h.NewProject(ctx, "origin", "")
	if err != nil {
		t.Fatal(err)
	}
	p, _, err := h.NewProject(ctx, "copy", src.Path)
	if err != nil {
		t.Fatal(err)
	}
	if p.Source != src.Source || len(p.Problems) != 0 || gitOut(t, p.Path, "log", "--format=%s") != "Project HQ" {
		t.Errorf("cloned %+v", p)
	}
	marker := filepath.Join(t.TempDir(), "ran")
	for _, source := range []string{filepath.Join(t.TempDir(), "nope"), "--upload-pack=touch " + marker, "ext::sh -c touch% " + marker} {
		if _, _, err := h.NewProject(ctx, "bad", source); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: %v", source, err)
		}
		if exists(h.Path("projects", "bad")) {
			t.Errorf("%s: left its folder", source)
		}
		if _, err := h.Project("bad"); !errors.Is(err, ErrNotFound) {
			t.Errorf("%s: left its record", source)
		}
	}
	if exists(marker) {
		t.Error("a URL ran a command")
	}
}

// sandboxWorktree makes the agent's worktree of a project's repo, as a start
// would.
func sandboxWorktree(t *testing.T, h *Home, project, agent, repo, path string) string {
	t.Helper()
	wt := filepath.Join(h.Sandbox(project, agent), repo)
	if err := os.MkdirAll(filepath.Dir(wt), 0o755); err != nil {
		t.Fatal(err)
	}
	gitOut(t, path, "worktree", "add", "--quiet", "-b", "wt/"+agent+"/main", wt, "main")
	return wt
}

// Renaming a repo moves its agents' worktrees with it; refused while a team
// is deployed, while the repo is missing, and onto a taken or bad name.
func TestRenameRepo(t *testing.T) {
	h := loaded(t, nil)
	if _, _, err := h.NewProject(ctx, "ledger", ""); err != nil {
		t.Fatal(err)
	}
	api := codeRepo(t, "dev/api")
	web := codeRepo(t, "dev/web")
	for name, path := range map[string]string{"api": api, "web": web} {
		if _, err := h.AddRepo(ctx, "ledger", name, path, ""); err != nil {
			t.Fatal(err)
		}
	}
	old := sandboxWorktree(t, h, "ledger", "mara", "api", api)
	deployed := true
	h.deployed = func(context.Context, string) bool { return deployed }
	if _, err := h.RenameRepo(ctx, "ledger", "api", "ledger-api"); !errors.Is(err, ErrDeployed) {
		t.Errorf("while deployed: %v", err)
	}
	deployed = false
	for newName, want := range map[string]error{"WEB": ErrExists, "hq": ErrInvalid, "a/b": ErrInvalid} {
		if _, err := h.RenameRepo(ctx, "ledger", "api", newName); !errors.Is(err, want) {
			t.Errorf("onto %q: %v", newName, err)
		}
	}
	if _, err := h.RenameRepo(ctx, "ledger", "nope", "x"); !errors.Is(err, ErrNotFound) {
		t.Errorf("no such repo: %v", err)
	}
	p, err := h.RenameRepo(ctx, "ledger", "api", "ledger-api")
	if err != nil {
		t.Fatal(err)
	}
	if dir, ok := p.RepoDir("ledger-api"); !ok || dir != api {
		t.Errorf("renamed: %+v", p.Repos)
	}
	moved := filepath.Join(h.Sandbox("ledger", "mara"), "ledger-api")
	if exists(old) || gitOut(t, moved, "branch", "--show-current") != "wt/mara/main" {
		t.Errorf("the worktree didn't move")
	}
	// Resolved: git names it by its real path (macOS: /var is /private/var).
	real, err := filepath.EvalSymlinks(moved)
	if err != nil {
		t.Fatal(err)
	}
	if list := gitOut(t, api, "worktree", "list", "--porcelain"); !strings.Contains(list, "worktree "+real) {
		t.Errorf("the repo doesn't know the moved worktree:\n%s", list)
	}
	// A missing repo can't move its worktrees.
	if err := os.Rename(web, web+"-moved"); err != nil {
		t.Fatal(err)
	}
	h.reload()
	if _, err := h.RenameRepo(ctx, "ledger", "web", "site"); !errors.Is(err, ErrInvalid) {
		t.Errorf("a missing repo: %v", err)
	}
}

// Removing a project takes its record, its logs and its harness folders; its
// worktrees and its HQ go only when asked, its code repos keep
// their branches, its files stay, and its emptied folders go.
func TestRemoveProject(t *testing.T) {
	h := loaded(t, nil)
	for _, id := range []string{"ledger", "relay"} {
		if _, _, err := h.NewProject(ctx, id, ""); err != nil {
			t.Fatal(err)
		}
	}
	api := codeRepo(t, "dev/api")
	for _, id := range []string{"ledger", "relay"} {
		if _, err := h.AddRepo(ctx, id, "api", api, ""); err != nil {
			t.Fatal(err)
		}
	}
	ledger, _ := h.Project("ledger")
	sandboxWorktree(t, h, "ledger", "mara", "api", api)
	writeFile(t, filepath.Join(ledger.HarnessDir("mara"), "claude-code", "CLAUDE.md"), "x")
	writeFile(t, h.Path("logs", "mara@ledger", "1.log"), "x")
	writeFile(t, h.Path("logs", "kai@relay", "1.log"), "x")
	// Any project, hidden or not (relay is), but never while deployed.
	if _, err := h.SetHidden(ctx, "relay", true); err != nil {
		t.Fatal(err)
	}
	deployed := true
	h.deployed = func(_ context.Context, project string) bool { return deployed && project == "ledger" }
	if err := h.RemoveProject(ctx, "ledger", false, true); !errors.Is(err, ErrDeployed) {
		t.Errorf("while deployed: %v", err)
	}
	deployed = false
	if err := h.RemoveProject(ctx, "ledger", false, true); err != nil {
		t.Fatal(err)
	}
	if _, err := h.Project("ledger"); !errors.Is(err, ErrNotFound) {
		t.Errorf("still a project: %v", err)
	}
	if exists(ledger.Sandbox("mara")) || exists(h.Path("harness", "ledger")) || exists(h.Path("logs", "mara@ledger")) || !exists(h.Path("logs", "kai@relay")) {
		t.Error("worktrees, harness folders or logs: wrong ones left")
	}
	if !exists(filepath.Join(ledger.Path, "project.toml")) {
		t.Error("the HQ went, unasked")
	}
	if list := gitOut(t, api, "worktree", "list", "--porcelain"); strings.Count(list, "worktree ") != 1 {
		t.Errorf("the repo still has the sandbox's worktree:\n%s", list)
	}
	if gitOut(t, api, "branch", "--list", "wt/mara/main") == "" {
		t.Error("the agent's branch went")
	}
	reload(t, h)
	if _, err := h.Project("ledger"); !errors.Is(err, ErrNotFound) {
		t.Error("back after a restart")
	}

	// Its worktrees kept unless asked; its HQ gone when asked; its files
	// kept, and its folders gone when nothing else is left in them.
	relay, _ := h.Project("relay")
	tree := sandboxWorktree(t, h, "relay", "kai", "api", api)
	if err := relay.MakeFiles(); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(relay.Files, FilesShared, "data.csv"), "x")
	if err := h.RemoveProject(ctx, "relay", true, false); err != nil {
		t.Fatal(err)
	}
	if exists(relay.Path) || !exists(tree) || !strings.Contains(gitOut(t, api, "worktree", "list"), tree) {
		t.Errorf("HQ %v, worktree %v", exists(relay.Path), exists(tree))
	}
	if got := entries(t, relay.Files); !slices.Equal(got, []string{FilesShared}) {
		t.Errorf("files left %v", got)
	}
	if err := h.RemoveProject(ctx, "relay", true, false); !errors.Is(err, ErrNotFound) {
		t.Errorf("removed twice: %v", err)
	}
	if _, _, err := h.NewProject(ctx, "books", ""); err != nil {
		t.Fatal(err)
	}
	if err := h.RemoveProject(ctx, "books", true, true); err != nil {
		t.Fatal(err)
	}
	if exists(filepath.Join(h.Folder(), "projects", "books")) || exists(filepath.Join(h.Folder(), "worktrees", "books")) {
		t.Error("an empty folder stayed")
	}
}

// A project is gone when its HQ's folder is, or every code repo's
// folder; one of several missing isn't gone.
func TestGone(t *testing.T) {
	h := loaded(t, nil)
	p, _, _ := h.NewProject(ctx, "ledger", "")
	if p.Gone() != "" {
		t.Error("a project without repos is gone")
	}
	api, web := codeRepo(t, "dev/api"), codeRepo(t, "dev/web")
	h.AddRepo(ctx, "ledger", "api", api, "")
	p, _ = h.AddRepo(ctx, "ledger", "web", web, "")
	_ = os.RemoveAll(api)
	if p.Gone() != "" {
		t.Error("gone with one repo left")
	}
	_ = os.RemoveAll(web)
	if p.Gone() != "all its code repos are missing" {
		t.Errorf("all repos missing: %q", p.Gone())
	}
	_ = os.RemoveAll(p.Path)
	if !strings.Contains(p.Gone(), "its HQ") {
		t.Errorf("HQ deleted: %q", p.Gone())
	}
}

// A repo's main branch is main when it has one; otherwise the CEO picks one
// of its branches (RepoBranches lists them), and can change it later.
func TestARepoHasItsOwnDefaultBranch(t *testing.T) {
	h := loaded(t, nil)
	if _, _, err := h.NewProject(ctx, "aragon", ""); err != nil {
		t.Fatal(err)
	}
	app := codeRepo(t, "dev/app")
	poc := codeRepo(t, "dev/poc")
	gitOut(t, poc, "branch", "-m", "main", "master")
	gitOut(t, poc, "switch", "--quiet", "-c", "feature")

	b, err := h.RepoBranches(ctx, poc)
	if err != nil || !slices.Equal(b.Branches, []string{"feature", "master"}) || b.Current != "feature" {
		t.Errorf("%+v: %v", b, err)
	}
	// A repo without a commit has no branch yet: an empty list, not null (AddRepo then says why).
	empty := filepath.Join(os.Getenv("HOME"), "dev", "empty")
	if err := os.MkdirAll(empty, 0o755); err != nil {
		t.Fatal(err)
	}
	gitOut(t, empty, "init", "--quiet")
	if b, err := h.RepoBranches(ctx, empty); err != nil || b.Branches == nil || len(b.Branches) != 0 {
		t.Errorf("a repo without a commit: %#v %v", b, err)
	} else if out, _ := json.Marshal(b); !strings.Contains(string(out), `"branches":[]`) {
		t.Errorf("sent as %s", out)
	}
	if _, err := h.RepoBranches(ctx, filepath.Join(t.TempDir(), "x")); !errors.Is(err, ErrInvalid) {
		t.Errorf("branches of no repo: %v", err)
	}
	for name, tc := range map[string]struct{ def, says string }{
		"no main, none picked": {"", "has no branch main: pick its default branch"},
		"a branch it hasn't":   {"trunk", `has no branch "trunk"`},
		"an agent's branch":    {"wt/tove/x", "agents write it"},
	} {
		if _, err := h.AddRepo(ctx, "aragon", "poc", poc, tc.def); !errors.Is(err, ErrInvalid) || !strings.Contains(err.Error(), tc.says) {
			t.Errorf("%s: %v", name, err)
		}
	}
	if _, err := h.AddRepo(ctx, "aragon", "app", app, ""); err != nil {
		t.Fatal(err)
	}
	p, err := h.AddRepo(ctx, "aragon", "poc", poc, "master")
	if err != nil || p.Repos[0].Default != "main" || p.Repos[1].Default != "master" {
		t.Fatalf("%+v %v", p.Repos, err)
	}

	if _, err := h.SetRepoDefault(ctx, "aragon", "poc", "trunk"); !errors.Is(err, ErrInvalid) {
		t.Errorf("a default it hasn't: %v", err)
	}
	if _, err := h.SetRepoDefault(ctx, "aragon", "nope", "main"); !errors.Is(err, ErrNotFound) {
		t.Errorf("no such repo: %v", err)
	}
	// Agents write their own branches: none is ever the default.
	gitOut(t, poc, "branch", "lead/atlas/x")
	if _, err := h.SetRepoDefault(ctx, "aragon", "poc", "lead/atlas/x"); !errors.Is(err, ErrInvalid) {
		t.Errorf("a lead branch as the default: %v", err)
	}
	if p, err = h.SetRepoDefault(ctx, "aragon", "poc", "feature"); err != nil || p.Repos[1].Default != "feature" || p.Repos[0].Default != "main" {
		t.Errorf("changed: %+v %v", p.Repos, err)
	}
	// Kept in the record: a reload still has it.
	h.reload()
	if p, _ := h.Project("aragon"); p.Repos[1].Default != "feature" {
		t.Errorf("after a reload: %+v", p.Repos)
	}
}

// A project's name is only how it shows: any words, while a team runs too,
// kept across a restart; "" shows it by its id again.
func TestSetName(t *testing.T) {
	h := loaded(t, nil)
	if _, _, err := h.NewProject(ctx, "ledger", ""); err != nil {
		t.Fatal(err)
	}
	h.deployed = func(context.Context, string) bool { return true }
	var changes []Change
	h.notify = func(c Change) { changes = append(changes, c) }
	if p, err := h.SetName(ctx, "ledger", "  Ledger · v2 (EUR) "); err != nil || p.Name != "Ledger · v2 (EUR)" || p.ID != "ledger" {
		t.Errorf("%+v %v", p, err)
	}
	if len(changes) != 1 || changes[0].Kind != "projects" {
		t.Errorf("changes %+v", changes)
	}
	reload(t, h)
	if p, _ := h.Project("ledger"); p.Name != "Ledger · v2 (EUR)" || !exists(p.Path) {
		t.Errorf("after a restart: %+v", p)
	}
	for _, bad := range []string{strings.Repeat("é", maxProjectName+1), "two\nlines", "tab\there"} {
		if _, err := h.SetName(ctx, "ledger", bad); !errors.Is(err, ErrInvalid) {
			t.Errorf("%q: %v", bad, err)
		}
	}
	if p, err := h.SetName(ctx, "ledger", strings.Repeat("é", maxProjectName)); err != nil || p.Name == "" {
		t.Errorf("at the limit, in characters not bytes: %v", err)
	}
	if p, err := h.SetName(ctx, "ledger", " "); err != nil || p.Name != "" {
		t.Errorf("cleared: %+v %v", p, err)
	}
	if _, err := h.SetName(ctx, "nope", "x"); !errors.Is(err, ErrNotFound) {
		t.Errorf("no such project: %v", err)
	}
}

// Two projects never read the same, in any case: a display name taken by
// another's (or by another's id made readable) is refused, and so is a new
// project whose id reads as another's name.
func TestProjectNamesAreUnique(t *testing.T) {
	h := loaded(t, nil)
	for _, id := range []string{"ledger", "big-ledger"} {
		if _, _, err := h.NewProject(ctx, id, ""); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := h.SetName(ctx, "big-ledger", "Ledger"); !errors.Is(err, ErrExists) {
		t.Errorf("another's id, made readable: %v", err)
	}
	if _, err := h.SetName(ctx, "big-ledger", "big LEDGER"); err != nil {
		t.Errorf("its own readable id, in another case: %v", err)
	}
	if _, err := h.SetName(ctx, "ledger", "Ledger · EUR"); err != nil {
		t.Fatal(err)
	}
	// Ledger is free now: ledger shows as Ledger · EUR.
	if p, err := h.SetName(ctx, "big-ledger", "Ledger"); err != nil || p.Name != "Ledger" {
		t.Errorf("a name freed: %+v %v", p, err)
	}
	if _, err := h.SetName(ctx, "ledger", ""); !errors.Is(err, ErrExists) || !strings.Contains(err.Error(), "big-ledger shows as Ledger already") {
		t.Errorf("back to its id, which reads as another's name: %v", err)
	}
	if _, _, err := h.NewProject(ctx, "ledger-eur", ""); err != nil {
		t.Errorf("an id reading differently: %v", err)
	}
	if _, err := h.SetName(ctx, "ledger-eur", "ledger · eur"); !errors.Is(err, ErrExists) {
		t.Errorf("another's display name, in another case: %v", err)
	}
	if _, err := h.SetName(ctx, "ledger", "Big ledger"); err != nil {
		t.Errorf("big-ledger's readable id, no longer shown: %v", err)
	}
	if _, _, err := h.NewProject(ctx, "big-ledger-2", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := h.SetName(ctx, "big-ledger", ""); !errors.Is(err, ErrExists) {
		t.Errorf("cleared to an id that reads as ledger's name: %v", err)
	}
	if _, err := h.SetName(ctx, "ledger", "Old ledger"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := h.NewProject(ctx, "old-ledger", ""); !errors.Is(err, ErrExists) || !strings.Contains(err.Error(), "ledger shows as Old ledger already") {
		t.Errorf("a new project reading as another's name: %v", err)
	}
}

// The server reads ids as the UI does (app.test.ts, readable).
func TestReadable(t *testing.T) {
	for id, want := range map[string]string{"my-portfolio": "My portfolio", "api": "Api", "my__data--set_": "My data set", "Ledger.v2": "Ledger.v2", "": ""} {
		if got := readable(id); got != want {
			t.Errorf("%q: %q, want %q", id, got, want)
		}
	}
}
