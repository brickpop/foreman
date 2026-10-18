// Package userdata is what Foreman keeps for the CEO on this machine, the
// same for every workspace (Settings → Global): the profiles, the
// preferences, and the agent images' build state. It lives in one folder,
// ~/.local/share/foreman, on macOS and Linux alike (the CEO, 2026-10-18), or
// $XDG_DATA_HOME/foreman when that is set (absolute).
package userdata

import (
	"os"
	"path/filepath"
)

// Dir is the folder.
func Dir() (string, error) {
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
