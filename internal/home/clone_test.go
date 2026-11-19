package home

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCloneDir(t *testing.T) {
	h := &Home{folder: "/home/ceo/foreman"}
	for remote, want := range map[string]string{
		"git@github.com:me/ledger-api.git":                  "me/ledger-api",
		"git@github.com:me/ledger-api":                      "me/ledger-api",
		"ssh://git@github.com/me/ledger-api.git":            "me/ledger-api",
		"ssh://git@host:2222/~ceo/ledger.git":               "ceo/ledger",
		"https://gitlab.com/acme/platform/infra/deploy.git": "acme/platform/infra/deploy",
		"https://ghp_secret@github.com/me/api.git/":         "me/api",
		"git://example.org/tools/x.y_z":                     "tools/x.y_z",
		"https://github.com/Me/App.git":                     "Me/App",
	} {
		got, err := h.cloneDir(remote)
		if err != nil || got != filepath.Join("/home/ceo/foreman/repos", want) {
			t.Errorf("%s: %q %v, want repos/%s", remote, got, err, want)
		}
	}
	for _, remote := range []string{
		"/home/ceo/dev/api",                 // a local path: pick it
		"./api",                             // relative
		"file:///home/ceo/dev/api",          // a local path still
		"ext::sh -c touch% /tmp/pwned",      // no command transports
		"https://github.com",                // no path
		"https://github.com/me/../../etc",   // out of repos/
		"git@github.com:../me/api.git",      // out of repos/
		"https://github.com/me/.hidden.git", // not a repo name
		"git@github.com:me//api.git",        // an empty part
	} {
		if got, err := h.cloneDir(remote); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: %q %v, want refused", remote, got, err)
		}
	}
}

// remotes makes https://git.example.invalid/<path> reach repos under a
// temporary folder, through the CEO's git config (insteadOf), as a real
// remote would.
func remotes(t *testing.T) string {
	t.Helper()
	served := t.TempDir()
	config := filepath.Join(t.TempDir(), "gitconfig")
	writeFile(t, config, "[url \""+served+"/\"]\n\tinsteadOf = https://git.example.invalid/\n")
	t.Setenv("GIT_CONFIG_GLOBAL", config)
	return served
}

// served makes a repo at path under served, on branch, with a first commit.
func served(t *testing.T, root, path, branch string) string {
	t.Helper()
	dir := filepath.Join(root, path)
	writeFile(t, filepath.Join(dir, "README.md"), "# "+path+"\n")
	gitOut(t, dir, "init", "--quiet", "--initial-branch="+branch)
	gitOut(t, dir, "add", "README.md")
	gitOut(t, dir, "commit", "--quiet", "-m", "first")
	return dir
}

// A repo cloned into repos/<its URL's path>, once: another project, or the
// same URL again, uses that clone; anything else there is refused and left.
func TestCloneRepo(t *testing.T) {
	h := loaded(t, nil)
	root := remotes(t)
	api := served(t, root, "acme/api.git", "main")
	served(t, root, "acme/legacy.git", "trunk")
	for _, id := range []string{"ledger", "relay"} {
		if _, _, err := h.NewProject(ctx, id, "", ""); err != nil {
			t.Fatal(err)
		}
	}
	const url = "https://git.example.invalid/acme/api.git"
	p, err := h.CloneRepo(ctx, "ledger", "api", url, "")
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(h.Folder(), "repos", "acme", "api")
	first := gitOut(t, api, "rev-list", "--max-parents=0", "HEAD")
	if len(p.Repos) != 1 || p.Repos[0].Path != dir || p.Repos[0].Default != "main" || !p.Repos[0].Ready() {
		t.Errorf("%+v", p.Repos)
	}
	if got := readRecords(t, h)["ledger"].Repos["api"]; got.Remote != url || got.RootCommit != first {
		t.Errorf("record %+v", got)
	}
	// The same clone for another project.
	writeFile(t, filepath.Join(dir, "work.txt"), "the CEO's\n")
	if p, err := h.CloneRepo(ctx, "relay", "core", url, ""); err != nil || p.Repos[0].Path != dir {
		t.Errorf("reused: %+v %v", p.Repos, err)
	}
	if !exists(filepath.Join(dir, "work.txt")) {
		t.Error("the clone was made again")
	}
	// No main: the branch the remote is on.
	if p, err := h.CloneRepo(ctx, "ledger", "legacy", "https://git.example.invalid/acme/legacy.git", ""); err != nil || p.Repos[1].Default != "trunk" {
		t.Errorf("default: %+v %v", p.Repos, err)
	}

	// Another repo at the place: refused, untouched.
	other := filepath.Join(h.Folder(), "repos", "acme", "web")
	codeRepo(t, other)
	served(t, root, "acme/web.git", "main")
	if _, err := h.CloneRepo(ctx, "ledger", "web", "https://git.example.invalid/acme/web.git", ""); !errors.Is(err, ErrExists) || !strings.Contains(err.Error(), "another repo") {
		t.Errorf("another repo there: %v", err)
	}
	if !exists(filepath.Join(other, "README.md")) {
		t.Error("the repo there was touched")
	}
	plain := filepath.Join(h.Folder(), "repos", "acme", "plain")
	writeFile(t, filepath.Join(plain, "notes"), "")
	if _, err := h.CloneRepo(ctx, "ledger", "plain", "https://git.example.invalid/acme/plain.git", ""); !errors.Is(err, ErrExists) {
		t.Errorf("a plain folder there: %v", err)
	}
	// A clone that fails leaves nothing, the folders it made included.
	if _, err := h.CloneRepo(ctx, "ledger", "gone", "https://git.example.invalid/nobody/gone.git", ""); !errors.Is(err, ErrInvalid) {
		t.Errorf("no such remote: %v", err)
	}
	if exists(filepath.Join(h.Folder(), "repos", "nobody")) {
		t.Error("a failed clone left its folder")
	}
	// Refused before cloning: a taken name, a local path.
	if _, err := h.CloneRepo(ctx, "ledger", "API", url, ""); !errors.Is(err, ErrExists) {
		t.Errorf("a taken name: %v", err)
	}
	if _, err := h.CloneRepo(ctx, "ledger", "local", api, ""); !errors.Is(err, ErrInvalid) {
		t.Errorf("a local path: %v", err)
	}
	if _, err := h.CloneRepo(ctx, "nope", "x", url, ""); !errors.Is(err, ErrNotFound) {
		t.Errorf("no such project: %v", err)
	}
}

// A repo from another machine's record is cloned from the remote it keeps,
// and only when it is the same repo.
func TestCloneMissingRepo(t *testing.T) {
	h := loaded(t, nil)
	root := remotes(t)
	api := served(t, root, "acme/api.git", "main")
	first := gitOut(t, api, "rev-list", "--max-parents=0", "HEAD")
	writeFile(t, h.HQ(projectsFile), `[ledger.repos.api]
remote = "https://git.example.invalid/acme/api.git"
root_commit = "`+first+`"
default_branch = "main"

[ledger.repos.web]
root_commit = "`+first+`"
default_branch = "main"

[ledger.repos.moved]
remote = "https://git.example.invalid/acme/moved.git"
root_commit = "`+strings.Repeat("0", 40)+`"
default_branch = "main"
`)
	served(t, root, "acme/moved.git", "main")
	reload(t, h)
	p, err := h.CloneMissingRepo(ctx, "ledger", "api")
	if err != nil {
		t.Fatal(err)
	}
	if p.Repos[0].ID != "api" || p.Repos[0].Path != filepath.Join(h.Folder(), "repos", "acme", "api") || !p.Repos[0].Ready() {
		t.Errorf("%+v", p.Repos[0])
	}
	if _, err := h.CloneMissingRepo(ctx, "ledger", "web"); !errors.Is(err, ErrInvalid) || !strings.Contains(err.Error(), "no remote") {
		t.Errorf("no remote recorded: %v", err)
	}
	// The remote holds another repo now: refused, the clone gone.
	if _, err := h.CloneMissingRepo(ctx, "ledger", "moved"); !errors.Is(err, ErrInvalid) {
		t.Errorf("another repo at the remote: %v", err)
	}
	if exists(filepath.Join(h.Folder(), "repos", "acme", "moved")) {
		t.Error("the refused clone stayed")
	}
	if _, err := h.CloneMissingRepo(ctx, "ledger", "nope"); !errors.Is(err, ErrNotFound) {
		t.Errorf("no such repo: %v", err)
	}
	if entries, _ := os.ReadDir(filepath.Join(h.Folder(), "repos", "acme")); len(entries) != 1 {
		t.Errorf("repos/acme: %v", entries)
	}
}
