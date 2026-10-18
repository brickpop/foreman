package userdata

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"

	"github.com/BurntSushi/toml"
)

// ErrInvalid means preferences that can't be kept.
var ErrInvalid = errors.New("userdata: invalid preferences")

var themes = []string{"system", "dark", "light"}

// DefaultPreferences are the preferences before any is set.
func DefaultPreferences() Preferences { return Preferences{Theme: "system"} }

// PreferencesIn is preferences.toml in the CEO's folder.
func PreferencesIn(dir string) PreferencesFile {
	return PreferencesFile{Path: filepath.Join(dir, "preferences.toml")}
}

// PreferencesFile is the preferences' file.
type PreferencesFile struct{ Path string }

// Get reads the preferences over the defaults; a missing or unreadable file
// is the defaults (a hand edit gone wrong never stops the UI).
func (f PreferencesFile) Get() Preferences {
	p := DefaultPreferences()
	if _, err := toml.DecodeFile(f.Path, &p); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return DefaultPreferences()
	}
	if !slices.Contains(themes, p.Theme) {
		p.Theme = DefaultPreferences().Theme
	}
	return p
}

// Set checks and writes the preferences.
func (f PreferencesFile) Set(p Preferences) error {
	if !slices.Contains(themes, p.Theme) {
		return fmt.Errorf("%w: theme %q is not one of system, dark, light", ErrInvalid, p.Theme)
	}
	var buf bytes.Buffer
	if err := toml.NewEncoder(&buf).Encode(p); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(f.Path), 0o700); err != nil {
		return err
	}
	// Write then rename: another Foreman may be reading it.
	tmp := f.Path + ".tmp"
	if err := os.WriteFile(tmp, buf.Bytes(), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, f.Path)
}
