package repos

import (
	"context"
	"errors"
	"path"
	"slices"
	"strings"

	"github.com/agentic-wiki/wiki/index"

	"github.com/brickpop/foreman/internal/git"
	"github.com/brickpop/foreman/internal/wiki"
)

// ImageLimit is the largest image served.
const ImageLimit = 10 << 20

// wikiRoot names the file that makes a folder a wiki's root: a root-absolute
// link in a file below it resolves from there, as the wiki engine does.
const wikiRoot = "wiki.toml"

// trees reads the folders of one commit, each once.
type trees struct {
	ctx         context.Context
	git         Git
	dir, commit string
	read        map[string][]git.TreeEntry
}

// entry is what the commit has at p, if anything ("" is the root: a folder).
func (t *trees) entry(p string) (git.TreeEntry, bool) {
	if p == "" {
		return git.TreeEntry{Kind: "dir"}, true
	}
	parent := path.Dir(p)
	if parent == "." {
		parent = ""
	}
	list, ok := t.read[parent]
	if !ok {
		list, _ = t.git.Tree(t.ctx, t.dir, t.commit, parent) // a folder it lacks lists nothing
		t.read[parent] = list
	}
	for _, e := range list {
		if e.Name == path.Base(p) {
			return e, true
		}
	}
	return git.TreeEntry{}, false
}

// base is the folder a root-absolute link in file resolves from: the nearest
// one above it holding a wiki.toml, else the repo's root.
func (t *trees) base(file string) string {
	for d := path.Dir(file); d != "." && d != "/"; d = path.Dir(d) {
		if _, ok := t.entry(path.Join(d, wikiRoot)); ok {
			return d
		}
	}
	return ""
}

// links resolves the internal links of the Markdown file at file, by the wiki
// library's one rule: the file is read from its wiki's root (index.Read), so
// a root-absolute link resolves from there, a relative one from its folder,
// and its percent-encoding, ?query and #anchor are read as the engine reads
// them. Each link, the climbing ones too, is then joined to the wiki's root:
// it is outside only when it climbs out of the repo. Web links and same-page
// anchors are left to the browser.
func (t *trees) links(file, text string) []Link {
	root := t.base(file)
	e := index.Read(path.Join("/", strings.TrimPrefix(file, root)), text) // base is a folder above file
	out := make([]Link, 0, len(e.Links)+len(e.Outside))
	for _, l := range slices.Concat(e.Links, e.Outside) {
		out = append(out, t.link(root, l))
	}
	return out
}

// link is l, read from the wiki rooted at root, as a path in the repo: its
// Target joined to root, outside when that climbs out of the repo
// (segment-exact: "..notes/a.md" is inside).
func (t *trees) link(root string, l index.Link) Link {
	out := Link{Raw: l.Raw, Anchor: l.Anchor}
	p := path.Clean(path.Join(root, strings.TrimPrefix(l.Target, "/")))
	if p == ".." || strings.HasPrefix(p, "../") {
		out.Outside = true
		return out
	}
	if p == "." {
		p = ""
	}
	out.To = p
	e, ok := t.entry(p)
	out.Exists = ok
	out.Folder = ok && e.Kind == "dir"
	out.Image = ok && e.Kind == "file" && wiki.ImageType(p) != ""
	return out
}

// Image returns the image at path on ref, and its content type: only a file
// of an image type, never one larger than ImageLimit.
func (b Browser) Image(ctx context.Context, project, repo, ref, p string) ([]byte, string, error) {
	contentType := wiki.ImageType(p)
	if contentType == "" {
		return nil, "", git.ErrNoPath
	}
	dir, commit, err := b.at(ctx, project, repo, ref)
	if err != nil {
		return nil, "", err
	}
	// A symlink is a blob too: its content would be the path it names.
	t := &trees{ctx: ctx, git: b.Git, dir: dir, commit: commit, read: map[string][]git.TreeEntry{}}
	if e, ok := t.entry(strings.Trim(path.Clean(p), "/")); !ok || e.Kind != "file" {
		return nil, "", git.ErrNoPath
	}
	content, size, err := b.Git.File(ctx, dir, commit, p, ImageLimit)
	if err == nil && content == nil && size > ImageLimit {
		err = errors.Join(git.ErrNoPath, errors.New("over the image limit"))
	}
	return content, contentType, err
}
