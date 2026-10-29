package harness

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
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

type mover struct {
	moves *[]string
	fail  bool
}

func (m mover) MoveConversations(_, from, to string) error {
	if m.fail {
		return errors.New("disk full")
	}
	*m.moves = append(*m.moves, from+">"+to)
	return nil
}

type notMover struct{ Adapter }

// Every harness that files conversations by folder carries them over; if
// one fails, the others' go back.
func TestMoveConversations(t *testing.T) {
	var moves []string
	type both struct {
		Adapter
		mover
	}
	ok := map[string]Adapter{"a": both{mover: mover{moves: &moves}}, "b": notMover{}}
	if err := MoveConversations(ok, "/h", "/old", "/new"); err != nil || len(moves) != 1 || moves[0] != "/old>/new" {
		t.Errorf("%v %v", err, moves)
	}
	moves = nil
	failing := map[string]Adapter{"a": both{mover: mover{moves: &moves}}, "z": both{mover: mover{fail: true}}}
	err := MoveConversations(failing, "/h", "/old", "/new")
	if err == nil || (len(moves) != 0 && !reflect.DeepEqual(moves, []string{"/old>/new", "/new>/old"})) {
		t.Errorf("%v %v", err, moves)
	}
}
