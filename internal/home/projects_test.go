package home

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
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

// A new project is a record and a project repo at <workspace>/project,
// committed, with no repos yet and the default profile; the workspace and
// its missing parents are made. Names keep their case, and a name is taken in
// any case. Nothing goes in Foreman's home.
func TestNewProject(t *testing.T) {
	h := loaded(t, nil)
	ws := workspace("DevProject")
	p, _, err := h.NewProject(ctx, "DevProject", "", ws)
	if err != nil {
		t.Fatal(err)
	}
	if p.ID != "DevProject" || p.Workspace != ws || p.Path != filepath.Join(ws, "project") || p.Profile != "default" || len(p.Repos) != 0 || len(p.Problems) != 0 {
		t.Errorf("project %+v", p)
	}
	if p.Sandbox("mara") != filepath.Join(ws, "agents", "mara") || p.HarnessDir("mara") != h.Path("harness", "DevProject", "mara") {
		t.Errorf("sandbox %s, harness %s", p.Sandbox("mara"), p.HarnessDir("mara"))
	}
	if want, _ := starterFile("DevProject", "project.toml"); p.Source != want {
		t.Errorf("project.toml %q %+v", p.Source, p.Config)
	}
	if log := gitOut(t, p.Path, "log", "--format=%s"); log != "Project repo" {
		t.Errorf("project repo log %q", log)
	}
	if status := gitOut(t, p.Path, "status", "--porcelain"); status != "" {
		t.Errorf("left uncommitted: %q", status)
	}
	if list := h.Projects(); len(list) != 1 || list[0].ID != "DevProject" {
		t.Errorf("projects %+v", list)
	}
	if exists(h.Path("projects")) || exists(h.Path("sandboxes")) {
		t.Error("the home holds a project's files")
	}
	// A restart finds it: it is a record, not a folder.
	reload(t, h)
	if got, err := h.Project("DevProject"); err != nil || got.Path != p.Path {
		t.Errorf("after a restart: %+v %v", got, err)
	}
	for name, want := range map[string]error{
		"devproject":            ErrExists,
		"../escape":             ErrInvalid,
		"a b":                   ErrInvalid,
		"":                      ErrInvalid,
		".hidden":               ErrInvalid,
		"with/slash":            ErrInvalid,
		strings.Repeat("x", 64): ErrInvalid,
	} {
		if _, _, err := h.NewProject(ctx, name, "", workspace("other")); !errors.Is(err, want) {
			t.Errorf("%q: %v, want %v", name, err, want)
		}
	}
	if got := entries(t, filepath.Dir(ws)); !slices.Equal(got, []string{"DevProject"}) {
		t.Errorf("the projects folder holds %v", got)
	}
}

// A workspace is an absolute path in the CEO's home folder, outside
// Foreman's home, not a file, not overlapping another project's, and not
// inside a code repo; a refused one is left as it was.
func TestTheWorkspaceRules(t *testing.T) {
	h := loaded(t, nil)
	if _, _, err := h.NewProject(ctx, "ledger", "", workspace("ledger")); err != nil {
		t.Fatal(err)
	}
	api := codeRepo(t, "dev/api")
	if _, err := h.AddRepo(ctx, "ledger", "api", api, ""); err != nil {
		t.Fatal(err)
	}
	home := os.Getenv("HOME")
	writeFile(t, filepath.Join(home, "notes.txt"), "")
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(home, "out")); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		name, path string
		want       error
		says       string
	}{
		{"relative", "workspace/relay", ErrInvalid, "not an absolute path"},
		{"outside the home folder", filepath.Join(outside, "relay"), ErrInvalid, "outside your home folder"},
		{"through a symlink out", filepath.Join(home, "out", "relay"), ErrInvalid, "outside your home folder"},
		{"in Foreman's home", h.Path("relay"), ErrInvalid, "inside Foreman's home"},
		{"a file", filepath.Join(home, "notes.txt"), ErrExists, "is a file"},
		{"another project's", workspace("ledger"), ErrExists, "overlaps ledger's workspace"},
		{"inside another project's", filepath.Join(workspace("ledger"), "relay"), ErrExists, "overlaps ledger's workspace"},
		{"holding another project's", filepath.Dir(workspace("ledger")), ErrExists, "overlaps ledger's workspace"},
		{"inside a code repo", filepath.Join(api, "relay"), ErrInvalid, "inside ledger's repo api"},
	} {
		if _, _, err := h.NewProject(ctx, "relay", "", c.path); !errors.Is(err, c.want) || !strings.Contains(err.Error(), c.says) {
			t.Errorf("%s: %v", c.name, err)
		}
	}
	if exists(filepath.Join(outside, "relay")) || exists(h.Path("relay")) || exists(filepath.Join(api, "relay")) {
		t.Error("a refused workspace was made")
	}
	// An existing empty folder is fine, and stays when the project is made.
	empty := filepath.Join(home, "elsewhere", "relay")
	if err := os.MkdirAll(empty, 0o755); err != nil {
		t.Fatal(err)
	}
	if p, _, err := h.NewProject(ctx, "relay", "", empty); err != nil || p.Workspace != empty {
		t.Errorf("an existing folder: %+v %v", p, err)
	}
}

// A project removed with its workspace kept leaves its project repo there; a
// new project in that workspace takes it over as it is, its history and edits
// included. A clone, or a folder there that isn't a git repo of its own,
// is refused, and a refused takeover never deletes the folder.
func TestNewProjectTakesOverAKeptProjectRepo(t *testing.T) {
	h := loaded(t, nil)
	ws := workspace("ledger")
	p, _, err := h.NewProject(ctx, "ledger", "", ws)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(p.Path, "wiki", "notes.md"), "kept\n")
	gitOut(t, p.Path, "add", "-A")
	gitOut(t, p.Path, "commit", "--quiet", "-m", "Notes")
	if _, err := h.SetArchived(ctx, "ledger", true); err != nil {
		t.Fatal(err)
	}
	if err := h.RemoveProject(ctx, "ledger", false, false); err != nil {
		t.Fatal(err)
	}
	if _, _, err := h.NewProject(ctx, "ledger", p.Path, ws); !errors.Is(err, ErrExists) || !strings.Contains(err.Error(), "without Clone") {
		t.Errorf("a clone over it: %v", err)
	}
	// A record the snapshot doesn't know yet makes the record step fail.
	if err := h.records.Create(ctx, "ledger", ws); err != nil {
		t.Fatal(err)
	}
	if _, _, err := h.NewProject(ctx, "ledger", "", ws); !errors.Is(err, ErrExists) {
		t.Errorf("record taken: %v", err)
	}
	if err := h.records.Remove(ctx, "ledger"); err != nil {
		t.Fatal(err)
	}
	if !exists(filepath.Join(p.Path, "wiki", "notes.md")) {
		t.Fatal("a refused takeover deleted the kept project repo")
	}

	// Under another name too: the workspace is what it is found by.
	got, created, err := h.NewProject(ctx, "Books", "", ws)
	if err != nil {
		t.Fatal(err)
	}
	if created || got.Path != p.Path || got.Source != p.Source || len(got.Problems) != 0 {
		t.Errorf("taken over (created %v) %+v", created, got)
	}
	if log := gitOut(t, got.Path, "log", "--format=%s"); log != "Notes\nProject repo" {
		t.Errorf("its history changed: %q", log)
	}

	// A project/ that isn't a repo's top, or a symlink to one elsewhere, is refused.
	loose := workspace("loose")
	writeFile(t, filepath.Join(loose, "project", "project.toml"), "")
	linked := workspace("linked")
	if err := os.MkdirAll(linked, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(codeRepo(t, "dev/elsewhere"), filepath.Join(linked, "project")); err != nil {
		t.Fatal(err)
	}
	for _, ws := range []string{loose, linked} {
		if _, _, err := h.NewProject(ctx, "x", "", ws); !errors.Is(err, ErrExists) || !strings.Contains(err.Error(), "isn't a git repo") {
			t.Errorf("%s: %v", ws, err)
		}
	}
	if !exists(filepath.Join(loose, "project", "project.toml")) || !exists(filepath.Join(linked, "project")) {
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
	h.git = failingGit{real, "Project repo"}
	if _, _, err := h.NewProject(ctx, "ledger", "", workspace("ledger")); err == nil {
		t.Error("new project: no error")
	}
	if exists(filepath.Join(os.Getenv("HOME"), "workspace")) || len(h.Projects()) != 0 {
		t.Errorf("left behind: %v %+v", exists(filepath.Join(os.Getenv("HOME"), "workspace")), h.Projects())
	}
	h.git = real
	if _, _, err := h.NewProject(ctx, "ledger", "", workspace("ledger")); err != nil {
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
		if _, _, err := h.NewProject(ctx, id, "", workspace(id)); err != nil {
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
	if !slices.Equal(p.Repos, []Repo{{Name: "app", Path: app, Main: "main", root: gitOut(t, app, "rev-parse", "HEAD")}, {Name: "sdk", Path: sdk, Main: "main", root: gitOut(t, sdk, "rev-parse", "HEAD")}}) || len(p.Problems) != 0 {
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
		{"the project repo's name", "Project", codeRepo(t, "dev/p"), ErrInvalid, "the project repo's name"},
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
	if _, _, err := h.NewProject(ctx, "ledger", "", workspace("ledger")); err != nil {
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
	p, _, err := h.NewProject(ctx, "ledger", "", workspace("ledger"))
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
	if branch := gitOut(t, path, "branch", "--show-current"); branch != "main" || p.Repos[0].Main != "main" {
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
	if _, _, err := h.NewProject(ctx, "ledger", "", workspace("ledger")); err != nil {
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

// Archiving keeps everything but is refused while a team is deployed;
// unarchiving never is. Each is announced, and survives a restart.
func TestSetArchived(t *testing.T) {
	h := loaded(t, nil)
	if _, _, err := h.NewProject(ctx, "ledger", "", workspace("ledger")); err != nil {
		t.Fatal(err)
	}
	deployed := true
	h.deployed = func(_ context.Context, project string) bool { return deployed && project == "ledger" }
	if _, err := h.SetArchived(ctx, "ledger", true); !errors.Is(err, ErrDeployed) || !strings.Contains(err.Error(), "stop the team on ledger before archiving it") {
		t.Errorf("while deployed: %v", err)
	}
	if p, _ := h.Project("ledger"); p.Archived {
		t.Error("archived although refused")
	}
	deployed = false
	var changes []Change
	h.notify = func(c Change) { changes = append(changes, c) }
	if p, err := h.SetArchived(ctx, "ledger", true); err != nil || !p.Archived {
		t.Errorf("%+v %v", p, err)
	}
	if len(changes) != 1 || changes[0].Kind != "projects" {
		t.Errorf("changes %+v", changes)
	}
	reload(t, h)
	if p, _ := h.Project("ledger"); !p.Archived || !exists(p.Path) {
		t.Errorf("after a restart: %+v", p)
	}
	deployed = true // can't be, once archived; unarchiving is never refused
	if p, err := h.SetArchived(ctx, "ledger", false); err != nil || p.Archived {
		t.Errorf("unarchived: %+v %v", p, err)
	}
	if _, err := h.SetArchived(ctx, "nope", true); !errors.Is(err, ErrNotFound) {
		t.Errorf("no such project: %v", err)
	}
}

// A project repo without the scaffold (made elsewhere, or its files deleted)
// gets only what it lacks, committed on its own.
func TestScaffoldFillsWhatIsMissing(t *testing.T) {
	h := loaded(t, nil)
	p, _, err := h.NewProject(ctx, "old", "", workspace("old"))
	if err != nil {
		t.Fatal(err)
	}
	repo := p.Path
	gitOut(t, repo, "rm", "--quiet", "-r", "wiki", "AGENTS.md", ".gitignore")
	writeFile(t, filepath.Join(repo, "wiki", "spec.md"), "# Spec\n")
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
	}{{"wiki/wiki.toml", true}, {"wiki/index.md", true}, {"AGENTS.md", false}, {".gitignore", false}, {"wiki/product/index.md", false}, {"wiki/backlog/index.md", false}, {"wiki/technical-debt.md", false}} {
		if got[i].File != want.file || got[i].Required != want.required || got[i].Why == "" {
			t.Errorf("missing[%d] = %+v, want %+v", i, got[i], want)
		}
	}
	// The CEO's own draft index stays: never overwritten.
	writeFile(t, filepath.Join(repo, "wiki", "index.md"), "# Mine\n")
	if err := h.Scaffold(ctx, "old"); err != nil {
		t.Fatal(err)
	}
	if got, _ := h.Missing("old"); len(got) != 0 {
		t.Errorf("still missing %v", got)
	}
	if data, _ := os.ReadFile(filepath.Join(repo, "wiki", "index.md")); string(data) != "# Mine\n" {
		t.Errorf("index.md overwritten: %q", data)
	}
	if data, _ := os.ReadFile(filepath.Join(repo, "AGENTS.md")); !strings.Contains(string(data), "type: task") {
		t.Errorf("AGENTS.md = %q", data)
	}
	if log := gitOut(t, repo, "log", "-1", "--format=%s", "--name-only"); log != "Project repo: add wiki/wiki.toml, AGENTS.md, .gitignore, wiki/product/index.md, wiki/backlog/index.md, wiki/technical-debt.md\n\n.gitignore\nAGENTS.md\nwiki/backlog/index.md\nwiki/product/index.md\nwiki/technical-debt.md\nwiki/wiki.toml" {
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
	p, _, err := h.NewProject(ctx, "ledger", "", workspace("ledger"))
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
	if data, _ := os.ReadFile(filepath.Join(repo, "wiki", "index.md")); !strings.Contains(string(data), "# ledger\n") {
		t.Errorf("index.md = %q", data)
	}
	// The wiki's index, secrets and clutter never show as changes to commit.
	for _, f := range []string{"wiki/.wiki/index.db", ".env", "wiki/.DS_Store", "wiki/backlog/task.md.swp", ".obsidian/workspace.json"} {
		writeFile(t, filepath.Join(repo, f), "x")
	}
	writeFile(t, filepath.Join(repo, "wiki", "product", "flow.png"), "x")
	if status := gitOut(t, repo, "status", "--porcelain", "--untracked-files=all"); status != "?? wiki/product/flow.png" {
		t.Errorf("status %q", status)
	}
	bin, err := exec.LookPath("wiki")
	if err != nil {
		t.Skip("the wiki CLI isn't installed")
	}
	if out, err := exec.Command(bin, "--root", filepath.Join(repo, "wiki"), "check").CombinedOutput(); err != nil || !strings.Contains(string(out), "no issues") {
		t.Errorf("wiki check: %v\n%s", err, out)
	}
}

// A project repo can be cloned, from a local path here; a clone that fails
// leaves nothing behind, and a URL can't run a command.
func TestCloneAProjectRepo(t *testing.T) {
	h := loaded(t, nil)
	src, _, err := h.NewProject(ctx, "origin", "", workspace("origin"))
	if err != nil {
		t.Fatal(err)
	}
	p, _, err := h.NewProject(ctx, "copy", src.Path, workspace("copy"))
	if err != nil {
		t.Fatal(err)
	}
	if p.Source != src.Source || len(p.Problems) != 0 || gitOut(t, p.Path, "log", "--format=%s") != "Project repo" {
		t.Errorf("cloned %+v", p)
	}
	marker := filepath.Join(t.TempDir(), "ran")
	for _, source := range []string{filepath.Join(t.TempDir(), "nope"), "--upload-pack=touch " + marker, "ext::sh -c touch% " + marker} {
		if _, _, err := h.NewProject(ctx, "bad", source, workspace("bad")); !errors.Is(err, ErrInvalid) {
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
	if _, _, err := h.NewProject(ctx, "ledger", "", workspace("ledger")); err != nil {
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
	for newName, want := range map[string]error{"WEB": ErrExists, "project": ErrInvalid, "a/b": ErrInvalid} {
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
// worktrees and its project repo go only when asked, its code repos keep
// their branches, and an empty workspace goes.
func TestRemoveProject(t *testing.T) {
	h := loaded(t, nil)
	for _, id := range []string{"ledger", "relay"} {
		if _, _, err := h.NewProject(ctx, id, "", workspace(id)); err != nil {
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
	// Only once archived; and never while deployed (a deploy racing the archive).
	if err := h.RemoveProject(ctx, "ledger", false, true); !errors.Is(err, ErrInvalid) || !strings.Contains(err.Error(), "archive ledger before removing it") {
		t.Errorf("not archived: %v", err)
	}
	for _, id := range []string{"ledger", "relay"} {
		if _, err := h.SetArchived(ctx, id, true); err != nil {
			t.Fatal(err)
		}
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
		t.Error("the project repo went, unasked")
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

	// Its worktrees kept unless asked; its project repo gone when asked, and
	// then the workspace, if nothing else is left in it.
	relay, _ := h.Project("relay")
	tree := sandboxWorktree(t, h, "relay", "kai", "api", api)
	if err := h.RemoveProject(ctx, "relay", true, false); err != nil {
		t.Fatal(err)
	}
	if exists(relay.Path) || !exists(tree) || !strings.Contains(gitOut(t, api, "worktree", "list"), tree) {
		t.Errorf("project repo %v, worktree %v", exists(relay.Path), exists(tree))
	}
	if err := h.RemoveProject(ctx, "relay", true, false); !errors.Is(err, ErrNotFound) {
		t.Errorf("removed twice: %v", err)
	}
	if _, _, err := h.NewProject(ctx, "books", "", workspace("books")); err != nil {
		t.Fatal(err)
	}
	if _, err := h.SetArchived(ctx, "books", true); err != nil {
		t.Fatal(err)
	}
	if err := h.RemoveProject(ctx, "books", true, true); err != nil {
		t.Fatal(err)
	}
	if exists(workspace("books")) {
		t.Error("an empty workspace stayed")
	}
}

// A project is gone when its project repo's folder is, or every code repo's
// folder; one of several missing isn't gone.
func TestGone(t *testing.T) {
	h := loaded(t, nil)
	p, _, _ := h.NewProject(ctx, "ledger", "", workspace("ledger"))
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
	if !strings.Contains(p.Gone(), "its project repo") {
		t.Errorf("project repo deleted: %q", p.Gone())
	}
}

// A repo's main branch is main when it has one; otherwise the CEO picks one
// of its branches (RepoBranches lists them), and can change it later.
func TestARepoHasItsOwnMainBranch(t *testing.T) {
	h := loaded(t, nil)
	if _, _, err := h.NewProject(ctx, "aragon", "", workspace("aragon")); err != nil {
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
	for name, tc := range map[string]struct{ main, says string }{
		"no main, none picked": {"", "has no branch main: pick its main branch"},
		"a branch it hasn't":   {"trunk", `has no branch "trunk"`},
	} {
		if _, err := h.AddRepo(ctx, "aragon", "poc", poc, tc.main); !errors.Is(err, ErrInvalid) || !strings.Contains(err.Error(), tc.says) {
			t.Errorf("%s: %v", name, err)
		}
	}
	if _, err := h.AddRepo(ctx, "aragon", "app", app, ""); err != nil {
		t.Fatal(err)
	}
	p, err := h.AddRepo(ctx, "aragon", "poc", poc, "master")
	if err != nil || p.Repos[0].Main != "main" || p.Repos[1].Main != "master" {
		t.Fatalf("%+v %v", p.Repos, err)
	}

	if _, err := h.SetRepoMain(ctx, "aragon", "poc", "trunk"); !errors.Is(err, ErrInvalid) {
		t.Errorf("a main it hasn't: %v", err)
	}
	if _, err := h.SetRepoMain(ctx, "aragon", "nope", "main"); !errors.Is(err, ErrNotFound) {
		t.Errorf("no such repo: %v", err)
	}
	if p, err = h.SetRepoMain(ctx, "aragon", "poc", "feature"); err != nil || p.Repos[1].Main != "feature" || p.Repos[0].Main != "main" {
		t.Errorf("changed: %+v %v", p.Repos, err)
	}
	// Kept in the record: a reload still has it.
	h.reload()
	if p, _ := h.Project("aragon"); p.Repos[1].Main != "feature" {
		t.Errorf("after a reload: %+v", p.Repos)
	}
}
