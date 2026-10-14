package workspace

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestOpenRequiresForemanDir(t *testing.T) {
	dir := t.TempDir()
	if _, err := Open(dir); !errors.Is(err, ErrNotInitialized) {
		t.Fatalf("Open on a plain folder: got %v, want ErrNotInitialized", err)
	}
	if _, err := Init(dir); err != nil {
		t.Fatal(err)
	}
	ws, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(ws.Root()) {
		t.Errorf("Root %q is not absolute", ws.Root())
	}
}

func TestOpenRejectsFilesAndMissingFolders(t *testing.T) {
	dir := t.TempDir()
	if _, err := Open(filepath.Join(dir, "missing")); err == nil {
		t.Error("Open on a missing folder succeeded")
	}
	file := filepath.Join(dir, "file")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(file); err == nil {
		t.Error("Open on a file succeeded")
	}
	// A file named .foreman is not a workspace either.
	if err := os.WriteFile(filepath.Join(dir, DirName), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(dir); err == nil || errors.Is(err, ErrNotInitialized) {
		t.Errorf("Open with a .foreman file: got %v, want a distinct error", err)
	}
}

func TestInitTwiceFails(t *testing.T) {
	dir := t.TempDir()
	if _, err := Init(dir); err != nil {
		t.Fatal(err)
	}
	if _, err := Init(dir); err == nil {
		t.Error("second Init succeeded; it must never clobber an existing workspace")
	}
}

func TestInitCreatesPrivateFolder(t *testing.T) {
	ws, err := Init(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(ws.Path())
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm&0o077 != 0 {
		t.Errorf(".foreman permissions %o: readable by others", perm)
	}
}

func TestAccessTokenIsStableAndPrivate(t *testing.T) {
	ws, err := Init(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	first, err := ws.AccessToken(false)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) < 40 {
		t.Errorf("token %q is too short to be unguessable", first)
	}
	again, err := ws.AccessToken(false)
	if err != nil {
		t.Fatal(err)
	}
	if again != first {
		t.Error("token changed between starts without rotation")
	}
	info, err := os.Stat(ws.Path(tokenFile))
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("token file permissions %o, want 600", perm)
	}
}

func TestAccessTokenRotation(t *testing.T) {
	ws, err := Init(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	first, _ := ws.AccessToken(false)
	rotated, err := ws.AccessToken(true)
	if err != nil {
		t.Fatal(err)
	}
	if rotated == first {
		t.Error("rotation kept the old token")
	}
	after, _ := ws.AccessToken(false)
	if after != rotated {
		t.Error("rotated token was not persisted")
	}
}

func TestAccessTokenRegeneratedWhenEmpty(t *testing.T) {
	ws, err := Init(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ws.Path(tokenFile), []byte("\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	token, err := ws.AccessToken(false)
	if err != nil {
		t.Fatal(err)
	}
	if token == "" {
		t.Error("an empty token file produced an empty token")
	}
}
