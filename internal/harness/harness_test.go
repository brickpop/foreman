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
