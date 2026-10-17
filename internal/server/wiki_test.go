package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/brickpop/foreman/internal/git"
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
	}, nil)
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

func TestWikiWrites(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "project", "wiki")
	for name, content := range map[string]string{
		"wiki.toml":    "spec = \"0.1\"\n\n[[tool.wikiview.board]]\nid = \"b\"\npath = \"/backlog\"\ncolumns = [\"todo\", \"done\"]\n",
		"backlog/x.md": "---\ntype: task\nstatus: todo\n---\n- [ ] one\n",
	} {
		p := filepath.Join(dir, filepath.FromSlash(name))
		_ = os.MkdirAll(filepath.Dir(p), 0o755)
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	g := git.New()
	if err := g.Init(context.Background(), filepath.Dir(dir), "main"); err != nil {
		t.Fatal(err)
	}
	if err := g.Commit(context.Background(), dir, "Wiki", "."); err != nil {
		t.Fatal(err)
	}
	wk := wiki.New(func(string) (string, error) { return dir, nil }, g)
	h := New(Options{Guard: openGuard{}, Workspace: fakeWorkspace("/ws"), Wiki: wk, UI: builtUI})

	var boards []wiki.Board
	decodeBody(t, get(t, h, "/api/wiki/ledger/boards"), &boards)
	if len(boards) != 1 || len(boards[0].Cards) != 1 || boards[0].Cards[0].Column != "todo" {
		t.Fatalf("boards %+v", boards)
	}
	version := boards[0].Cards[0].Version

	if w := send(h, "PUT", "/api/wiki/ledger/checkbox/backlog/x.md", `{"line":5,"done":true,"version":"`+version+`"}`); w.Code != http.StatusNoContent {
		t.Fatalf("tick: %d %s", w.Code, w.Body)
	}
	var e wiki.Entry
	decodeBody(t, get(t, h, "/api/wiki/ledger/entry/backlog/x.md"), &e)
	if !e.Checkboxes[0].Done {
		t.Errorf("not ticked: %+v", e.Checkboxes)
	}
	if w := send(h, "PUT", "/api/wiki/ledger/card/b/backlog/x.md", `{"value":"done","version":"`+e.Version+`"}`); w.Code != http.StatusNoContent {
		t.Fatalf("move: %d %s", w.Code, w.Body)
	}

	for _, c := range []struct{ method, path, body, code string }{
		{"PUT", "/api/wiki/ledger/checkbox/backlog/x.md", `{"line":5,"done":false,"version":"` + version + `"}`, "CONFLICT"},
		{"PUT", "/api/wiki/ledger/card/b/backlog/x.md", `{"value":"todo","version":"` + version + `"}`, "CONFLICT"},
		{"PUT", "/api/wiki/ledger/card/b/backlog/x.md", `{"value":"","version":"x"}`, "INVALID"},
		{"PUT", "/api/wiki/ledger/card/nope/backlog/x.md", `{"value":"todo","version":"x"}`, "NOT_FOUND"},
		{"PUT", "/api/wiki/ledger/checkbox/backlog/x.md", `nope`, "BAD_REQUEST"},
	} {
		w := send(h, c.method, c.path, c.body)
		var got map[string]string
		_ = json.Unmarshal(w.Body.Bytes(), &got)
		if got["code"] != c.code {
			t.Errorf("%s %s: %d %s, want %s", c.path, c.body, w.Code, w.Body, c.code)
		}
	}
}
