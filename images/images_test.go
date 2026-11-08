package images

import (
	"os"
	"regexp"
	"testing"
)

// The agents' wiki CLI and Foreman's wiki library are one version, so both
// read and write a wiki by the same rules: go.mod is the source.
func TestWikiVersionMatchesGoMod(t *testing.T) {
	mod, err := os.ReadFile("../go.mod")
	if err != nil {
		t.Fatal(err)
	}
	version := func(re string, in []byte) string {
		if m := regexp.MustCompile(re).FindSubmatch(in); m != nil {
			return string(m[1])
		}
		return ""
	}
	library := version(`(?m)^\s*github\.com/agentic-wiki/wiki (v\S+)`, mod)
	cli := version(`(?m)^ARG WIKI_VERSION=(\S+)$`, Harness)
	if library == "" || cli != library {
		t.Errorf("go.mod requires wiki %q, the Containerfile installs %q", library, cli)
	}
}
