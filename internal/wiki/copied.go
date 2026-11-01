package wiki

// Copied from wikiview v0.13.0 (internal/slug, internal/server/entry.go),
// whose Go code is internal and can't be imported, and from the engine, which
// keeps these rules unexported. They go when wikiview and the engine export
// them (wiki/technical-debt.md).

import (
	"fmt"
	"path"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/agentic-wiki/wiki/index"
)

// headingIDs assigns an id to each heading text, in document order, by the
// engine's rule (unexported upstream): the one `wiki check` validates `#anchor`
// links against. Repeats get -1, -2…, as on GitHub.
func headingIDs(texts []string) []string {
	seen := map[string]int{}
	out := make([]string, len(texts))
	for i, text := range texts {
		s := headingID(text)
		n := seen[s]
		seen[s] = n + 1
		if n > 0 {
			s = fmt.Sprintf("%s-%d", s, n)
		}
		out[i] = s
	}
	return out
}

// headingID slugs one heading: lower-cased, spaces to hyphens, keeping
// letters, numbers, hyphens and underscores.
func headingID(text string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(text) {
		switch {
		case r == ' ':
			b.WriteByte('-')
		case unicode.IsLetter(r) || unicode.IsNumber(r) || r == '-' || r == '_':
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return "heading"
	}
	return b.String()
}

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

// Copied from the engine v0.10.0 (index/index.go: frontmatterRewrites and
// normalizeLink, unexported): which frontmatter values are references.

// ref is v as a value of e's frontmatter: a link to the entry it names when
// it is a reference (ends in .md, as the engine reads them: relative to e, or
// from the wiki's root with a leading /) to an entry that exists. One that
// climbs above the wiki names nothing, as the engine has it.
func ref(idx *index.Index, e *index.Entry) func(v string) Value {
	return func(v string) Value {
		target, _, _ := strings.Cut(v, "#")
		if !strings.HasSuffix(target, ".md") {
			return Value{Text: v}
		}
		if !strings.HasPrefix(target, "/") {
			target = path.Join(strings.TrimPrefix(path.Dir(e.Path), "/"), target)
			if target == ".." || strings.HasPrefix(target, "../") {
				return Value{Text: v}
			}
		}
		// Rooted, so Resolve looks it up by path, never by file name.
		to, err := idx.Resolve("/" + strings.TrimPrefix(path.Clean(target), "/"))
		if err != nil {
			return Value{Text: v}
		}
		return Value{Text: v, To: rel(to.Path), Title: title(to)}
	}
}
