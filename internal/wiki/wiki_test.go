package wiki

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// fixture writes files under a fresh project folder's wiki and returns the
// Wikis reading it as project "p".
func fixture(t *testing.T, files map[string]string) (*Wikis, string) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "p", "project", "wiki")
	files["wiki.toml"] = "spec = \"0.1\"\n"
	write(t, dir, files)
	return New(func(project string) (string, error) {
		if project != "p" {
			return "", errors.New("no such project")
		}
		return dir, nil
	}), dir
}

func write(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, content := range files {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func note(body string) string { return "---\ntype: note\n---\n" + body }

func TestTreeOrderAndFolders(t *testing.T) {
	w, _ := fixture(t, map[string]string{
		"index.md":                  "# Home\n",
		"zeta.md":                   note(""),
		"alpha.md":                  "---\ntype: note\ntitle: The Alpha\n---\n",
		"backlog/index.md":          "# Backlog\n",
		"backlog/002-second.md":     note(""),
		"backlog/001-first.md":      note(""),
		"backlog/epics/one.md":      note(""),
		"backlog/archive/old.md":    note(""),
		"adr/014-settlement-v2.md":  note(""),
		"adr/deep/er/most.md":       note(""),
		"adr/deep/other/sibling.md": note(""),
	})
	tree, err := w.Tree("p")
	if err != nil {
		t.Fatal(err)
	}
	paths := func(nodes []Node) []string {
		var out []string
		for _, n := range nodes {
			out = append(out, n.Path)
		}
		return out
	}
	if got, want := paths(tree), []string{"index.md", "adr", "backlog", "alpha.md", "zeta.md"}; !slices.Equal(got, want) {
		t.Fatalf("top level = %v, want %v", got, want)
	}
	backlog := tree[2]
	if !backlog.Folder || !backlog.HasIndex || backlog.Title != "Backlog" {
		t.Errorf("backlog = %+v, want a folder with an index", backlog)
	}
	// Its own index.md opens the folder: it isn't among its children.
	if got, want := paths(backlog.Children), []string{"backlog/archive", "backlog/epics", "backlog/001-first.md", "backlog/002-second.md"}; !slices.Equal(got, want) {
		t.Errorf("backlog = %v, want %v", got, want)
	}
	if tree[1].HasIndex {
		t.Error("adr has no index.md")
	}
	// Sibling folders deep down both keep their entries.
	deep := tree[1].Children[0]
	if got, want := paths(deep.Children), []string{"adr/deep/er", "adr/deep/other"}; !slices.Equal(got, want) {
		t.Fatalf("adr/deep = %v, want %v", got, want)
	}
	if paths(deep.Children[0].Children)[0] != "adr/deep/er/most.md" || paths(deep.Children[1].Children)[0] != "adr/deep/other/sibling.md" {
		t.Errorf("deep entries lost: %+v", deep.Children)
	}
	if tree[3].Title != "The Alpha" || backlog.Children[2].Title != "001 First" || tree[1].Children[1].Title != "014 Settlement v2" {
		t.Errorf("titles: %q %q %q", tree[3].Title, backlog.Children[2].Title, tree[1].Children[1].Title)
	}
	// Entries carry an empty list, never null, so the UI can walk any node.
	if tree[0].Children == nil {
		t.Error("an entry's children are null")
	}
}

func TestEntry(t *testing.T) {
	w, dir := fixture(t, map[string]string{
		"index.md": "---\nokf_version: \"0.1\"\n---\n# Home\n\nSee [the plan](plan/a.md#goals).\n",
		"plan/a.md": "---\ntype: note\ntitle: Plan A\ntags: [x, y]\nstatus: todo\n---\n" +
			"# Plan A\n\n## Goals\n\n- [ ] first\n- [x] second\n\n## Goals\n\n" +
			"[home](../index.md) [later](not-yet.md) [diagram](img/d.png) ![shot](img/d.png) [contract](c.pdf)\n" +
			"[up](../../../code.go) [web](https://example.com) [here](#goals)\n",
		"plan/img/d.png": "png",
		"plan/c.pdf":     "pdf",
		"other.md":       note("[a](plan/a.md) and [again](/plan/a.md)\n"),
	})
	e, err := w.Entry("p", "plan/a.md")
	if err != nil {
		t.Fatal(err)
	}
	if e.Path != "plan/a.md" || e.Title != "Plan A" || e.Version == "" {
		t.Errorf("entry = %q %q %q", e.Path, e.Title, e.Version)
	}
	if e.Body[:9] != "# Plan A\n" {
		t.Errorf("body starts %q: the frontmatter should be stripped", e.Body[:9])
	}
	if e.Frontmatter["tags"] != "x, y" || e.Frontmatter["status"] != "todo" || e.Frontmatter["type"] != "note" {
		t.Errorf("frontmatter = %v", e.Frontmatter)
	}

	// Headings: ids by the engine's rule, the repeat numbered, lines in the body.
	if len(e.Headings) != 3 || e.Headings[1].ID != "goals" || e.Headings[2].ID != "goals-1" || e.Headings[1].BodyLine != 3 {
		t.Errorf("headings = %+v", e.Headings)
	}
	// Checkboxes: the file line (6 lines of frontmatter before the body) for writes.
	if len(e.Checkboxes) != 2 || e.Checkboxes[0].Line != 11 || e.Checkboxes[0].BodyLine != 5 || !e.Checkboxes[1].Done {
		t.Errorf("checkboxes = %+v", e.Checkboxes)
	}

	byRaw := map[string]Link{}
	for _, l := range e.Links {
		byRaw[l.Raw] = l
	}
	for raw, want := range map[string]Link{
		"../index.md":      {Raw: "../index.md", To: "index.md", Exists: true},
		"not-yet.md":       {Raw: "not-yet.md", To: "plan/not-yet.md"},
		"img/d.png":        {Raw: "img/d.png", To: "plan/img/d.png", File: true, Image: true},
		"c.pdf":            {Raw: "c.pdf", To: "plan/c.pdf", File: true},
		"../../../code.go": {Raw: "../../../code.go", Outside: true},
	} {
		if got := byRaw[raw]; got != want {
			t.Errorf("link %q = %+v, want %+v", raw, got, want)
		}
	}
	if _, ok := byRaw["https://example.com"]; ok {
		t.Error("an external link is left to the renderer, not looked up")
	}

	// One backlink per linking entry, named by its title.
	if got := e.Backlinks; len(got) != 2 || got[0].From != "index.md" || got[1].From != "other.md" || got[1].Title != "Other" {
		t.Errorf("backlinks = %+v", got)
	}
	home, _ := w.Entry("p", "index.md")
	if len(home.Frontmatter) != 0 {
		t.Errorf("the root index shows %v: okf_version is the format's, not a field", home.Frontmatter)
	}
	if l := home.Links[0]; l.To != "plan/a.md" || l.Anchor != "goals" || !l.Exists {
		t.Errorf("anchored link = %+v", l)
	}

	// The version follows the content.
	write(t, dir, map[string]string{"plan/a.md": "---\ntype: note\n---\nchanged\n"})
	if again, _ := w.Entry("p", "plan/a.md"); again.Version == e.Version {
		t.Error("the version didn't change with the file")
	}
}

func TestNotFound(t *testing.T) {
	w, dir := fixture(t, map[string]string{"index.md": "# Home\n"})
	write(t, filepath.Dir(dir), map[string]string{"secret.md": note("")})
	for _, p := range []string{"missing.md", "../secret.md", "/../secret.md", "%2e%2e/secret.md", "index", ""} {
		if _, err := w.Entry("p", p); !errors.Is(err, ErrNotFound) {
			t.Errorf("Entry(%q) = %v, want not found", p, err)
		}
	}
	if _, err := w.Tree("nope"); !errors.Is(err, ErrNotFound) {
		t.Errorf("an unknown project: %v", err)
	}

	// A project without wiki.toml doesn't borrow one from a folder above it.
	outer := t.TempDir()
	write(t, outer, map[string]string{"wiki.toml": "spec = \"0.1\"\n", "index.md": "# Outer\n"})
	inner := filepath.Join(outer, "p", "project", "wiki")
	write(t, inner, map[string]string{"index.md": "# Inner\n"})
	bare := New(func(string) (string, error) { return inner, nil })
	if _, err := bare.Tree("p"); !errors.Is(err, ErrNotFound) {
		t.Errorf("a wiki without wiki.toml: %v", err)
	}
}

func TestImage(t *testing.T) {
	w, dir := fixture(t, map[string]string{
		"index.md": "![a](a.png) ![b](B.SVG) [page](page.html) ![dir](folder.png) " +
			"![link](link.png) ![nested](img/c.webp) ![out](../../outside.png)\n",
		"a.png":        "png",
		"B.SVG":        "<svg/>",
		"page.html":    "<script></script>",
		"unlinked.png": "png",
		"img/c.webp":   "webp",
		"folder.png/x": "",
	})
	write(t, filepath.Dir(filepath.Dir(dir)), map[string]string{"outside.png": "png"})
	if err := os.Symlink(filepath.Join(filepath.Dir(filepath.Dir(dir)), "outside.png"), filepath.Join(dir, "link.png")); err != nil {
		t.Fatal(err)
	}
	for p, want := range map[string]string{"a.png": "image/png", "B.SVG": "image/svg+xml", "img/c.webp": "image/webp"} {
		file, ct, err := w.Image("p", p)
		if err != nil || ct != want || filepath.Base(file) != filepath.Base(p) {
			t.Errorf("Image(%q) = %q %q %v, want %s", p, file, ct, err, want)
		}
	}
	// Not an image, not linked, a folder, a symlink out of the wiki, or above it.
	for _, p := range []string{"page.html", "unlinked.png", "folder.png", "link.png", "../../outside.png", "index.md", "wiki.toml"} {
		if _, _, err := w.Image("p", p); !errors.Is(err, ErrNotFound) {
			t.Errorf("Image(%q) = %v, want not found", p, err)
		}
	}
}
