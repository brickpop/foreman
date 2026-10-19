package userdata

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/BurntSushi/toml"
)

// ErrInvalid means preferences that can't be kept.
var ErrInvalid = errors.New("userdata: invalid preferences")

var themes = []string{"system", "dark", "light"}

// TerminalFonts are the terminal's fonts (the UI's terminalFonts lists the
// same ids); system is the operating system's monospace.
var TerminalFonts = []string{"geist-mono", "jetbrains-mono", "ibm-plex-mono", "fira-code", "source-code-pro", "system"}

// The terminal's font sizes, in px.
const (
	MinFontSize = 10
	MaxFontSize = 24
)

// DefaultPreferences are the preferences before any is set.
func DefaultPreferences() Preferences {
	return Preferences{Theme: "system", TerminalFont: "geist-mono", TerminalFontSize: 13}
}

// PreferencesIn is preferences.toml in the CEO's folder.
func PreferencesIn(dir string) PreferencesFile {
	return PreferencesFile{Path: filepath.Join(dir, "preferences.toml")}
}

// PreferencesFile is the preferences' file.
type PreferencesFile struct{ Path string }

// Get reads the preferences over the defaults; a missing or unreadable file
// is the defaults, and a value that can't be kept is its default (a hand edit
// gone wrong never stops the UI).
func (f PreferencesFile) Get() Preferences {
	p, def := DefaultPreferences(), DefaultPreferences()
	if _, err := toml.DecodeFile(f.Path, &p); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return def
	}
	if !slices.Contains(themes, p.Theme) {
		p.Theme = def.Theme
	}
	if !slices.Contains(TerminalFonts, p.TerminalFont) {
		p.TerminalFont = def.TerminalFont
	}
	if p.TerminalFontSize < MinFontSize || p.TerminalFontSize > MaxFontSize {
		p.TerminalFontSize = def.TerminalFontSize
	}
	return p
}

// Set checks and writes the preferences.
func (f PreferencesFile) Set(p Preferences) error {
	switch {
	case !slices.Contains(themes, p.Theme):
		return fmt.Errorf("%w: theme %q is not one of %s", ErrInvalid, p.Theme, strings.Join(themes, ", "))
	case !slices.Contains(TerminalFonts, p.TerminalFont):
		return fmt.Errorf("%w: terminal font %q is not one of %s", ErrInvalid, p.TerminalFont, strings.Join(TerminalFonts, ", "))
	case p.TerminalFontSize < MinFontSize || p.TerminalFontSize > MaxFontSize:
		return fmt.Errorf("%w: terminal font size %d is not %d to %d", ErrInvalid, p.TerminalFontSize, MinFontSize, MaxFontSize)
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
