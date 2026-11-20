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
	"github.com/fsnotify/fsnotify"

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

	mu        sync.Mutex
	watched   map[string]*watched // by project, while Watch runs
	unrenamed map[string]error    // by project: why its old board tables couldn't be renamed
	writing   sync.Mutex          // one write at a time: check, change, commit
	watcher   *fsnotify.Watcher   // Watch's: set once, before it follows anything
}

// New reads the wiki that dir names for each project (its bundle folder, the
// one holding wiki.toml), and commits its writes with git.
func New(dir func(project string) (string, error), git Git) *Wikis {
	return &Wikis{dir: dir, git: git, watched: map[string]*watched{}, unrenamed: map[string]error{}}
}

// refresh rebuilds a followed wiki's index after Foreman's own write, so the
// next read sees it. Only the index: the versions stay, so the watcher still
// finds the change and tells everyone else. Called under writing.
func (w *Wikis) refresh(project string) {
	_, idx, err := w.load(project)
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

// build reads the project's wiki from disk: its folder and its index. It
// takes writing, since it may rewrite and commit wiki.toml first (migrate).
func (w *Wikis) build(project string) (string, *index.Index, error) {
	w.writing.Lock()
	defer w.writing.Unlock()
	return w.load(project)
}

// load is build for a caller that holds writing.
func (w *Wikis) load(project string) (string, *index.Index, error) {
	dir, err := w.dir(project)
	if err != nil {
		return "", nil, fmt.Errorf("%w: %s", ErrNotFound, project)
	}
	failed := w.migrate(dir)
	w.mu.Lock()
	if failed != nil {
		w.unrenamed[project] = failed
	} else {
		delete(w.unrenamed, project)
	}
	w.mu.Unlock()
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
		f.children = append(f.children, &branch{node: Node{Path: p, Title: title(e), FrontmatterError: frontmatterError(e)}})
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

// Title is an entry's title, from the index alone: for a name in a list, where
// Entry would read and work out the whole page.
func (w *Wikis) Title(project, p string) (string, error) {
	idx, err := w.open(project)
	if err != nil {
		return "", err
	}
	e, err := idx.Resolve("/" + strings.TrimPrefix(p, "/"))
	if err != nil {
		return "", fmt.Errorf("%w: %s", ErrNotFound, p)
	}
	return title(e), nil
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
	off := offset(raw, body)
	return Entry{
		Path:             rel(e.Path),
		Title:            title(e),
		Version:          home.Version([]byte(raw)),
		Frontmatter:      fields(e, idx),
		FrontmatterError: frontmatterError(e),
		Body:             body,
		Links:            links(idx, e),
		Headings:         headings(e, off),
		Checkboxes:       checkboxes(e, off),
		Backlinks:        backlinks(idx, e),
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

// ImageType is the content type of a file the readers show inline, by its
// name's extension; "" for anything else.
func ImageType(name string) string { return images[strings.ToLower(path.Ext(name))] }

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
	contentType = ImageType(target)
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
			Image:  file && ImageType(l.Target) != "",
		})
	}
	for _, l := range e.Outside {
		out = append(out, Link{Raw: l.Raw, Outside: true})
	}
	return out
}

// Markdown is a Markdown file outside the wiki (a repo's), read by the wiki's
// rules: its frontmatter, its body, its headings' ids and why its frontmatter
// couldn't be read ("" when it could).
func Markdown(p, text string) (fm []Field, body string, heads []Heading, frontmatterErr string) {
	e := index.Read(p, text)
	body, _ = e.Body() // never fails: a Read entry's text is in memory
	return fields(e, nil), body, headings(e, offset(text, body)), frontmatterError(e)
}

// offset is how many lines the frontmatter took, from what Body stripped.
func offset(raw, body string) int { return strings.Count(raw[:len(raw)-len(body)], "\n") }

// headings are the entry's headings, each with its id (the engine's anchor
// rule) and its line in the body.
func headings(e *index.Entry, offset int) []Heading {
	out := make([]Heading, len(e.Headings))
	for i, h := range e.Headings {
		out[i] = Heading{ID: h.ID, BodyLine: h.Line - offset}
	}
	return out
}

// frontmatterError is why the entry's frontmatter couldn't be read, or "".
func frontmatterError(e *index.Entry) string {
	if err := e.FrontmatterErr(); err != nil {
		return err.Error()
	}
	return ""
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

// fields is an entry's frontmatter as the reader shows it, by key, each
// field's values without empty ones (FieldList). Not shown: the index's own
// keys (leading underscore), and okf_version, the format's bookkeeping on the
// root index.md (WikiShell hides it too). With idx, a value that names an
// existing entry (Entry.FrontmatterRefs) links to it, with its title; a repo
// file's frontmatter (idx nil) is text.
func fields(e *index.Entry, idx *index.Index) []Field {
	named := map[string]map[string]string{} // key -> value -> the entry's path
	for _, r := range e.FrontmatterRefs() {
		if idx != nil && !r.Outside {
			if named[r.Key] == nil {
				named[r.Key] = map[string]string{}
			}
			named[r.Key][r.Value] = r.Path
		}
	}
	out := []Field{}
	for _, k := range slices.Sorted(maps.Keys(e.Frontmatter())) {
		if strings.HasPrefix(k, "_") || k == "okf_version" {
			continue
		}
		f := Field{Key: k, Values: []Value{}}
		for _, v := range e.FieldList(k) {
			value := Value{Text: v}
			if p, ok := named[k][v]; ok {
				if to, err := idx.Resolve(p); err == nil { // rooted: by path, never by file name
					value.To, value.Title = rel(to.Path), title(to)
				}
			}
			f.Values = append(f.Values, value)
		}
		out = append(out, f)
	}
	return out
}

// title is the entry's title by the wiki library's rule (`title:`, else its
// first `# heading`), else a readable name from its filename: cards written
// either way get a title.
func title(e *index.Entry) string {
	if t := e.Title(); t != "" {
		return t
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
