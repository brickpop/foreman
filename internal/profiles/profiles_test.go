package profiles

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func write(t *testing.T, path, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
}

func names(t *testing.T, s Store) []string {
	t.Helper()
	list, err := s.List()
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, p := range list {
		out = append(out, p.Name+":"+p.Harness)
	}
	return out
}

func TestCreateListDelete(t *testing.T) {
	s := Store{Root: filepath.Join(t.TempDir(), "profiles")}
	if got := names(t, s); len(got) != 0 {
		t.Errorf("an empty store lists %v", got)
	}
	if err := s.Ensure("claude-code"); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"work", "alpha"} {
		if _, err := s.Create(p, "claude-code"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.Create("zeta", "pi"); err != nil {
		t.Fatal(err)
	}
	// The default first, then by name; a folder without profile.toml isn't one.
	write(t, filepath.Join(s.Root, "stray", "notes.txt"), "x")
	if got := names(t, s); !slices.Equal(got, []string{"default:claude-code", "alpha:claude-code", "work:claude-code", "zeta:pi"}) {
		t.Errorf("list %v", got)
	}
	if info, err := os.Stat(s.Dir("work")); err != nil || info.Mode().Perm() != 0o700 {
		t.Errorf("the folder: %v %v", info, err)
	}
	if _, err := s.Create("work", "claude-code"); !errors.Is(err, ErrExists) {
		t.Errorf("twice: %v", err)
	}
	for _, bad := range []string{"", "Work", "../x", "a/b", "-a", string(make([]byte, 41))} {
		if _, err := s.Create(bad, "claude-code"); !errors.Is(err, ErrInvalid) {
			t.Errorf("created %q: %v", bad, err)
		}
	}
	if _, err := s.Create("ok", ""); !errors.Is(err, ErrInvalid) {
		t.Errorf("no harness: %v", err)
	}
	// Ensure leaves an existing default alone, whatever its harness.
	if err := s.Ensure("pi"); err != nil {
		t.Fatal(err)
	}
	if p, _ := s.Get(Default); p.Harness != "claude-code" {
		t.Errorf("default became %+v", p)
	}
	if err := s.Delete("work"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(s.Dir("work")); !os.IsNotExist(err) {
		t.Errorf("deleted, still there: %v", err)
	}
	if err := s.Delete("work"); !errors.Is(err, ErrNotFound) {
		t.Errorf("deleted twice: %v", err)
	}
	if err := s.Delete(Default); !errors.Is(err, ErrInvalid) {
		t.Errorf("the default deleted: %v", err)
	}
	if err := s.Delete("../profiles"); !errors.Is(err, ErrNotFound) {
		t.Errorf("a path deleted: %v", err)
	}
}

// A name that isn't one never reaches outside the store.
func TestDirStaysInTheStore(t *testing.T) {
	s := Store{Root: "/home/u/.local/share/foreman/profiles"}
	if got := s.Dir("work"); got != "/home/u/.local/share/foreman/profiles/work" {
		t.Errorf("work: %s", got)
	}
	for _, bad := range []string{"", ".", "..", "../../etc", "a/b", `a\b`, "/abs"} {
		if got := s.Dir(bad); got != "/home/u/.local/share/foreman/profiles/default" {
			t.Errorf("%q: %s", bad, got)
		}
		if _, err := s.Get(bad); !errors.Is(err, ErrNotFound) {
			t.Errorf("get %q: %v", bad, err)
		}
	}
}
