package repos

import (
	"context"
	"errors"
	"path"
	"strings"

	"github.com/agentic-wiki/wiki/parse"

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

// links resolves the internal links of the Markdown file at file: relative
// ones from its folder, root-absolute ones from its wiki's root. Web links
// and same-page anchors are left to the browser.
func (t *trees) links(file, body string) []Link {
	set := parse.Links(body)
	out := make([]Link, 0, len(set.Absolute)+len(set.Relative))
	root := t.base(file)
	for _, bucket := range [][]parse.Link{set.Absolute, set.Relative} {
		for _, l := range bucket {
			out = append(out, t.resolve(file, root, l.Target))
		}
	}
	return out
}

func (t *trees) resolve(file, root, raw string) Link {
	l := Link{Raw: raw}
	target := raw
	if h := strings.IndexByte(target, '#'); h >= 0 {
		target, l.Anchor = target[:h], target[h+1:]
	}
	if q := strings.IndexByte(target, '?'); q >= 0 {
		target = target[:q]
	}
	target = wiki.Decoded(target)
	from := path.Dir(file)
	if strings.HasPrefix(target, "/") {
		from, target = root, strings.TrimLeft(target, "/")
	}
	p := path.Clean(path.Join(from, target))
	if p == ".." || strings.HasPrefix(p, "../") {
		l.Outside = true
		return l
	}
	if p == "." {
		p = ""
	}
	l.To = p
	e, ok := t.entry(l.To)
	l.Exists = ok
	l.Folder = ok && e.Kind == "dir"
	l.Image = ok && e.Kind == "file" && wiki.ImageType(l.To) != ""
	return l
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
