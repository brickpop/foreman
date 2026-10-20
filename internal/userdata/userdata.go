// Package userdata is what Foreman keeps for the CEO on this machine, the
// same for every workspace (Settings → Global): the profiles, the
// preferences, and the agent images' build state. It lives in Foreman's
// home, ~/.foreman, on macOS and Linux alike (the CEO, 2026-10-20), which
// also holds the lock that keeps Foreman to one server per machine
// (wiki/backlog/epics/projects-anywhere.md).
package userdata

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Dir is Foreman's home on this machine.
func Dir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".foreman"), nil
}

// legacyDir is where an older Foreman kept the same things:
// $XDG_DATA_HOME/foreman when that is set (absolute), else ~/.local/share/foreman.
func legacyDir() (string, error) {
	dir := os.Getenv("XDG_DATA_HOME")
	if !filepath.IsAbs(dir) {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(dir, "foreman"), nil
}

// kept are what the CEO's folder holds, by name.
var kept = []string{"profiles", "images", "preferences.toml"}

// MoveIn moves what an older Foreman kept in its folder into home, once:
// each thing home doesn't have yet (or has only as an empty folder), and into
// a folder home has already, each entry it lacks. What home has is left in the
// old folder, which goes once it's empty.
func MoveIn(home string) error {
	old, err := legacyDir()
	if err != nil {
		return err
	}
	return moveIn(old, home)
}

func moveIn(old, home string) error {
	if _, err := os.Stat(old); errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err := os.MkdirAll(home, 0o700); err != nil {
		return err
	}
	for _, name := range kept {
		if err := move(filepath.Join(old, name), filepath.Join(home, name), true); err != nil {
			return err
		}
	}
	_ = os.Remove(old) // only when empty
	return nil
}

// move moves from to to, unless to has something already: then, when both
// are folders and deeper is true, each entry to lacks (a profile made since,
// say, keeps its place, and the others still come).
func move(from, to string, deeper bool) error {
	info, err := os.Lstat(from)
	if err != nil {
		return nil // nothing to move
	}
	if emptyOrMissing(to) {
		if err := os.Remove(to); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if err := os.Rename(from, to); err != nil {
			return fmt.Errorf("moving %s to %s (move it by hand, then start again): %w", from, to, err)
		}
		return nil
	}
	if dst, err := os.Stat(to); !deeper || !info.IsDir() || err != nil || !dst.IsDir() {
		return nil
	}
	entries, err := os.ReadDir(from)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if err := move(filepath.Join(from, e.Name()), filepath.Join(to, e.Name()), false); err != nil {
			return err
		}
	}
	_ = os.Remove(from) // only when empty
	return nil
}

// emptyOrMissing says whether path doesn't exist, or is an empty folder.
func emptyOrMissing(path string) bool {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return true
	}
	if err != nil || !info.IsDir() {
		return false
	}
	entries, err := os.ReadDir(path)
	return err == nil && len(entries) == 0
}
