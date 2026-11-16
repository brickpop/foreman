// Package userdata finds Foreman's home on this machine, ~/.foreman, on
// macOS and Linux alike (the CEO, 2026-10-20), and holds the lock that keeps
// Foreman to one server per machine (wiki/backlog/epics/projects-anywhere.md).
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
