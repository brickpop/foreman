package server

import (
	"net/http"
	"path/filepath"
	"testing"

	"github.com/brickpop/foreman/internal/userdata"
)

func TestPreferences(t *testing.T) {
	file := userdata.PreferencesIn(t.TempDir())
	h := New(Options{Guard: openGuard{}, Workspace: fakeWorkspace("/ws"), Preferences: file, UI: builtUI})
	var got userdata.Preferences
	if decodeBody(t, get(t, h, "/api/preferences"), &got); got.Theme != "system" {
		t.Errorf("before any: %+v", got)
	}
	if w := send(h, "PUT", "/api/preferences", `{"theme":"dark"}`); w.Code != http.StatusOK {
		t.Fatalf("put: %d %s", w.Code, w.Body)
	}
	// What another workspace's Foreman would read.
	if p := userdata.PreferencesIn(filepath.Dir(file.Path)).Get(); p.Theme != "dark" {
		t.Errorf("kept %+v", p)
	}
	for body, want := range map[string]int{`{"theme":"neon"}`: http.StatusBadRequest, `nope`: http.StatusBadRequest} {
		if w := send(h, "PUT", "/api/preferences", body); w.Code != want {
			t.Errorf("%s: %d", body, w.Code)
		}
	}
}
