package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/brickpop/foreman/internal/desktop"
	"github.com/brickpop/foreman/internal/home"
	"github.com/brickpop/foreman/internal/store"
)

// fakeDesktop records the folders it opened; without a desktop, or for a
// folder named in refuse, it refuses as desktop.Opener does.
type fakeDesktop struct {
	can    bool
	refuse string
	opened []string
}

func (f *fakeDesktop) CanOpen() bool { return f.can }
func (f *fakeDesktop) Open(dir string) error {
	switch {
	case !f.can:
		return desktop.ErrUnavailable
	case dir == f.refuse:
		return fmt.Errorf("%w: %s", desktop.ErrNotAFolder, dir)
	}
	f.opened = append(f.opened, dir)
	return nil
}

func withDesktop(d *fakeDesktop, files string) http.Handler {
	agents := &fakeAgents{list: []store.Instance{{ID: "tove@ledger", SandboxPath: "/ws/ledger/agents/tove"}}}
	config := fakeConfig{projects: []home.Project{{ID: "ledger", Files: files}, {ID: "legacy"}}}
	return New(Options{Guard: openGuard{}, Home: fakeHome("/home/ceo/.foreman"), Agents: agents, Config: config, Desktop: d, UI: builtUI})
}

// The UI asks once whether to show Open buttons at all.
func TestDesktopSaysWhetherItCanOpen(t *testing.T) {
	for _, can := range []bool{true, false} {
		var got map[string]bool
		decodeBody(t, get(t, withDesktop(&fakeDesktop{can: can}, ""), "/api/desktop"), &got)
		if got["canOpen"] != can {
			t.Errorf("canOpen %v, want %v", got, can)
		}
	}
}

// The server opens the folder it works out from an instance, or from a
// project and one of its file folders (made first, so the inbox opens before
// any agent has started); the browser never names a path, so no other
// folder can be opened.
func TestOpenFolders(t *testing.T) {
	files := filepath.Join(t.TempDir(), "files")
	d := &fakeDesktop{can: true}
	h := withDesktop(d, files)
	for _, body := range []string{
		`{"instance":"tove@ledger"}`,
		`{"project":"ledger","folder":"inbox"}`,
		`{"project":"ledger","folder":"shared"}`,
	} {
		if w := send(h, "POST", "/api/desktop/open", body); w.Code != http.StatusNoContent {
			t.Fatalf("%s: %d %s", body, w.Code, w.Body)
		}
	}
	want := []string{"/ws/ledger/agents/tove", filepath.Join(files, "inbox"), filepath.Join(files, "shared")}
	if !slices.Equal(d.opened, want) {
		t.Errorf("opened %v, want %v", d.opened, want)
	}
	for _, f := range home.FileFolders {
		if _, err := os.Stat(filepath.Join(files, f)); err != nil {
			t.Errorf("%s wasn't made: %v", f, err)
		}
	}
}

// Every request that doesn't name exactly one known folder is refused, and
// nothing is opened.
func TestOpenRefuses(t *testing.T) {
	files := filepath.Join(t.TempDir(), "files")
	for _, tc := range []struct {
		name, body string
		desk       *fakeDesktop
		code       int
		errCode    string
	}{
		{"no desktop", `{"instance":"tove@ledger"}`, &fakeDesktop{}, http.StatusConflict, "NO_DESKTOP"},
		{"unknown instance", `{"instance":"nope@ledger"}`, &fakeDesktop{can: true}, http.StatusNotFound, "NOT_FOUND"},
		{"unknown project", `{"project":"nope","folder":"inbox"}`, &fakeDesktop{can: true}, http.StatusNotFound, "NOT_FOUND"},
		{"another folder", `{"project":"ledger","folder":"../project"}`, &fakeDesktop{can: true}, http.StatusBadRequest, "INVALID"},
		{"no folder", `{"project":"ledger"}`, &fakeDesktop{can: true}, http.StatusBadRequest, "INVALID"},
		{"both", `{"instance":"tove@ledger","project":"ledger","folder":"inbox"}`, &fakeDesktop{can: true}, http.StatusBadRequest, "INVALID"},
		{"neither", `{}`, &fakeDesktop{can: true}, http.StatusBadRequest, "INVALID"},
		{"a path", `{"path":"/etc"}`, &fakeDesktop{can: true}, http.StatusBadRequest, "INVALID"},
		{"not JSON", `nope`, &fakeDesktop{can: true}, http.StatusBadRequest, "INVALID"},
		{"legacy project", `{"project":"legacy","folder":"inbox"}`, &fakeDesktop{can: true}, http.StatusConflict, "NO_FILES"},
		{"not a folder yet", `{"instance":"tove@ledger"}`, &fakeDesktop{can: true, refuse: "/ws/ledger/agents/tove"}, http.StatusConflict, "NOT_A_FOLDER"},
	} {
		w := send(withDesktop(tc.desk, files), "POST", "/api/desktop/open", tc.body)
		var got struct{ Code string }
		_ = json.Unmarshal(w.Body.Bytes(), &got)
		if w.Code != tc.code || got.Code != tc.errCode || len(tc.desk.opened) != 0 {
			t.Errorf("%s: %d %s, opened %v; want %d %s", tc.name, w.Code, w.Body, tc.desk.opened, tc.code, tc.errCode)
		}
	}
}
