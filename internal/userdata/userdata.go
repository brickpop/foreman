// Package userdata is what Foreman keeps for the CEO on this machine outside
// the global repo (Settings → Global): the profiles, the preferences and the
// agent images' build state. It lives in Foreman's home, ~/.foreman, on macOS
// and Linux alike (the CEO, 2026-10-20), which also holds the lock that keeps
// Foreman to one server per machine
// (wiki/backlog/epics/projects-anywhere.md).
package userdata

import (
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
