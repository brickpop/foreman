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
	// The accent: one of the UI's, kept; another refused.
	green := dark
	green.Accent = "green"
	if err := f.Set(green); err != nil || f.Get().Accent != "green" {
		t.Errorf("green: %v, %+v", err, f.Get())
	}
	for _, bad := range []string{"", "Blue", "#ff0000"} {
		odd := dark
		odd.Accent = bad
		if err := f.Set(odd); !errors.Is(err, ErrInvalid) {
			t.Errorf("accent %q: %v", bad, err)
		}
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
	edit := "theme = \"light\"\naccent = \"chartreuse\"\nterminal_font = \"comic-sans\"\nterminal_font_size = 99\n"
	if err := os.WriteFile(f.Path, []byte(edit), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := f.Get(); got != (Preferences{Theme: "light", Accent: "blue", TerminalFont: "jetbrains-mono", TerminalFontSize: 13}) {
		t.Errorf("one bad value: %+v", got)
	}
	// A file from before the terminal settings has their defaults.
	if err := os.WriteFile(f.Path, []byte(`theme = "dark"`), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := f.Get(); got != (Preferences{Theme: "dark", Accent: "blue", TerminalFont: "jetbrains-mono", TerminalFontSize: 13}) {
		t.Errorf("an older file: %+v", got)
	}
	// A font no longer offered (Geist Mono, IBM Plex Mono until 2026-10-20) reads as the default.
	if err := os.WriteFile(f.Path, []byte("terminal_font = \"geist-mono\"\nterminal_font_size = 15\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := f.Get(); got != (Preferences{Theme: "system", Accent: "blue", TerminalFont: "jetbrains-mono", TerminalFontSize: 15}) {
		t.Errorf("a retired font: %+v", got)
	}
	for _, bad := range []Preferences{
		{Theme: "dark", TerminalFont: "comic-sans", TerminalFontSize: 13},
		{Theme: "dark", TerminalFont: "ibm-plex-mono", TerminalFontSize: 13},
		{Theme: "dark", TerminalFont: "system", TerminalFontSize: MinFontSize - 1},
		{Theme: "dark", TerminalFont: "system", TerminalFontSize: MaxFontSize + 1},
	} {
		if err := f.Set(bad); !errors.Is(err, ErrInvalid) {
			t.Errorf("%+v: %v", bad, err)
		}
	}
}
