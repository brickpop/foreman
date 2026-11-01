package server

import (
	"net/http"
	"path/filepath"
	"testing"

	"github.com/brickpop/foreman/internal/userdata"
)

func TestPreferences(t *testing.T) {
	file := userdata.PreferencesIn(t.TempDir())
	h := New(Options{Guard: openGuard{}, Home: fakeHome("/home/ceo/.foreman"), Preferences: file, UI: builtUI})
	var got userdata.Preferences
	if decodeBody(t, get(t, h, "/api/preferences"), &got); got != userdata.DefaultPreferences() {
		t.Errorf("before any: %+v", got)
	}
	if w := send(h, "PUT", "/api/preferences", `{"theme":"dark"}`); w.Code != http.StatusOK {
		t.Fatalf("put: %d %s", w.Code, w.Body)
	}
	if w := send(h, "PUT", "/api/preferences", `{"accent":"teal"}`); w.Code != http.StatusOK {
		t.Fatalf("put the accent: %d %s", w.Code, w.Body)
	}
	// Each setting is saved on its own: the font leaves the theme and accent alone.
	if w := send(h, "PUT", "/api/preferences", `{"terminalFont":"jetbrains-mono","terminalFontSize":15}`); w.Code != http.StatusOK {
		t.Fatalf("put the font: %d %s", w.Code, w.Body)
	}
	// What another home's Foreman would read.
	want := userdata.Preferences{Theme: "dark", Accent: "teal", TerminalFont: "jetbrains-mono", TerminalFontSize: 15}
	if p := userdata.PreferencesIn(filepath.Dir(file.Path)).Get(); p != want {
		t.Errorf("kept %+v", p)
	}
	for _, body := range []string{`{"theme":"neon"}`, `{"accent":"neon"}`, `{"terminalFont":"comic-sans"}`, `{"terminalFontSize":9}`, `{"terminalFontSize":25}`, `nope`} {
		if w := send(h, "PUT", "/api/preferences", body); w.Code != http.StatusBadRequest {
			t.Errorf("%s: %d", body, w.Code)
		}
	}
	if p := file.Get(); p != want {
		t.Errorf("a refused put changed %+v", p)
	}
}
