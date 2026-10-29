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

	top, err := h.Browse("")
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, e := range top.Entries {
		got = append(got, e.Name+map[bool]string{true: " (repo)"}[e.Repo])
	}
	if top.Path != real || top.Parent != realHome || strings.Join(got, ", ") != "app (repo), link-to-app (repo), Notes" {
		t.Errorf("top: %s, parent %q, %v", top.Path, top.Parent, got)
	}
	notes, err := h.Browse(filepath.Join(dev, "Notes"))
	if err != nil || notes.Parent != real || len(notes.Entries) != 0 {
		t.Errorf("a subfolder: %+v %v", notes, err)
	}
	// Up to the home folder, and no higher.
	up, err := h.Browse(home)
	if err != nil || up.Path != realHome || up.Parent != "" {
		t.Errorf("the home folder: %+v %v", up, err)
	}
	for _, path := range []string{filepath.Dir(home), filepath.Join(dev, "out"), "dev", filepath.Join(dev, "readme.txt"), filepath.Join(dev, "nope")} {
		if _, err := h.Browse(path); !errors.Is(err, ErrInvalid) {
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
	if _, _, err := h.NewProject(ctx, "ledger", "", workspace("ledger")); err != nil {
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
