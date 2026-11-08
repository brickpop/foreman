package wiki

// Copied from wikiview v0.13.0 (internal/server/entry.go), whose Go code is
// internal and can't be imported, and from the engine, which keeps its
// frontmatter-reference rule unexported. They go when wikiview and the engine
// export them (wiki/technical-debt.md).

import (
	"path"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/agentic-wiki/wiki/index"
)

// titleFromFilename makes a readable name out of a wiki path:
// "003-watch-and-events.md" reads "003 Watch and events".
func titleFromFilename(p string) string {
	base := path.Base(p)
	if readable := readableName(strings.TrimSuffix(base, ".md")); readable != "" {
		return readable
	}
	return base
}

// readableName turns a slug into words, capitalizing the first word that
// starts with a letter (a leading number has no case). "" when nothing is left.
func readableName(name string) string {
	name = strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(name, "-", " "), "_", " "))
	if name == "" {
		return ""
	}
	offset := 0
	for _, word := range strings.Split(name, " ") {
		if r, size := utf8.DecodeRuneInString(word); unicode.IsLetter(r) {
			return name[:offset] + string(unicode.ToUpper(r)) + name[offset+size:]
		}
		offset += len(word) + 1
	}
	return name
}

// Copied from the engine (index/index.go: frontmatterRewrites, unexported):
// which frontmatter values are references.

// ref is v as a value of e's frontmatter: a link to the entry it names when
// it is a reference (ends in .md, as the engine reads them) to an entry that
// exists. The engine resolves it: relative to e, or from the wiki's root with
// a leading /, and one that climbs above the wiki names nothing.
func ref(idx *index.Index, e *index.Entry) func(v string) Value {
	return func(v string) Value {
		if target, _, _ := strings.Cut(v, "#"); !strings.HasSuffix(target, ".md") {
			return Value{Text: v}
		}
		target, outside := idx.ResolveLink(e.Path, v)
		if outside {
			return Value{Text: v}
		}
		target, _, _ = strings.Cut(target, "#")
		// Rooted, so Resolve looks it up by path, never by file name.
		to, err := idx.Resolve(target)
		if err != nil {
			return Value{Text: v}
		}
		return Value{Text: v, To: rel(to.Path), Title: title(to)}
	}
}
