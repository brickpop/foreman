package userdata

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func put(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func has(t *testing.T, path, want string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil || string(data) != want {
		t.Errorf("%s: %q %v, want %q", path, data, err, want)
	}
}

func TestMoveInBringsTheOldFolderOnce(t *testing.T) {
	old, home := filepath.Join(t.TempDir(), "share", "foreman"), filepath.Join(t.TempDir(), ".foreman")
	put(t, filepath.Join(old, "profiles", "default", ".credentials.json"), "login")
	put(t, filepath.Join(old, "images", "state.json"), "built")
	put(t, filepath.Join(old, "preferences.toml"), "theme = 'dark'")
	put(t, filepath.Join(old, "notes.txt"), "not Foreman's")
	if err := moveIn(old, home); err != nil {
		t.Fatal(err)
	}
	has(t, filepath.Join(home, "profiles", "default", ".credentials.json"), "login")
	has(t, filepath.Join(home, "images", "state.json"), "built")
	has(t, filepath.Join(home, "preferences.toml"), "theme = 'dark'")
	// Only what Foreman kept moves: the folder stays with the rest.
	has(t, filepath.Join(old, "notes.txt"), "not Foreman's")
	if _, err := os.Stat(filepath.Join(home, "notes.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Error("a stranger's file moved")
	}
	if info, err := os.Stat(home); err != nil || info.Mode().Perm() != 0o700 {
		t.Errorf("home %v %v", info, err)
	}
	// Again: nothing left to move, nothing breaks.
	if err := moveIn(old, home); err != nil {
		t.Error(err)
	}
}

// What home has already keeps its place; a folder gets the entries it lacks.
func TestMoveInNeverOverwrites(t *testing.T) {
	old, home := filepath.Join(t.TempDir(), "foreman"), filepath.Join(t.TempDir(), ".foreman")
	put(t, filepath.Join(old, "profiles", "default", "login"), "old default")
	put(t, filepath.Join(old, "profiles", "work", "login"), "old work")
	put(t, filepath.Join(old, "preferences.toml"), "old")
	put(t, filepath.Join(home, "profiles", "default", "login"), "new default")
	put(t, filepath.Join(home, "preferences.toml"), "new")
	if err := os.MkdirAll(filepath.Join(home, "images"), 0o755); err != nil { // empty: taken over
		t.Fatal(err)
	}
	put(t, filepath.Join(old, "images", "state.json"), "built")
	if err := moveIn(old, home); err != nil {
		t.Fatal(err)
	}
	has(t, filepath.Join(home, "profiles", "default", "login"), "new default")
	has(t, filepath.Join(home, "profiles", "work", "login"), "old work")
	has(t, filepath.Join(home, "preferences.toml"), "new")
	has(t, filepath.Join(home, "images", "state.json"), "built")
	// What wasn't moved is still there, for the CEO to look at.
	has(t, filepath.Join(old, "profiles", "default", "login"), "old default")
	has(t, filepath.Join(old, "preferences.toml"), "old")
}

func TestMoveInWithoutAnOldFolder(t *testing.T) {
	home := filepath.Join(t.TempDir(), ".foreman")
	if err := moveIn(filepath.Join(t.TempDir(), "none"), home); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(home); !errors.Is(err, os.ErrNotExist) {
		t.Error("home made for nothing")
	}
}

func TestTheLockKeepsOneServer(t *testing.T) {
	home := t.TempDir()
	first, err := TakeLock(home)
	if err != nil {
		t.Fatal(err)
	}
	if err := first.Announce("http://127.0.0.1:8888/?token=abc", "/Users/ceo/dev"); err != nil {
		t.Fatal(err)
	}
	_, err = TakeLock(home)
	var running *RunningError
	if !errors.As(err, &running) || running.URL != "http://127.0.0.1:8888/?token=abc" || running.Workspace != "/Users/ceo/dev" {
		t.Fatalf("a second lock: %v %+v", err, running)
	}
	if info, err := os.Stat(filepath.Join(home, "server.lock")); err != nil || info.Mode().Perm() != 0o600 {
		t.Errorf("the lock holds the token: %v %v", info, err)
	}
	if err := first.Release(); err != nil {
		t.Fatal(err)
	}
	again, err := TakeLock(home)
	if err != nil {
		t.Fatalf("after a release: %v", err)
	}
	defer again.Release()
	// A shorter URL leaves nothing of the longer one behind.
	if err := again.Announce("http://x/", "/w"); err != nil {
		t.Fatal(err)
	}
	if _, err := TakeLock(home); !errors.As(err, &running) || running.URL != "http://x/" || running.Workspace != "/w" {
		t.Errorf("%+v", running)
	}
}

// A Foreman killed outright leaves no lock behind.
func TestALockDiesWithItsProcess(t *testing.T) {
	if os.Getenv("FOREMAN_HOLD_LOCK") != "" {
		if _, err := TakeLock(os.Getenv("FOREMAN_HOLD_LOCK")); err != nil {
			os.Exit(2)
		}
		os.Stdout.WriteString("held\n")
		time.Sleep(time.Minute)
		return
	}
	home := t.TempDir()
	cmd := exec.Command(os.Args[0], "-test.run=^TestALockDiesWithItsProcess$")
	cmd.Env = append(os.Environ(), "FOREMAN_HOLD_LOCK="+home)
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 5)
	if _, err := out.Read(buf); err != nil || !strings.HasPrefix(string(buf), "held") {
		t.Fatalf("the holder: %q %v", buf, err)
	}
	var running *RunningError
	if _, err := TakeLock(home); !errors.As(err, &running) {
		t.Fatalf("while held elsewhere: %v", err)
	}
	_ = cmd.Process.Kill()
	_ = cmd.Wait()
	l, err := TakeLock(home)
	if err != nil {
		t.Fatalf("after the holder was killed: %v", err)
	}
	_ = l.Release()
}
