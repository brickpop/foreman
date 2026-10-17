package workspace

import (
	"context"
	"errors"
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
		!strings.Contains(log, "wiki/wiki.toml") || !strings.Contains(log, "wiki/index.md") {
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
