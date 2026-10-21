package home

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The browser lists one folder at a time from where Foreman was started,
// never above it: subfolders only, hidden ones and symlinks leading out left
// out, git repos marked.
func TestBrowse(t *testing.T) {
	h := loaded(t, nil)
	home := os.Getenv("HOME")
	dev := filepath.Join(home, "dev")
	codeRepo(t, filepath.Join(dev, "app"))
	writeFile(t, filepath.Join(dev, "Notes", "todo.md"), "x")
	writeFile(t, filepath.Join(dev, ".cache", "x"), "x")
	writeFile(t, filepath.Join(dev, "readme.txt"), "x")
	writeFile(t, filepath.Join(home, "elsewhere", "x"), "x")
	if err := os.Symlink(filepath.Join(home, "elsewhere"), filepath.Join(dev, "out")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(dev, "app"), filepath.Join(dev, "link-to-app")); err != nil {
		t.Fatal(err)
	}
	startedIn(t, h, dev)
	real, _ := filepath.EvalSymlinks(dev)

	top, err := h.Browse("")
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, e := range top.Entries {
		got = append(got, e.Name+map[bool]string{true: " (repo)"}[e.Repo])
	}
	if top.Path != real || top.Parent != "" || strings.Join(got, ", ") != "app (repo), link-to-app (repo), Notes" {
		t.Errorf("top: %s, parent %q, %v", top.Path, top.Parent, got)
	}
	notes, err := h.Browse(filepath.Join(dev, "Notes"))
	if err != nil || notes.Parent != real || len(notes.Entries) != 0 {
		t.Errorf("a subfolder: %+v %v", notes, err)
	}
	for _, path := range []string{home, filepath.Join(dev, "out"), "dev", filepath.Join(dev, "readme.txt"), filepath.Join(dev, "nope")} {
		if _, err := h.Browse(path); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: %v", path, err)
		}
	}
}

// Foreman started outside the CEO's home folder adds repos from that home
// folder; started inside it, from where it was started.
func TestTheStartFolderBoundsAddingRepos(t *testing.T) {
	h := loaded(t, nil)
	home := os.Getenv("HOME")
	p, _, err := h.NewProject(ctx, "ledger", "")
	if err != nil {
		t.Fatal(err)
	}
	outside := codeRepo(t, filepath.Join(home, "src", "api"))
	if p, err = h.AddRepo(ctx, "ledger", "api", outside, ""); err != nil {
		t.Fatal(err)
	}
	inside := codeRepo(t, filepath.Join(home, "dev", "web"))
	startedIn(t, h, filepath.Join(home, "dev"))
	if filepath.Base(h.Start()) != "dev" {
		t.Fatalf("start %s", h.Start())
	}
	if _, err := h.AddRepo(ctx, "ledger", "web", inside, ""); err != nil {
		t.Errorf("inside the start folder: %v", err)
	}
	other := codeRepo(t, filepath.Join(home, "src", "docs"))
	if _, err := h.AddRepo(ctx, "ledger", "docs", other, ""); err == nil || !strings.Contains(err.Error(), "where repos are added from") {
		t.Errorf("outside the start folder: %v", err)
	}
	if _, err := h.LocateRepo(ctx, "ledger", "api", outside); err == nil || !strings.Contains(err.Error(), "where repos are added from") {
		t.Errorf("located outside the start folder: %v", err)
	}
	// A repo added before stays usable.
	if p, _ = h.Project("ledger"); !p.Repos[0].Ready() {
		t.Errorf("the repo added before: %+v", p.Repos[0])
	}
	// Started outside the CEO's home folder: the home folder is the bound.
	startedIn(t, h, t.TempDir())
	if real, _ := filepath.EvalSymlinks(home); h.Start() != real {
		t.Errorf("started outside the home folder: start %s", h.Start())
	}
	if _, err := h.AddRepo(ctx, "ledger", "docs", other, ""); err != nil {
		t.Errorf("then anywhere in the home folder: %v", err)
	}
}
