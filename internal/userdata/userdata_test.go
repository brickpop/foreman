package userdata

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// ~/.foreman on macOS and Linux alike, whatever XDG_DATA_HOME says.
func TestDir(t *testing.T) {
	t.Setenv("HOME", "/home/u")
	t.Setenv("XDG_DATA_HOME", "/data/share")
	if got, err := Dir(); err != nil || got != "/home/u/.foreman" {
		t.Errorf("%s %v", got, err)
	}
}

func TestPreferences(t *testing.T) {
	f := PreferencesFile{Path: filepath.Join(t.TempDir(), "foreman", "preferences.toml")}
	if got := f.Get(); got != DefaultPreferences() {
		t.Errorf("before any: %+v", got)
	}
	dark := DefaultPreferences()
	dark.Theme = "dark"
	if err := f.Set(dark); err != nil {
		t.Fatal(err)
	}
	// Another Foreman reads what this one wrote.
	if got := (PreferencesFile{Path: f.Path}).Get(); got.Theme != "dark" {
		t.Errorf("after: %+v", got)
	}
	neon := dark
	neon.Theme = "neon"
	if err := f.Set(neon); !errors.Is(err, ErrInvalid) {
		t.Errorf("an unknown theme: %v", err)
	}
	if got := f.Get(); got.Theme != "dark" {
		t.Errorf("a refused set changed it: %+v", got)
	}
	// A hand edit gone wrong is the defaults, never an error.
	for _, bad := range []string{`theme = "neon"`, `theme = `, `colour = "red"`} {
		if err := os.WriteFile(f.Path, []byte(bad), 0o600); err != nil {
			t.Fatal(err)
		}
		if got := f.Get(); got != DefaultPreferences() {
			t.Errorf("%q: %+v", bad, got)
		}
	}
	// A value that can't be kept is its default; the others stay.
	edit := "theme = \"light\"\nterminal_font = \"comic-sans\"\nterminal_font_size = 99\n"
	if err := os.WriteFile(f.Path, []byte(edit), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := f.Get(); got != (Preferences{Theme: "light", TerminalFont: "geist-mono", TerminalFontSize: 13}) {
		t.Errorf("one bad value: %+v", got)
	}
	// A file from before the terminal settings has their defaults.
	if err := os.WriteFile(f.Path, []byte(`theme = "dark"`), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := f.Get(); got != (Preferences{Theme: "dark", TerminalFont: "geist-mono", TerminalFontSize: 13}) {
		t.Errorf("an older file: %+v", got)
	}
	for _, bad := range []Preferences{
		{Theme: "dark", TerminalFont: "comic-sans", TerminalFontSize: 13},
		{Theme: "dark", TerminalFont: "system", TerminalFontSize: MinFontSize - 1},
		{Theme: "dark", TerminalFont: "system", TerminalFontSize: MaxFontSize + 1},
	} {
		if err := f.Set(bad); !errors.Is(err, ErrInvalid) {
			t.Errorf("%+v: %v", bad, err)
		}
	}
}
