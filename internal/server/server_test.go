package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

// The server is tested only through its interfaces, with doubles behind them.

type openGuard struct{}

func (openGuard) Wrap(next http.Handler) http.Handler { return next }

type closedGuard struct{}

func (closedGuard) Wrap(http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})
}

type fakeWorkspace string

func (f fakeWorkspace) Root() string { return string(f) }

var builtUI = fstest.MapFS{
	"index.html":    {Data: []byte("<!doctype html><title>Foreman</title>")},
	"assets/app.js": {Data: []byte("console.log('app')")},
}

func get(t *testing.T, h http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
	return w
}

func open(ui fstest.MapFS) http.Handler {
	return New(Options{Guard: openGuard{}, Workspace: fakeWorkspace("/ws"), UI: ui})
}

func TestEveryRouteGoesThroughTheGuard(t *testing.T) {
	h := New(Options{Guard: closedGuard{}, Workspace: fakeWorkspace("/ws"), UI: builtUI})
	for _, path := range []string{"/", "/api/workspace", "/api/instances", "/api/deployments", "/api/projects", "/api/teams/x", "/api/settings", "/api/events", "/ws/pty/x", "/assets/app.js", "/no/such/route"} {
		if w := get(t, h, path); w.Code != http.StatusTeapot {
			t.Errorf("%s bypassed the guard: %d", path, w.Code)
		}
	}
}

func TestWorkspaceSummaryComesFromTheWorkspace(t *testing.T) {
	w := get(t, open(builtUI), "/api/workspace")
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"path":"/ws"`) {
		t.Errorf("got %d %q", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type %q", ct)
	}
}

func TestUIServesFilesAndFallsBackToIndex(t *testing.T) {
	h := open(builtUI)
	if w := get(t, h, "/assets/app.js"); !strings.Contains(w.Body.String(), "console.log") {
		t.Errorf("asset not served: %d %q", w.Code, w.Body.String())
	}
	for _, path := range []string{"/", "/projects/ledger/board", "/etc/passwd"} {
		w := get(t, h, path)
		if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "<title>Foreman</title>") {
			t.Errorf("%s: got %d %q, want index.html", path, w.Code, w.Body.String())
		}
	}
}

// An unknown API path answers with the API's error format, not index.html.
func TestUnknownEndpointsAreAPIErrors(t *testing.T) {
	h := open(builtUI)
	for _, path := range []string{"/api/nothing", "/api/workspace/extra", "/ws/nothing"} {
		w := get(t, h, path)
		if w.Code != http.StatusNotFound || !strings.Contains(w.Body.String(), `"code":"NOT_FOUND"`) {
			t.Errorf("%s: got %d %q", path, w.Code, w.Body.String())
		}
	}
	if w := get(t, h, "/api/workspace"); w.Code != http.StatusOK {
		t.Errorf("a real endpoint: %d", w.Code)
	}
}

// Path traversal never escapes the UI files: the mux redirects to the cleaned
// path, which then resolves inside the UI file system only.
func TestTraversalStaysInsideTheUI(t *testing.T) {
	w := get(t, open(builtUI), "/../../etc/passwd")
	if w.Code == http.StatusOK && !strings.Contains(w.Body.String(), "<title>Foreman</title>") {
		t.Fatalf("traversal served something other than the UI: %q", w.Body.String())
	}
	if w.Code != http.StatusOK {
		if loc := w.Header().Get("Location"); loc != "/etc/passwd" {
			t.Errorf("got %d to %q, want a redirect to the cleaned path", w.Code, loc)
		}
	}
}

func TestUnbuiltUISaysHowToBuildIt(t *testing.T) {
	w := get(t, open(fstest.MapFS{}), "/")
	if w.Code != http.StatusServiceUnavailable || !strings.Contains(w.Body.String(), "just build") {
		t.Errorf("got %d %q", w.Code, w.Body.String())
	}
}
