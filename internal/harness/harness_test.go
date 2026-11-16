package harness

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteFilesMakesFoldersAndModes(t *testing.T) {
	dir := t.TempDir()
	files := []File{
		{Path: filepath.Join(dir, "a", "b", "CLAUDE.md"), Data: []byte("prompt"), Mode: 0o600},
		{Path: filepath.Join(dir, "settings.json"), Data: []byte("{}"), Mode: 0o644},
	}
	if err := WriteFiles(files); err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		info, err := os.Stat(f.Path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != f.Mode {
			t.Errorf("%s mode %o, want %o", f.Path, info.Mode().Perm(), f.Mode)
		}
		if data, _ := os.ReadFile(f.Path); string(data) != string(f.Data) {
			t.Errorf("%s holds %q", f.Path, data)
		}
	}
}

// A link replaces whatever is at its path, a file or an older link, and
// writes go through it to the target.
func TestWriteFilesLinks(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "home", "auth.json")
	target := filepath.Join(dir, "profile", "auth.json")
	if err := WriteFiles([]File{{Path: path, Data: []byte("stale copy"), Mode: 0o600}}); err != nil {
		t.Fatal(err)
	}
	for _, link := range []string{filepath.Join(dir, "old", "auth.json"), target} {
		if err := WriteFiles([]File{{Path: path, Link: link}}); err != nil {
			t.Fatal(err)
		}
	}
	if got, err := os.Readlink(path); err != nil || got != target {
		t.Fatalf("link to %q (%v), want %q", got, err, target)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("login"), 0o600); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(path); string(data) != "login" {
		t.Errorf("read through the link: %q", data)
	}
}
