package home

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The browser lists one folder at a time, opening where Foreman was started
// and going up to the CEO's home folder, never above it: subfolders only,
// hidden ones, symlinks leading out and Foreman's home left out, git repos
// marked.
func TestBrowse(t *testing.T) {
	h := loaded(t, nil)
	home := os.Getenv("HOME")
	realHome, _ := filepath.EvalSymlinks(home)
	dev := filepath.Join(home, "dev")
	codeRepo(t, filepath.Join(dev, "app"))
	writeFile(t, filepath.Join(dev, "Notes", "todo.md"), "x")
	writeFile(t, filepath.Join(dev, ".cache", "x"), "x")
	writeFile(t, filepath.Join(dev, "readme.txt"), "x")
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(dev, "out")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(dev, "app"), filepath.Join(dev, "link-to-app")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(h.Root(), filepath.Join(dev, "foreman-home")); err != nil {
		t.Fatal(err)
	}
	startedIn(t, h, dev)
	real, _ := filepath.EvalSymlinks(dev)

	top, err := h.Browse("", "")
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, e := range top.Entries {
		got = append(got, e.Name+map[bool]string{true: " (repo)"}[e.Repo])
	}
	if top.Path != real || top.Home != realHome || top.Repo || strings.Join(got, ", ") != "app (repo), link-to-app (repo), Notes" {
		t.Errorf("top: %s, home %q, repo %v, %v", top.Path, top.Home, top.Repo, got)
	}
	notes, err := h.Browse(filepath.Join(dev, "Notes"), "")
	if err != nil || notes.Path != filepath.Join(real, "Notes") || notes.Repo || len(notes.Entries) != 0 {
		t.Errorf("a subfolder: %+v %v", notes, err)
	}
	// A repo's top folder is one to add, and through a symlink too.
	for _, path := range []string{filepath.Join(dev, "app"), filepath.Join(dev, "link-to-app")} {
		if app, err := h.Browse(path, ""); err != nil || !app.Repo || app.Path != filepath.Join(real, "app") {
			t.Errorf("%s: %+v %v", path, app, err)
		}
	}
	// Up to the home folder, and no higher.
	up, err := h.Browse(home, "")
	if err != nil || up.Path != realHome || up.Home != realHome {
		t.Errorf("the home folder: %+v %v", up, err)
	}
	for _, path := range []string{filepath.Dir(home), filepath.Join(dev, "out"), "dev", filepath.Join(dev, "readme.txt"), filepath.Join(dev, "nope")} {
		if _, err := h.Browse(path, ""); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: %v", path, err)
		}
	}
}

// Repos are added from anywhere in the CEO's home folder, wherever Foreman
// was started (the CEO, 2026-10-28), never from outside it nor from
// Foreman's home.
func TestTheHomeFolderBoundsAddingRepos(t *testing.T) {
	h := loaded(t, nil)
	home := os.Getenv("HOME")
	if _, _, err := h.NewProject(ctx, "ledger", "", ""); err != nil {
		t.Fatal(err)
	}
	startedIn(t, h, filepath.Join(home, "dev"))
	elsewhere := codeRepo(t, filepath.Join(home, "src", "api"))
	if _, err := h.AddRepo(ctx, "ledger", "api", elsewhere, ""); err != nil {
		t.Errorf("outside the start folder: %v", err)
	}
	if _, err := h.RepoBranches(ctx, elsewhere); err != nil {
		t.Errorf("its branches: %v", err)
	}
	inHome := codeRepo(t, h.Path("stray"))
	if _, err := h.AddRepo(ctx, "ledger", "stray", inHome, ""); err == nil || !strings.Contains(err.Error(), "inside Foreman's home") {
		t.Errorf("in Foreman's home: %v", err)
	}
	if _, err := h.NewRepo(ctx, "ledger", "fresh", h.Path("fresh")); err == nil || !strings.Contains(err.Error(), "inside Foreman's home") {
		t.Errorf("a new repo in Foreman's home: %v", err)
	}
	// Started outside the CEO's home folder: the browser opens at the home folder.
	startedIn(t, h, t.TempDir())
	if real, _ := filepath.EvalSymlinks(home); h.Start() != real {
		t.Errorf("started outside the home folder: start %s", h.Start())
	}
}

// Browsed for a project, a folder says which of its repos it is already,
// by real path: a repo added through a symlink is still that folder.
func TestBrowseSaysWhatTheProjectHasAdded(t *testing.T) {
	h := loaded(t, nil)
	home := os.Getenv("HOME")
	if _, _, err := h.NewProject(ctx, "ledger", "", ""); err != nil {
		t.Fatal(err)
	}
	app := codeRepo(t, filepath.Join(home, "dev", "app"))
	other := codeRepo(t, filepath.Join(home, "dev", "other"))
	link := filepath.Join(home, "dev", "link-to-app")
	if err := os.Symlink(app, link); err != nil {
		t.Fatal(err)
	}
	if _, err := h.AddRepo(ctx, "ledger", "api", link, ""); err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string]string{app: "api", link: "api", other: ""} {
		if f, err := h.Browse(path, "ledger"); err != nil || f.Added != want {
			t.Errorf("%s: added %q, %v", path, f.Added, err)
		}
	}
	if f, _ := h.Browse(app, ""); f.Added != "" {
		t.Errorf("for no project: %q", f.Added)
	}
	if _, err := h.Browse(app, "nope"); !errors.Is(err, ErrNotFound) {
		t.Errorf("an unknown project: %v", err)
	}
}

// Foreman's own folders show for what they are: its HQ, projects/ and
// worktrees/ (their repos are never code repos); repos/, where the code
// repos it clones go, is not one of them.
func TestBrowseMarksForemansFolders(t *testing.T) {
	h := loaded(t, nil)
	home := os.Getenv("HOME")
	if _, _, err := h.NewProject(ctx, "ledger", "", ""); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{"repos", "worktrees"} {
		if err := os.MkdirAll(filepath.Join(h.Folder(), dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	marks := func(path string) string {
		f, err := h.Browse(path, "")
		if err != nil {
			t.Fatal(err)
		}
		var got []string
		for _, e := range f.Entries {
			got = append(got, e.Name+"="+e.Foreman)
		}
		return f.Foreman + " | " + strings.Join(got, ", ")
	}
	if got := marks(home); got != " | foreman=" {
		t.Errorf("the home folder: %q", got)
	}
	if got := marks(h.Folder()); got != " | hq=Foreman's HQ, projects=Foreman's projects, repos=, worktrees=the agents' worktrees" {
		t.Errorf("Foreman's folder: %q", got)
	}
	// Through a symlink, the same folder.
	if err := os.Symlink(filepath.Join(h.Folder(), "projects"), filepath.Join(home, "p")); err != nil {
		t.Fatal(err)
	}
	if f, err := h.Browse(filepath.Join(home, "p"), ""); err != nil || f.Foreman != "Foreman's projects" {
		t.Errorf("projects/ through a symlink: %+v %v", f, err)
	}
}

// No code repo goes in Foreman's own folders, whichever way it comes (added,
// created, located; through a symlink too): the server refuses what the
// browser never offers. One in repos/ is fine.
func TestNoCodeRepoInForemansFolders(t *testing.T) {
	h := loaded(t, nil)
	home := os.Getenv("HOME")
	ledger, _, err := h.NewProject(ctx, "ledger", "", "")
	if err != nil {
		t.Fatal(err)
	}
	strays := []string{
		ledger.Path,
		codeRepo(t, filepath.Join(h.Folder(), "projects", "stray")),
		codeRepo(t, filepath.Join(h.Folder(), "worktrees", "stray")),
		codeRepo(t, h.HQ("stray")),
	}
	if err := os.Symlink(ledger.Path, filepath.Join(home, "link")); err != nil {
		t.Fatal(err)
	}
	for _, path := range append(strays, filepath.Join(home, "link")) {
		if _, err := h.AddRepo(ctx, "ledger", "x", path, ""); !errors.Is(err, ErrInvalid) || !strings.Contains(err.Error(), "Foreman's own") {
			t.Errorf("added %s: %v", path, err)
		}
	}
	for _, path := range []string{filepath.Join(h.Folder(), "projects", "ledger", "fresh"), filepath.Join(h.Folder(), "worktrees", "fresh"), h.HQ("fresh")} {
		if _, err := h.NewRepo(ctx, "ledger", "fresh", path); !errors.Is(err, ErrInvalid) || exists(path) {
			t.Errorf("created %s: %v", path, err)
		}
	}
	api := codeRepo(t, filepath.Join(home, "dev", "api"))
	if _, err := h.AddRepo(ctx, "ledger", "api", api, ""); err != nil {
		t.Fatalf("a repo of the CEO's: %v", err)
	}
	if _, err := h.LocateRepo(ctx, "ledger", "api", ledger.Path); !errors.Is(err, ErrInvalid) {
		t.Errorf("located in an HQ: %v", err)
	}
	if _, err := h.NewRepo(ctx, "ledger", "web", filepath.Join(home, "dev", "web")); err != nil {
		t.Errorf("a new repo of the CEO's: %v", err)
	}
	if _, err := h.AddRepo(ctx, "ledger", "app", codeRepo(t, filepath.Join(h.Folder(), "repos", "me", "app")), ""); err != nil {
		t.Errorf("a repo in repos/: %v", err)
	}
}
