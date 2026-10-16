//go:build release

package ui

import (
	"io/fs"
	"regexp"
	"testing"
)

// A release binary serves the built app from inside itself: index.html and
// every file it loads.
func TestTheAppIsEmbedded(t *testing.T) {
	app := UI()
	index, err := fs.ReadFile(app, "index.html")
	if err != nil {
		t.Fatal(err)
	}
	loads := regexp.MustCompile(`(?:src|href)="/([^"]+)"`).FindAllSubmatch(index, -1)
	if len(loads) == 0 {
		t.Fatalf("index.html loads nothing: %s", index)
	}
	for _, m := range loads {
		if _, err := fs.Stat(app, string(m[1])); err != nil {
			t.Errorf("index.html loads /%s, which isn't embedded", m[1])
		}
	}
}
