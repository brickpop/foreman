package home

import (
	"context"
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

// A new project is a record and a project repo under projects/, committed,
// with no repos yet and the default profile. Names keep their case, and a
// name is taken in any case.
func TestNewProject(t *testing.T) {
	h := loaded(t, nil)
	p, err := h.NewProject(ctx, "DevProject", "")
	if err != nil {
		t.Fatal(err)
	}
	if p.ID != "DevProject" || p.Path != h.Path("projects", "DevProject") || p.Profile != "default" || len(p.Repos) != 0 || len(p.Problems) != 0 {
		t.Errorf("project %+v", p)
	}
	if want, _ := starterFile("DevProject", "project.toml"); p.Source != want || p.Config.Main != "main" {
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
	// A restart finds it: it is a record, not a folder.
	reload(t, h)
	if _, err := h.Project("DevProject"); err != nil {
		t.Errorf("after a restart: %v", err)
	}

	// A project removed with its project repo kept leaves its folder: the name
	// is taken until it goes.
	writeFile(t, h.Path("projects", "Kept", "project.toml"), "")
	for name, want := range map[string]error{
		"devproject":            ErrExists,
		"kept":                  ErrExists,
		"../escape":             ErrInvalid,
		"a b":                   ErrInvalid,
		"":                      ErrInvalid,
		".hidden":               ErrInvalid,
		"with/slash":            ErrInvalid,
		strings.Repeat("x", 64): ErrInvalid,
	} {
		if _, err := h.NewProject(ctx, name, ""); !errors.Is(err, want) {
			t.Errorf("%q: %v, want %v", name, err, want)
		}
	}
	if got := entries(t, h.Path("projects")); !slices.Equal(got, []string{"DevProject", "Kept"}) {
		t.Errorf("projects/ holds %v", got)
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
	if _, err := h.NewProject(ctx, "ledger", ""); err == nil {
		t.Error("new project: no error")
	}
	if got := entries(t, h.Path("projects")); len(got) != 0 || len(h.Projects()) != 0 {
		t.Errorf("left behind: %v %+v", got, h.Projects())
	}
	h.git = real
	if _, err := h.NewProject(ctx, "ledger", ""); err != nil {
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
		if _, err := h.NewProject(ctx, id, ""); err != nil {
			t.Fatal(err)
		}
	}
	app, sdk := codeRepo(t, "dev/aragon/app"), codeRepo(t, "dev/aragon/sdk")
	if _, err := h.AddRepo(ctx, "aragon", "app", app); err != nil {
		t.Fatal(err)
	}
	p, err := h.AddRepo(ctx, "aragon", "sdk", sdk+"/")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(p.Repos, []Repo{{Name: "app", Path: app, root: gitOut(t, app, "rev-parse", "HEAD")}, {Name: "sdk", Path: sdk, root: gitOut(t, sdk, "rev-parse", "HEAD")}}) || len(p.Problems) != 0 {
		t.Errorf("aragon %+v", p)
	}
	if p, err := h.AddRepo(ctx, "app-only", "app", app); err != nil || len(p.Repos) != 1 {
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
		_, err := h.AddRepo(ctx, "aragon", tc.repo, tc.path)
		if !errors.Is(err, tc.want) || !strings.Contains(err.Error(), tc.says) {
			t.Errorf("%s: %v, want %v saying %q", tc.name, err, tc.want, tc.says)
		}
	}
	if p, _ := h.Project("aragon"); len(p.Repos) != 2 {
		t.Errorf("a refusal added a repo: %+v", p.Repos)
	}
	if _, err := h.AddRepo(ctx, "nope", "app", app); !errors.Is(err, ErrNotFound) {
		t.Errorf("no such project: %v", err)
	}
}

// A repo that moves, goes or is replaced is a problem on its project (which
// blocks its agents), until it is located again: only the same repo is
// accepted. Removing one is refused while a team is deployed on the project.
func TestMissingReposAreLocatedOrRemoved(t *testing.T) {
	h := loaded(t, nil)
	if _, err := h.NewProject(ctx, "ledger", ""); err != nil {
		t.Fatal(err)
	}
	api := codeRepo(t, "dev/api")
	if _, err := h.AddRepo(ctx, "ledger", "api", api); err != nil {
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
	p, err := h.NewProject(ctx, "ledger", "")
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(p.Path, "project.toml"), "main = \"trunk\"\n")
	h.reload()
	path := filepath.Join(os.Getenv("HOME"), "dev", "ledger-api")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	p, err = h.NewRepo(ctx, "ledger", "api", path)
	if err != nil || len(p.Repos) != 1 || p.Repos[0].Path != path || !p.Repos[0].Ready() {
		t.Fatalf("%+v %v", p, err)
	}
	if branch := gitOut(t, path, "branch", "--show-current"); branch != "trunk" {
		t.Errorf("branch %q", branch)
	}
	if log := gitOut(t, path, "log", "--format=%s", "--name-only"); log != "First commit\n\nREADME.md" {
		t.Errorf("log %q", log)
	}
	for _, tc := range []struct{ name, path, says string }{
		{"a folder that exists", path, "already exists"},
		{"a relative path", "dev/x", "not an absolute path"},
		{"in a folder that doesn't exist", filepath.Join(os.Getenv("HOME"), "nope", "x"), "a new folder in an existing one"},
		{"outside the CEO's home folder", filepath.Join(t.TempDir(), "x"), "where repos are added from"},
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
	if _, err := h.NewProject(ctx, "ledger", ""); err != nil {
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

// A project repo without the scaffold (made elsewhere, or its files deleted)
// gets only what it lacks, committed on its own.
func TestScaffoldFillsWhatIsMissing(t *testing.T) {
	h := loaded(t, nil)
	p, err := h.NewProject(ctx, "old", "")
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
	p, err := h.NewProject(ctx, "ledger", "")
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
	src, err := h.NewProject(ctx, "origin", "")
	if err != nil {
		t.Fatal(err)
	}
	p, err := h.NewProject(ctx, "copy", src.Path)
	if err != nil {
		t.Fatal(err)
	}
	if p.Source != src.Source || len(p.Problems) != 0 || gitOut(t, p.Path, "log", "--format=%s") != "Project repo" {
		t.Errorf("cloned %+v", p)
	}
	marker := filepath.Join(t.TempDir(), "ran")
	for _, source := range []string{filepath.Join(t.TempDir(), "nope"), "--upload-pack=touch " + marker, "ext::sh -c touch% " + marker} {
		if _, err := h.NewProject(ctx, "bad", source); !errors.Is(err, ErrInvalid) {
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
	wt := h.Path("sandboxes", project, agent, repo)
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
	if _, err := h.NewProject(ctx, "ledger", ""); err != nil {
		t.Fatal(err)
	}
	api := codeRepo(t, "dev/api")
	web := codeRepo(t, "dev/web")
	for name, path := range map[string]string{"api": api, "web": web} {
		if _, err := h.AddRepo(ctx, "ledger", name, path); err != nil {
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
	moved := h.Path("sandboxes", "ledger", "mara", "ledger-api")
	if exists(old) || gitOut(t, moved, "branch", "--show-current") != "wt/mara/main" {
		t.Errorf("the worktree didn't move")
	}
	if list := gitOut(t, api, "worktree", "list", "--porcelain"); !strings.Contains(list, "worktree "+moved) {
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

// Removing a project takes its sandboxes, its logs and its record; its code
// repos keep their branches, and its project repo stays unless asked.
func TestRemoveProject(t *testing.T) {
	h := loaded(t, nil)
	for _, id := range []string{"ledger", "relay"} {
		if _, err := h.NewProject(ctx, id, ""); err != nil {
			t.Fatal(err)
		}
	}
	api := codeRepo(t, "dev/api")
	if _, err := h.AddRepo(ctx, "ledger", "api", api); err != nil {
		t.Fatal(err)
	}
	sandboxWorktree(t, h, "ledger", "mara", "api", api)
	writeFile(t, h.Path("logs", "mara@ledger", "1.log"), "x")
	writeFile(t, h.Path("logs", "kai@relay", "1.log"), "x")
	deployed := true
	h.deployed = func(_ context.Context, project string) bool { return deployed && project == "ledger" }
	if err := h.RemoveProject(ctx, "ledger", false); !errors.Is(err, ErrDeployed) {
		t.Errorf("while deployed: %v", err)
	}
	deployed = false
	if err := h.RemoveProject(ctx, "ledger", false); err != nil {
		t.Fatal(err)
	}
	if _, err := h.Project("ledger"); !errors.Is(err, ErrNotFound) {
		t.Errorf("still a project: %v", err)
	}
	if exists(h.Path("sandboxes", "ledger")) || exists(h.Path("logs", "mara@ledger")) || !exists(h.Path("logs", "kai@relay")) {
		t.Error("sandboxes or logs: wrong ones left")
	}
	if !exists(h.Path("projects", "ledger", "project.toml")) {
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
	if err := h.RemoveProject(ctx, "relay", true); err != nil {
		t.Fatal(err)
	}
	if exists(h.Path("projects", "relay")) {
		t.Error("the project repo stayed, though asked to go")
	}
	if err := h.RemoveProject(ctx, "relay", true); !errors.Is(err, ErrNotFound) {
		t.Errorf("removed twice: %v", err)
	}
}

// A project is gone when its project repo's folder is, or every code repo's
// folder; one of several missing isn't gone.
func TestGone(t *testing.T) {
	h := loaded(t, nil)
	p, _ := h.NewProject(ctx, "ledger", "")
	if p.Gone() != "" {
		t.Error("a project without repos is gone")
	}
	api, web := codeRepo(t, "dev/api"), codeRepo(t, "dev/web")
	h.AddRepo(ctx, "ledger", "api", api)
	p, _ = h.AddRepo(ctx, "ledger", "web", web)
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
