package workspace

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

func gitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func TestCandidatesAreFoldersThatAreNotProjects(t *testing.T) {
	w := loaded(t, func(root string) {
		writeFile(t, filepath.Join(root, "ledger", "project", "project.toml"), `project = "ledger"`)
		gitRepo(t, filepath.Join(root, "billing", "billing-api"))
		if err := os.MkdirAll(filepath.Join(root, "empty"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Join(root, ".hidden"), 0o755); err != nil {
			t.Fatal(err)
		}
	})
	got := w.Candidates()
	if len(got) != 2 || got[0].Folder != "billing" || !slices.Equal(got[0].Repos, []string{"billing-api"}) ||
		got[1].Folder != "empty" || len(got[1].Repos) != 0 {
		t.Errorf("%+v", got)
	}
}

func TestAddProjectWithAFirstRepo(t *testing.T) {
	w := loaded(t, func(root string) {
		if err := os.MkdirAll(filepath.Join(root, "cal"), 0o755); err != nil {
			t.Fatal(err)
		}
	})
	var changes []Change
	w.notify = func(c Change) { changes = append(changes, c) }
	p, err := w.AddProject(ctx, "cal", "cal-go")
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Problems) != 0 || p.Config.Main != "main" || !slices.Equal(p.Repos, []string{"cal-go"}) {
		t.Errorf("project %+v", p)
	}
	dir := filepath.Join(w.Root(), "cal")
	if log := gitOut(t, filepath.Join(dir, "project"), "log", "--format=%s", "--name-only"); !strings.Contains(log, "project.toml") ||
		!strings.Contains(log, "wiki/wiki.toml") || !strings.Contains(log, "wiki/index.md") || !strings.Contains(log, "AGENTS.md") {
		t.Errorf("project repo: %q", log)
	}
	if b := gitOut(t, filepath.Join(dir, "cal-go"), "branch", "--show-current"); b != "main" {
		t.Errorf("repo on %q", b)
	}
	if log := gitOut(t, filepath.Join(dir, "cal-go"), "log", "--format=%s"); log != "First commit" {
		t.Errorf("repo log %q", log)
	}
	if len(changes) == 0 || changes[0].Kind != "projects" { // the list changed: the UI refetches it
		t.Errorf("changes %+v", changes)
	}
	// The wiki Foreman reads is the one it just created.
	if wiki, err := w.WikiDir("cal"); err != nil || wiki != filepath.Join(dir, "project", "wiki") {
		t.Errorf("wiki dir %q %v", wiki, err)
	} else if _, err := os.Stat(filepath.Join(wiki, "wiki.toml")); err != nil {
		t.Errorf("no wiki.toml in the wiki dir: %v", err)
	}
	if _, err := w.WikiDir("nope"); !errors.Is(err, ErrNotFound) {
		t.Errorf("an unknown project's wiki: %v", err)
	}
	if len(w.Candidates()) != 0 {
		t.Error("still a candidate")
	}
	if _, err := w.AddProject(ctx, "cal", ""); !errors.Is(err, ErrInvalid) {
		t.Errorf("twice: %v", err)
	}
}

func TestAddProjectRefusals(t *testing.T) {
	w := loaded(t, func(root string) {
		if err := os.MkdirAll(filepath.Join(root, "cal", "taken"), 0o755); err != nil {
			t.Fatal(err)
		}
	})
	for folder, repo := range map[string]string{"../etc": "", ".foreman": "", "nope": "", "cal": "Bad Name"} {
		if _, err := w.AddProject(ctx, folder, repo); !errors.Is(err, ErrInvalid) {
			t.Errorf("%q %q: %v", folder, repo, err)
		}
	}
	if _, err := w.AddProject(ctx, "cal", "taken"); !errors.Is(err, ErrExists) {
		t.Errorf("an existing folder: %v", err)
	}
	// A refusal leaves nothing behind: the folder is still a candidate.
	if _, err := os.Stat(filepath.Join(w.Root(), "cal", "project")); err == nil {
		t.Error("a refused add left project/")
	}
}

func TestAddRepoOnTheProjectsMainBranch(t *testing.T) {
	w := loaded(t, func(root string) {
		writeFile(t, filepath.Join(root, "ledger", "project", "project.toml"), "project = \"ledger\"\nmain = \"trunk\"\n")
	})
	p, err := w.AddRepo(ctx, "ledger", "ledger-api")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(p.Repos, []string{"ledger-api"}) {
		t.Errorf("repos %v", p.Repos)
	}
	if b := gitOut(t, filepath.Join(w.Root(), "ledger", "ledger-api"), "branch", "--show-current"); b != "trunk" {
		t.Errorf("on %q, not the project's main", b)
	}
	for name, want := range map[string]error{"ledger-api": ErrExists, "Ledger-API": ErrExists, "project": ErrInvalid, "PROJECT": ErrInvalid, "-x": ErrInvalid, "a/b": ErrInvalid, "..": ErrInvalid, "": ErrInvalid} {
		if _, err := w.AddRepo(ctx, "ledger", name); !errors.Is(err, want) {
			t.Errorf("%q: %v, want %v", name, err, want)
		}
	}
	if _, err := w.AddRepo(ctx, "nope", "x"); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown project: %v", err)
	}
}

// A project.toml that lists its repos would not see a new one: refused, with
// what to do instead.
func TestAddRepoRefusesWhenTheReposAreListed(t *testing.T) {
	w := loaded(t, func(root string) {
		writeFile(t, filepath.Join(root, "ledger", "project", "project.toml"), "project = \"ledger\"\nrepos = [\"api\"]\n")
		gitRepo(t, filepath.Join(root, "ledger", "api"))
	})
	if _, err := w.AddRepo(ctx, "ledger", "web"); !errors.Is(err, ErrInvalid) || !strings.Contains(err.Error(), "lists its repos") {
		t.Errorf("%v", err)
	}
	if _, err := os.Stat(filepath.Join(w.Root(), "ledger", "web")); err == nil {
		t.Error("created anyway")
	}
}

func TestNewProjectCreatesTheFolder(t *testing.T) {
	w := loaded(t, nil)
	p, err := w.NewProject(ctx, "cal", "cal")
	if err != nil {
		t.Fatal(err)
	}
	if p.ID != "cal" || len(p.Problems) != 0 || !slices.Equal(p.Repos, []string{"cal"}) {
		t.Errorf("project %+v", p)
	}
	dir := filepath.Join(w.Root(), "cal")
	if log := gitOut(t, filepath.Join(dir, "project"), "log", "--format=%s"); log != "Project repo" {
		t.Errorf("project repo log %q", log)
	}
	if log := gitOut(t, filepath.Join(dir, "cal"), "log", "--format=%s"); log != "First commit" {
		t.Errorf("repo log %q", log)
	}
	// Without a repo: the project alone, which Project settings can give one later.
	p, err = w.NewProject(ctx, "notes", "")
	if err != nil || len(p.Repos) != 0 {
		t.Errorf("no repo: %+v %v", p, err)
	}
	if len(w.Candidates()) != 0 {
		t.Errorf("candidates %+v", w.Candidates())
	}
}

func TestNewProjectRefusalsWriteNothing(t *testing.T) {
	w := loaded(t, func(root string) {
		if err := os.MkdirAll(filepath.Join(root, "billing"), 0o755); err != nil {
			t.Fatal(err)
		}
		writeFile(t, filepath.Join(root, "notes.txt"), "a file")
	})
	before := entries(t, w.Root())
	for name, repo := range map[string]string{
		"": "", ".hidden": "", "..": "", "a..b": "", "a/b": "", "-x": "",
		"cal": "Project", "cal2": "Bad Name",
	} {
		if _, err := w.NewProject(ctx, name, repo); !errors.Is(err, ErrInvalid) {
			t.Errorf("%q %q: %v", name, repo, err)
		}
	}
	for _, name := range []string{"billing", "Billing", "NOTES.TXT", ".foreman"} {
		if _, err := w.NewProject(ctx, name, ""); err == nil {
			t.Errorf("%q: created over what exists", name)
		}
	}
	if _, err := w.NewProject(ctx, "billing", ""); !errors.Is(err, ErrExists) {
		t.Errorf("an existing folder: %v", err)
	}
	if after := entries(t, w.Root()); !slices.Equal(before, after) {
		t.Errorf("the workspace changed: %v → %v", before, after)
	}
}

// failingGit fails the commit of a new code repo, after the project repo's.
type failingGit struct{ Git }

func (g failingGit) Commit(ctx context.Context, dir, message string, files ...string) error {
	if message == "First commit" {
		return errors.New("disk full")
	}
	return g.Git.Commit(ctx, dir, message, files...)
}

func TestAFailedStepLeavesNothingBehind(t *testing.T) {
	w := loaded(t, func(root string) {
		if err := os.MkdirAll(filepath.Join(root, "cal"), 0o755); err != nil {
			t.Fatal(err)
		}
	})
	w.git = failingGit{w.git}
	before := entries(t, w.Root())
	if _, err := w.NewProject(ctx, "ledger", "ledger-api"); err == nil {
		t.Error("new project: no error")
	}
	if _, err := w.AddProject(ctx, "cal", "cal-go"); err == nil {
		t.Error("add project: no error")
	}
	if after := entries(t, w.Root()); !slices.Equal(before, after) {
		t.Errorf("left behind: %v → %v", before, after)
	}
	if got := entries(t, filepath.Join(w.Root(), "cal")); len(got) != 0 {
		t.Errorf("cal/ holds %v", got)
	}
}

func entries(t *testing.T, dir string) []string {
	t.Helper()
	es, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range es {
		names = append(names, e.Name())
	}
	return names
}

// Names keep their case: a workspace's folders already may (DevProject).
func TestNewProjectKeepsUppercase(t *testing.T) {
	w := loaded(t, nil)
	p, err := w.NewProject(ctx, "DevProject", "Some-App")
	if err != nil {
		t.Fatal(err)
	}
	if p.ID != "DevProject" || !slices.Equal(p.Repos, []string{"Some-App"}) {
		t.Errorf("project %+v", p)
	}
	if _, err := w.NewProject(ctx, "devproject", ""); !errors.Is(err, ErrExists) || !strings.Contains(err.Error(), "DevProject") {
		t.Errorf("the same name in another case: %v", err)
	}
}

// A project from before the scaffold (like a wiki the agents filled without
// wiki.toml) gets only what it lacks, committed on its own.
func TestScaffoldFillsWhatIsMissing(t *testing.T) {
	w := loaded(t, func(root string) {
		repo := filepath.Join(root, "old", "project")
		gitRepo(t, repo)
		writeFile(t, filepath.Join(repo, "project.toml"), "project = \"old\"\n")
		writeFile(t, filepath.Join(repo, "wiki", "spec.md"), "# Spec\n")
		gitOut(t, repo, "add", ".")
		gitOut(t, repo, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-qm", "agents")
	})
	repo := filepath.Join(w.Root(), "old", "project")
	got, err := w.Missing("old")
	if err != nil || len(got) != 6 {
		t.Fatalf("missing %+v %v", got, err)
	}
	// The wiki can't be read without its files; it can without AGENTS.md.
	for i, want := range []struct {
		file     string
		required bool
	}{{"wiki/wiki.toml", true}, {"wiki/index.md", true}, {"AGENTS.md", false}, {"wiki/product/index.md", false}, {"wiki/backlog/index.md", false}, {"wiki/technical-debt.md", false}} {
		if got[i].File != want.file || got[i].Required != want.required || got[i].Why == "" {
			t.Errorf("missing[%d] = %+v, want %+v", i, got[i], want)
		}
	}
	// The CEO's own draft index stays: never overwritten.
	writeFile(t, filepath.Join(repo, "wiki", "index.md"), "# Mine\n")
	if err := w.Scaffold(ctx, "old"); err != nil {
		t.Fatal(err)
	}
	if got, _ := w.Missing("old"); len(got) != 0 {
		t.Errorf("still missing %v", got)
	}
	if data, _ := os.ReadFile(filepath.Join(repo, "wiki", "index.md")); string(data) != "# Mine\n" {
		t.Errorf("index.md overwritten: %q", data)
	}
	if data, _ := os.ReadFile(filepath.Join(repo, "AGENTS.md")); !strings.Contains(string(data), "type: task") {
		t.Errorf("AGENTS.md = %q", data)
	}
	if log := gitOut(t, repo, "log", "-1", "--format=%s", "--name-only"); log != "Project repo: add wiki/wiki.toml, AGENTS.md, wiki/product/index.md, wiki/backlog/index.md, wiki/technical-debt.md\n\nAGENTS.md\nwiki/backlog/index.md\nwiki/product/index.md\nwiki/technical-debt.md\nwiki/wiki.toml" {
		t.Errorf("commit %q", log)
	}
	// Nothing missing: nothing written, nothing committed.
	head := gitOut(t, repo, "rev-parse", "HEAD")
	if err := w.Scaffold(ctx, "old"); err != nil || gitOut(t, repo, "rev-parse", "HEAD") != head {
		t.Errorf("a second scaffold: %v", err)
	}
	for _, id := range []string{"nope", "../old"} {
		if _, err := w.Missing(id); !errors.Is(err, ErrNotFound) {
			t.Errorf("Missing(%q): %v", id, err)
		}
		if err := w.Scaffold(ctx, id); !errors.Is(err, ErrNotFound) {
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
	w := loaded(t, nil)
	if _, err := w.NewProject(ctx, "ledger", ""); err != nil {
		t.Fatal(err)
	}
	repo := filepath.Join(w.Root(), "ledger", "project")
	for _, f := range scaffold {
		data, err := os.ReadFile(filepath.Join(repo, f.File))
		if err != nil || strings.Contains(string(data), "{project}") {
			t.Errorf("%s: %v, unreplaced: %v", f.File, err, strings.Contains(string(data), "{project}"))
		}
	}
	if data, _ := os.ReadFile(filepath.Join(repo, "wiki", "index.md")); !strings.Contains(string(data), "# ledger\n") {
		t.Errorf("index.md = %q", data)
	}
	bin, err := exec.LookPath("wiki")
	if err != nil {
		t.Skip("the wiki CLI isn't installed")
	}
	if out, err := exec.Command(bin, "--root", filepath.Join(repo, "wiki"), "check").CombinedOutput(); err != nil || !strings.Contains(string(out), "no issues") {
		t.Errorf("wiki check: %v\n%s", err, out)
	}
}
