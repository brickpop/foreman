package repos

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/brickpop/foreman/internal/git"
	"github.com/brickpop/foreman/internal/home"
)

var ctx = context.Background()

type config struct{ root string }

// Project is ledger: its repo api, and web, which is missing.
func (c config) Project(id string) (home.Project, error) {
	if id != "ledger" {
		return home.Project{}, home.ErrNotFound
	}
	return home.Project{ID: id, Path: filepath.Join(c.root, "ledger", "project"), Config: home.ProjectConfig{},
		Repos: []home.Repo{{Name: "api", Path: filepath.Join(c.root, "ledger", "api"), Main: "trunk"}, {Name: "web", Path: filepath.Join(c.root, "gone"), Problem: "missing"}}}, nil
}

func (c config) Sandbox(project, agent string) string {
	return filepath.Join(c.root, project, ".sandboxes", agent)
}

func gitIn(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=t", "-c", "user.email=t@t", "-c", "commit.gpgsign=false"}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v %s", args, err, out)
	}
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// fixture: ledger with code repo api (main is trunk) and its project repo;
// tove works on wt/tove/x in a worktree, one commit ahead and an edit not
// committed; outside.txt sits beside the repos.
func fixture(t *testing.T) Browser {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	root := t.TempDir()
	api := filepath.Join(root, "ledger", "api")
	gitIn(t, root, "init", "--quiet", "--initial-branch=trunk", api)
	write(t, filepath.Join(api, "README.md"), "# API\n")
	write(t, filepath.Join(api, "logo.png"), "\x89PNG\x00\x01")
	write(t, filepath.Join(api, "big.txt"), strings.Repeat("x", FileLimit+1))
	gitIn(t, api, "add", ".")
	gitIn(t, api, "commit", "--quiet", "-m", "first")
	tree := filepath.Join(root, "ledger", ".sandboxes", "tove", "api")
	gitIn(t, api, "worktree", "add", "--quiet", "-b", "wt/tove/x", tree)
	write(t, filepath.Join(tree, "x.go"), "package x\n")
	gitIn(t, tree, "add", "x.go")
	t.Setenv("GIT_COMMITTER_DATE", "2030-01-01T10:00:00Z") // newer than trunk, whatever the clock
	gitIn(t, tree, "commit", "--quiet", "-m", "x")
	write(t, filepath.Join(tree, "x.go"), "package x // edited\n")
	project := filepath.Join(root, "ledger", "project")
	gitIn(t, root, "init", "--quiet", "--initial-branch=main", project)
	write(t, filepath.Join(project, "project.toml"), "project = \"ledger\"\n")
	gitIn(t, project, "add", ".")
	gitIn(t, project, "commit", "--quiet", "-m", "Project repo")
	write(t, filepath.Join(root, "ledger", "outside.txt"), "secret\n")
	return Browser{Config: config{root}, Git: git.New()}
}

func TestReposListCodeReposThenTheProjectRepo(t *testing.T) {
	b := fixture(t)
	list, err := b.Repos(ctx, "ledger")
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || list[0].Name != "api" || list[0].Project || list[1].Name != "project" || !list[1].Project {
		t.Fatalf("repos %+v", list)
	}
	api := list[0]
	if api.Main != "trunk" || len(api.Branches) != 2 || api.Branches[0].Name != "trunk" {
		t.Fatalf("api %+v", api)
	}
	if x := api.Branches[1]; x.Name != "wt/tove/x" || x.Ahead != 1 || !x.Uncommitted || x.Agent != "tove" {
		t.Errorf("tove's branch %+v", x)
	}
	// trunk is checked out in the CEO's own checkout: no agent's.
	if trunk := api.Branches[0]; trunk.Agent != "" {
		t.Errorf("trunk %+v", trunk)
	}
	if _, err := b.Repos(ctx, "nope"); !errors.Is(err, home.ErrNotFound) {
		t.Errorf("unknown project: %v", err)
	}
}

func TestFilesAsCommitted(t *testing.T) {
	b := fixture(t)
	f, err := b.File(ctx, "ledger", "api", "wt/tove/x", "x.go")
	if err != nil || f.Text != "package x\n" || f.Size != 10 {
		t.Errorf("x.go %+v %v", f, err)
	}
	if f, _ := b.File(ctx, "ledger", "api", "trunk", "logo.png"); !f.Binary || f.Text != "" {
		t.Errorf("binary %+v", f)
	}
	if f, _ := b.File(ctx, "ledger", "api", "trunk", "big.txt"); !f.TooLarge || f.Text != "" || f.Size != FileLimit+1 {
		t.Errorf("large: too large %v, %d bytes of text, size %d", f.TooLarge, len(f.Text), f.Size)
	}
	entries, err := b.Tree(ctx, "ledger", "project", "main", "")
	if err != nil || len(entries) != 1 || entries[0].Name != "project.toml" {
		t.Errorf("project repo %+v %v", entries, err)
	}
	page, err := b.Log(ctx, "ledger", "api", "wt/tove/x", 0)
	if err != nil || len(page.Commits) != 2 || page.Commits[0].Subject != "x" || page.Next != 0 {
		t.Errorf("log %+v %v", page, err)
	}
}

// Only the project's repos, and only what is in them.
func TestBrowserRefusals(t *testing.T) {
	b := fixture(t)
	for _, repo := range []string{"..", ".sandboxes", "outside.txt", "nope", "", "web"} {
		if _, err := b.Tree(ctx, "ledger", repo, "trunk", ""); !errors.Is(err, ErrNoRepo) {
			t.Errorf("repo %q: %v", repo, err)
		}
	}
	if _, err := b.Tree(ctx, "ledger", "api", "nope", ""); !errors.Is(err, git.ErrNoRef) {
		t.Errorf("unknown ref: %v", err)
	}
	if _, err := b.File(ctx, "ledger", "api", "trunk", "../outside.txt"); !errors.Is(err, git.ErrNoPath) {
		t.Errorf("a path out of the repo: %v", err)
	}
}

// A repo whose main branch is gone (deleted since it was picked) still lists: its
// branches just have nothing to be ahead of.
func TestAMainTheRepoHasNot(t *testing.T) {
	b := fixture(t)
	b.Config = mainIs{b.Config.(config), "develop"}
	list, err := b.Repos(ctx, "ledger")
	if err != nil {
		t.Fatal(err)
	}
	// Newest first, with no main to put first.
	if api := list[0]; len(api.Branches) != 2 || api.Branches[0].Name != "wt/tove/x" || api.Branches[0].Ahead != 0 {
		t.Errorf("api %+v", api)
	}
}

type mainIs struct {
	config
	main string
}

func (m mainIs) Project(id string) (home.Project, error) {
	p, err := m.config.Project(id)
	p.Repos[0].Main = m.main
	return p, err
}
