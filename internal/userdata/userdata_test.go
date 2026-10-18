package userdata

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// The same place on macOS and Linux: ~/.local/share, or XDG_DATA_HOME when
// it is set to an absolute path.
func TestDir(t *testing.T) {
	t.Setenv("HOME", "/home/u")
	for env, want := range map[string]string{
		"":            "/home/u/.local/share/foreman",
		"relative/x":  "/home/u/.local/share/foreman",
		"/data/share": "/data/share/foreman",
	} {
		t.Setenv("XDG_DATA_HOME", env)
		if got, err := Dir(); err != nil || got != want {
			t.Errorf("XDG_DATA_HOME=%q: %s %v, want %s", env, got, err, want)
		}
	}
}

func TestPreferences(t *testing.T) {
	f := PreferencesFile{Path: filepath.Join(t.TempDir(), "foreman", "preferences.toml")}
	if got := f.Get(); got != DefaultPreferences() {
		t.Errorf("before any: %+v", got)
	}
	if err := f.Set(Preferences{Theme: "dark"}); err != nil {
		t.Fatal(err)
	}
	// Another Foreman reads what this one wrote.
	if got := (PreferencesFile{Path: f.Path}).Get(); got.Theme != "dark" {
		t.Errorf("after: %+v", got)
	}
	if err := f.Set(Preferences{Theme: "neon"}); !errors.Is(err, ErrInvalid) {
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
		if got := f.Get(); got.Theme != "system" {
			t.Errorf("%q: %+v", bad, got)
		}
	}
}
