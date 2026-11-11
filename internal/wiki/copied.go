package wiki

// Copied from wikiview v0.13.0 (internal/server/entry.go), whose Go code is
// internal and can't be imported: the readable-name rule, presentation the
// wiki library leaves to its callers. It goes when wikiview exports it
// (wiki/technical-debt.md).

import (
	"path"
	"strings"
	"unicode"
	"unicode/utf8"
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
