package server

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/brickpop/foreman/internal/wiki"
)

func TestWiki(t *testing.T) {
	dir := t.TempDir()
	for name, content := range map[string]string{
		"wiki.toml":    "spec = \"0.1\"\n",
		"index.md":     "# Home\n\n![d](img/d.svg) ![p](p.png)\n",
		"backlog/x.md": "---\ntype: task\n---\n- [ ] one\n",
		"img/d.svg":    "<svg/>",
		"p.png":        "png",
		"unlinked.png": "png",
	} {
		p := filepath.Join(dir, filepath.FromSlash(name))
		_ = os.MkdirAll(filepath.Dir(p), 0o755)
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	wk := wiki.New(func(project string) (string, error) {
		if project != "ledger" {
			return "", errors.New("no such project")
		}
		return dir, nil
	})
	h := New(Options{Guard: openGuard{}, Workspace: fakeWorkspace("/ws"), Wiki: wk, UI: builtUI})

	var tree []wiki.Node
	decodeBody(t, get(t, h, "/api/wiki/ledger/tree"), &tree)
	if len(tree) != 2 || tree[0].Path != "index.md" || tree[1].Path != "backlog" {
		t.Errorf("tree %+v", tree)
	}
	var e wiki.Entry
	decodeBody(t, get(t, h, "/api/wiki/ledger/entry/backlog/x.md"), &e)
	if e.Path != "backlog/x.md" || len(e.Checkboxes) != 1 || e.Checkboxes[0].Line != 4 {
		t.Errorf("entry %+v", e)
	}

	svg := send(h, "GET", "/api/wiki/ledger/image/img/d.svg", "")
	if svg.Code != http.StatusOK || svg.Header().Get("Content-Type") != "image/svg+xml" ||
		svg.Header().Get("Content-Security-Policy") != svgPolicy || svg.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Errorf("svg: %d %v", svg.Code, svg.Header())
	}
	png := send(h, "GET", "/api/wiki/ledger/image/p.png", "")
	if png.Code != http.StatusOK || png.Body.String() != "png" || png.Header().Get("Content-Security-Policy") != "" {
		t.Errorf("png: %d %v %q", png.Code, png.Header(), png.Body)
	}

	for _, path := range []string{
		"/api/wiki/ledger/entry/missing.md",
		"/api/wiki/other/tree",
		"/api/wiki/ledger/image/unlinked.png",
		"/api/wiki/ledger/image/index.md",
		"/api/wiki/ledger/image/..%2f..%2fetc%2fpasswd",
	} {
		if w := send(h, "GET", path, ""); w.Code != http.StatusNotFound {
			t.Errorf("%s: %d %s, want 404", path, w.Code, w.Body)
		}
	}
}
