package wiki

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/agentic-wiki/wiki/bundle"
	"github.com/agentic-wiki/wiki/index"
	"github.com/agentic-wiki/wiki/parse"

	"github.com/brickpop/foreman/internal/home"
)

// ErrNotFound means no such project wiki, entry or image.
var ErrNotFound = errors.New("wiki: not found")

// Wikis reads the projects' wikis. While Watch follows them, each wiki's
// index is kept and rebuilt on change; otherwise each read builds it afresh,
// so nothing serves a stale index unnoticed.
type Wikis struct {
	dir func(project string) (string, error)
	git Git

	mu      sync.Mutex
	watched map[string]*watched // by project, while Watch runs
	writing sync.Mutex          // one write at a time: check, change, commit
}

// New reads the wiki that dir names for each project (its bundle folder, the
// one holding wiki.toml), and commits its writes with git.
func New(dir func(project string) (string, error), git Git) *Wikis {
	return &Wikis{dir: dir, git: git, watched: map[string]*watched{}}
}

// refresh rebuilds a followed wiki's index after Foreman's own write, so the
// next read sees it. Only the index: the versions stay, so the watcher still
// finds the change and tells everyone else.
func (w *Wikis) refresh(project string) {
	_, idx, err := w.build(project)
	if err != nil {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if old := w.watched[project]; old != nil {
		now := *old
		now.idx = idx
		w.watched[project] = &now
	}
}

// open is the project wiki's index: the watcher's, else a fresh one.
func (w *Wikis) open(project string) (*index.Index, error) {
	w.mu.Lock()
	cached := w.watched[project]
	w.mu.Unlock()
	if cached != nil && cached.idx != nil {
		return cached.idx, nil
	}
	_, idx, err := w.build(project)
	return idx, err
}

// build reads the project's wiki from disk: its folder and its index.
func (w *Wikis) build(project string) (string, *index.Index, error) {
	dir, err := w.dir(project)
	if err != nil {
		return "", nil, fmt.Errorf("%w: %s", ErrNotFound, project)
	}
	b, err := bundle.Discover(dir)
	// Discover walks up: a wiki.toml above the project's isn't its wiki.
	if err != nil || filepath.Clean(b.Dir) != filepath.Clean(dir) {
		return dir, nil, fmt.Errorf("%w: %s has no wiki", ErrNotFound, project)
	}
	idx, err := index.Build(b)
	return dir, idx, err
}

// Tree is the project wiki's tree, in the reader's order.
func (w *Wikis) Tree(project string) ([]Node, error) {
	idx, err := w.open(project)
	if err != nil {
		return nil, err
	}
	// Built with pointers first, then copied out: a Node holds its children by value.
	type branch struct {
		node     Node
		children []*branch
	}
	root := &branch{}
	folders := map[string]*branch{"": root}
	var folder func(dir string) *branch
	folder = func(dir string) *branch {
		if f, ok := folders[dir]; ok {
			return f
		}
		f := &branch{node: Node{Path: dir, Title: folderTitle(dir), Folder: true}}
		parent := folder(parentDir(dir))
		parent.children = append(parent.children, f)
		folders[dir] = f
		return f
	}
	for _, e := range idx.Entries {
		p := rel(e.Path)
		f := folder(parentDir(p))
		if path.Base(p) == "index.md" && f != root {
			f.node.HasIndex = true
			continue
		}
		f.children = append(f.children, &branch{node: Node{Path: p, Title: title(e)}})
	}
	var flatten func(b *branch) []Node
	flatten = func(b *branch) []Node {
		out := make([]Node, len(b.children))
		for i, c := range b.children {
			out[i] = c.node
			out[i].Children = flatten(c)
		}
		sortTree(out)
		return out
	}
	return flatten(root), nil
}

// sortTree orders a folder: index.md, subfolders, then entries, each alphabetically.
func sortTree(nodes []Node) {
	rank := func(n Node) int {
		switch {
		case !n.Folder && path.Base(n.Path) == "index.md":
			return 0
		case n.Folder:
			return 1
		}
		return 2
	}
	slices.SortFunc(nodes, func(a, b Node) int {
		if ra, rb := rank(a), rank(b); ra != rb {
			return ra - rb
		}
		return strings.Compare(a.Path, b.Path)
	})
}

// Entry is one entry of the project's wiki, with what the reader needs to
// render it.
func (w *Wikis) Entry(project, p string) (Entry, error) {
	idx, err := w.open(project)
	if err != nil {
		return Entry{}, err
	}
	// Rooted, the request path is only a key: `..` matches no entry.
	e, err := idx.Resolve("/" + strings.TrimPrefix(p, "/"))
	if err != nil {
		return Entry{}, fmt.Errorf("%w: %s", ErrNotFound, p)
	}
	raw, err := e.Raw()
	if err != nil {
		return Entry{}, err
	}
	body, err := e.Body()
	if err != nil {
		return Entry{}, err
	}
	// How many lines the frontmatter took, from what Body stripped.
	offset := strings.Count(raw[:len(raw)-len(body)], "\n")
	return Entry{
		Path:        rel(e.Path),
		Title:       title(e),
		Version:     home.Version([]byte(raw)),
		Frontmatter: Fields(e.Frontmatter(), ref(idx, e)),
		Body:        body,
		Links:       links(idx, e),
		Headings:    headings(e, offset),
		Checkboxes:  checkboxes(e, offset),
		Backlinks:   backlinks(idx, e),
	}, nil
}

// images are the files the reader shows inline, by extension: nothing a
// browser would run. An SVG runs no script inside an <img>, and the server
// serves it under a policy that forbids it anyway.
var images = map[string]string{
	".png":  "image/png",
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".gif":  "image/gif",
	".webp": "image/webp",
	".avif": "image/avif",
	".svg":  "image/svg+xml",
}

// Image finds an image of the project's wiki: its file on disk and its
// content type. Only an image an entry links to is served, and only one
// inside the wiki once symlinks are followed, so the route can't become a
// file server for whatever an agent links or symlinks.
func (w *Wikis) Image(project, p string) (file, contentType string, err error) {
	idx, err := w.open(project)
	if err != nil {
		return "", "", err
	}
	target := "/" + strings.TrimPrefix(p, "/")
	contentType = images[strings.ToLower(path.Ext(target))]
	linked := slices.ContainsFunc(idx.Entries, func(e *index.Entry) bool {
		return slices.ContainsFunc(e.Links, func(l index.Link) bool { return l.Target == target })
	})
	if contentType == "" || !linked {
		return "", "", fmt.Errorf("%w: %s", ErrNotFound, p)
	}
	root, err := filepath.EvalSymlinks(idx.Bundle.Dir)
	if err != nil {
		return "", "", err
	}
	file, err = filepath.EvalSymlinks(filepath.Join(idx.Bundle.Dir, filepath.FromSlash(target)))
	if err != nil || !within(root, file) {
		return "", "", fmt.Errorf("%w: %s", ErrNotFound, p)
	}
	if info, err := os.Stat(file); err != nil || !info.Mode().IsRegular() {
		return "", "", fmt.Errorf("%w: %s", ErrNotFound, p)
	}
	return file, contentType, nil
}

func links(idx *index.Index, e *index.Entry) []Link {
	out := make([]Link, 0, len(e.Links)+len(e.Outside))
	for _, l := range e.Links {
		_, err := idx.Resolve(l.Target)
		// Not an entry and not named like one: a file the wiki carries. A
		// missing .md is an entry not written yet, not a file.
		file := err != nil && !strings.HasSuffix(l.Target, ".md") && idx.FileExists(l.Target)
		out = append(out, Link{
			Raw:    l.Raw,
			To:     rel(l.Target),
			Anchor: l.Anchor,
			Exists: err == nil,
			File:   file,
			Image:  file && images[strings.ToLower(path.Ext(l.Target))] != "",
		})
	}
	for _, l := range e.Outside {
		out = append(out, Link{Raw: l.Raw, Outside: true})
	}
	return out
}

func headings(e *index.Entry, offset int) []Heading {
	texts := make([]string, len(e.Headings))
	for i, h := range e.Headings {
		texts[i] = h.Text
	}
	ids := headingIDs(texts)
	out := make([]Heading, len(e.Headings))
	for i, h := range e.Headings {
		out[i] = Heading{ID: ids[i], BodyLine: h.Line - offset}
	}
	return out
}

func checkboxes(e *index.Entry, offset int) []Checkbox {
	out := make([]Checkbox, len(e.Checkboxes))
	for i, c := range e.Checkboxes {
		out[i] = Checkbox{Line: c.Line, BodyLine: c.Line - offset, Done: c.Done, Text: c.Text}
	}
	return out
}

func backlinks(idx *index.Index, e *index.Entry) []Backlink {
	out := []Backlink{}
	for _, r := range idx.Backlinks(e.Path) {
		if slices.ContainsFunc(out, func(b Backlink) bool { return b.From == rel(r.From) }) {
			continue // one row per linking entry, however many links it has
		}
		t := titleFromFilename(r.From)
		if src, err := idx.Resolve(r.From); err == nil {
			t = title(src)
		}
		out = append(out, Backlink{From: rel(r.From), Title: t})
	}
	return out
}

// Fields is a frontmatter as the reader shows it, by key. Not shown: the
// index's own keys (leading underscore), and okf_version, the format's
// bookkeeping on the root index.md (wikiview hides it too). ref resolves a
// value that names an entry; nil resolves none (a repo file's frontmatter).
func Fields(fm map[string]any, ref func(v string) Value) []Field {
	out := []Field{}
	for _, k := range slices.Sorted(maps.Keys(fm)) {
		if strings.HasPrefix(k, "_") || k == "okf_version" {
			continue
		}
		f := Field{Key: k, Values: []Value{}}
		for _, v := range parse.Strings(fm, k) {
			if ref == nil {
				f.Values = append(f.Values, Value{Text: v})
			} else {
				f.Values = append(f.Values, ref(v))
			}
		}
		out = append(out, f)
	}
	return out
}

// title is an entry's `title` field, else its body's first `# heading`, else
// a readable name from its filename: cards written either way get a title.
func title(e *index.Entry) string {
	if t := e.Field("title"); t != "" {
		return t
	}
	for _, h := range e.Headings {
		if h.Level == 1 && h.Text != "" {
			return h.Text
		}
	}
	return titleFromFilename(e.Path)
}

func folderTitle(dir string) string {
	if t := readableName(path.Base(dir)); t != "" {
		return t
	}
	return path.Base(dir)
}

// rel turns the engine's root-absolute path (/backlog/x.md) into the wire's.
func rel(p string) string { return strings.TrimPrefix(p, "/") }

func parentDir(p string) string {
	if d := path.Dir(p); d != "." {
		return d
	}
	return ""
}

// within reports whether p is root or inside it; both clean and symlink-free.
func within(root, p string) bool {
	r, err := filepath.Rel(root, p)
	return err == nil && r != ".." && !strings.HasPrefix(r, ".."+string(filepath.Separator))
}
