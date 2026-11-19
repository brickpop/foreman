package home

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Foreman's folder is ~/foreman unless settings.toml names another, after ~
// or absolute; everything of a project follows from it.
func TestForemanFolder(t *testing.T) {
	h := loaded(t, nil)
	home, _ := filepath.EvalSymlinks(os.Getenv("HOME"))
	if h.Folder() != filepath.Join(home, "foreman") || !topOfRepo(h.HQ()) {
		t.Errorf("default folder %s, HQ a repo: %v", h.Folder(), topOfRepo(h.HQ()))
	}
	// After ~, or absolute: the same place.
	for _, absolute := range []bool{false, true} {
		h := loaded(t, func(root string) {
			set := "~/work/f"
			if absolute {
				set = filepath.Join(os.Getenv("HOME"), "work", "f")
			}
			writeFile(t, filepath.Join(root, "settings.toml"), "foreman_folder = \""+set+"\"\n")
		})
		home, _ := filepath.EvalSymlinks(os.Getenv("HOME"))
		want := filepath.Join(home, "work", "f")
		if h.Folder() != want || h.HQ() != filepath.Join(want, "hq") || !topOfRepo(h.HQ()) {
			t.Errorf("absolute %v: folder %s, HQ %s", absolute, h.Folder(), h.HQ())
		}
		p, _, err := h.NewProject(ctx, "ledger", "", "")
		if err != nil {
			t.Fatal(err)
		}
		if p.Path != filepath.Join(want, "projects", "ledger", "hq") || p.Sandbox("mara") != filepath.Join(want, "worktrees", "ledger", "mara") {
			t.Errorf("absolute %v: project at %s, sandbox %s", absolute, p.Path, p.Sandbox("mara"))
		}
	}
}

// Foreman doesn't start on a folder it can't use: one agents can't reach
// (outside the CEO's home folder), one overlapping its home, a relative one,
// or none it can read (settings.toml doesn't parse). Nothing is made.
func TestAnUnusableForemanFolderStopsTheStart(t *testing.T) {
	outside := t.TempDir()
	for name, c := range map[string]struct{ settings, says string }{
		"outside the home folder": {`foreman_folder = "` + outside + `"`, "outside your home folder"},
		"inside Foreman's home":   {`foreman_folder = "~/.foreman/f"`, "overlaps Foreman's home"},
		"holding Foreman's home":  {`foreman_folder = "~"`, "overlaps Foreman's home"},
		"relative":                {`foreman_folder = "foreman"`, "must be an absolute path"},
		"empty":                   {`foreman_folder = ""`, "must be an absolute path"},
		"a broken settings.toml":  {`port = `, "foreman_folder is read from it"},
	} {
		h, err := load(t, func(root string) { writeFile(t, filepath.Join(root, "settings.toml"), c.settings+"\n") })
		if err == nil || !strings.Contains(err.Error(), c.says) {
			t.Errorf("%s: %v", name, err)
		}
		if entries := entries(t, outside); len(entries) > 0 || exists(h.Path("f")) {
			t.Errorf("%s: made %v", name, entries)
		}
	}
}

// A project's files are in its folder, never in Foreman's home, and
// MakeFiles makes their two folders, keeping what is in them.
func TestProjectFiles(t *testing.T) {
	h := loaded(t, nil)
	p, _, err := h.NewProject(ctx, "ledger", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(h.Folder(), "projects", "ledger", "files"); p.Files != want {
		t.Fatalf("files %q, want %q", p.Files, want)
	}
	if err := p.MakeFiles(); err != nil {
		t.Fatal(err)
	}
	kept := filepath.Join(p.Files, FilesInbox, "brief.pdf")
	writeFile(t, kept, "%PDF\n")
	if err := p.MakeFiles(); err != nil { // again: nothing lost
		t.Fatal(err)
	}
	for _, f := range FileFolders {
		if fi, err := os.Stat(filepath.Join(p.Files, f)); err != nil || !fi.IsDir() {
			t.Errorf("%s: %v", f, err)
		}
	}
	if _, err := os.Stat(kept); err != nil {
		t.Errorf("the inbox lost its file: %v", err)
	}
}
