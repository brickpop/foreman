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

func read(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
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

// A login is its harness's file, as Claude Code's adapter checks it.
var connected = map[string]func(string) bool{
	"claude-code": func(dir string) bool {
		_, err := os.Stat(filepath.Join(dir, ".credentials.json"))
		return err == nil
	},
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

// workspace makes a workspace folder named name, with its own Claude Code
// login (from before profiles) holding account, unless account is "".
func workspace(t *testing.T, name, account string) string {
	t.Helper()
	ws := filepath.Join(t.TempDir(), name)
	if account != "" {
		write(t, filepath.Join(ws, ".foreman", "credentials", "claude-code", ".credentials.json"), account)
	}
	return ws
}

// The first workspace's login becomes the default; a second account gets a
// profile named after its workspace; nothing is overwritten.
func TestAdoptKeepsEveryLogin(t *testing.T) {
	s := Store{Root: filepath.Join(t.TempDir(), "profiles")}
	personal := workspace(t, "agents-ws", "personal account")
	got, err := s.Adopt(personal, connected)
	if err != nil || len(got) != 1 || got[0] != (Profile{Name: Default, Harness: "claude-code", Connected: true}) {
		t.Fatalf("first: %+v %v", got, err)
	}
	login := filepath.Join(s.Dir(Default), ".credentials.json")
	if read(t, login) != "personal account" {
		t.Error("the login isn't the default's")
	}
	if info, _ := os.Stat(login); info.Mode().Perm() != 0o600 {
		t.Errorf("mode %v", info.Mode().Perm())
	}
	if _, err := os.Stat(filepath.Join(personal, ".foreman", "credentials", "claude-code")); !os.IsNotExist(err) {
		t.Errorf("the workspace's own login is still there: %v", err)
	}
	if again, err := s.Adopt(personal, connected); err != nil || len(again) != 0 {
		t.Errorf("again: %+v %v", again, err)
	}
	for _, want := range []string{"my-work", "my-work-2"} {
		got, err := s.Adopt(workspace(t, "My Work", "account "+want), connected)
		if err != nil || len(got) != 1 || got[0].Name != want {
			t.Fatalf("got %+v %v, want %s", got, err, want)
		}
		if read(t, filepath.Join(s.Dir(want), ".credentials.json")) != "account "+want {
			t.Errorf("%s's login", want)
		}
	}
	if read(t, login) != "personal account" {
		t.Error("the default's login was overwritten")
	}
	if got := names(t, s); !slices.Equal(got, []string{"default:claude-code", "my-work:claude-code", "my-work-2:claude-code"}) {
		t.Errorf("profiles %v", got)
	}
}

// A default made before any login (Foreman's own, at an earlier start) takes
// the first login moved in; a folder without a login stays where it is.
func TestAdoptFillsAnEmptyDefault(t *testing.T) {
	s := Store{Root: filepath.Join(t.TempDir(), "profiles")}
	if err := s.Ensure("claude-code"); err != nil {
		t.Fatal(err)
	}
	ws := workspace(t, "ws", "")
	write(t, filepath.Join(ws, ".foreman", "credentials", "claude-code", ".claude.json"), "{}")
	if got, err := s.Adopt(ws, connected); err != nil || len(got) != 0 {
		t.Errorf("no login: %+v %v", got, err)
	}
	if _, err := os.Stat(filepath.Join(ws, ".foreman", "credentials", "claude-code", ".claude.json")); err != nil {
		t.Error("a folder without a login moved")
	}
	got, err := s.Adopt(workspace(t, "ws2", "account"), connected)
	if err != nil || len(got) != 1 || got[0].Name != Default {
		t.Fatalf("%+v %v", got, err)
	}
	if p, err := s.Get(Default); err != nil || p.Harness != "claude-code" || read(t, filepath.Join(s.Dir(Default), ".credentials.json")) != "account" {
		t.Errorf("default %+v %v", p, err)
	}
}

// Across file systems a move is a copy: modes and links kept.
func TestCopyTree(t *testing.T) {
	src := filepath.Join(t.TempDir(), "claude-code")
	write(t, filepath.Join(src, ".credentials.json"), "secret")
	write(t, filepath.Join(src, "projects", "a.jsonl"), "{}")
	if err := os.Chmod(filepath.Join(src, "projects", "a.jsonl"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(".credentials.json", filepath.Join(src, "link")); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(t.TempDir(), "copy")
	if err := copyTree(src, dst); err != nil {
		t.Fatal(err)
	}
	for path, mode := range map[string]os.FileMode{".credentials.json": 0o600, "projects/a.jsonl": 0o644, "projects": 0o700} {
		if info, err := os.Stat(filepath.Join(dst, path)); err != nil || info.Mode().Perm() != mode {
			t.Errorf("%s: %v %v", path, info, err)
		}
	}
	if target, err := os.Readlink(filepath.Join(dst, "link")); err != nil || target != ".credentials.json" {
		t.Errorf("link %q %v", target, err)
	}
}

func TestProfileName(t *testing.T) {
	for in, want := range map[string]string{
		"agents-ws": "agents-ws", "My Work": "my-work", "  Side__Projects!! ": "side-projects",
		"real": "real", "default": "workspace", "Ñ": "workspace", "2026": "2026",
		"a very long workspace folder name indeed": "a-very-long-workspace-folder-nam",
	} {
		if got := profileName(in); got != want || !Valid(got) {
			t.Errorf("%q: %q, want %q", in, got, want)
		}
	}
}
