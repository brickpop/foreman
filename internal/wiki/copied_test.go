package wiki

import (
	"os/exec"
	"slices"
	"strings"
	"testing"
)

func TestHeadingIDs(t *testing.T) {
	for in, want := range map[string]string{
		"Simple":                       "simple",
		"Two Words":                    "two-words",
		"With-Hyphen and_underscore":   "with-hyphen-and_underscore", // most sluggers drop the _
		"Punctuation: removed, right?": "punctuation-removed-right",
		"Numbers 123":                  "numbers-123",
		"Ünïcode wörds":                "ünïcode-wörds",
		"":                             "heading",
		"!!!":                          "heading",
	} {
		if got := headingID(in); got != want {
			t.Errorf("headingID(%q) = %q, want %q", in, got, want)
		}
	}
	if got := headingIDs([]string{"Notes", "Other", "Notes", "Notes"}); !slices.Equal(got, []string{"notes", "other", "notes-1", "notes-2"}) {
		t.Errorf("repeats = %v", got)
	}
}

// The rule copies one the engine keeps unexported, so it's checked against
// the engine: `wiki check` warns on an anchor that matches no heading.
func TestHeadingIDsAgreeWithTheEngine(t *testing.T) {
	bin, err := exec.LookPath("wiki")
	if err != nil {
		t.Skip("the wiki CLI isn't installed")
	}
	texts := []string{"Simple", "With-Hyphen and_underscore", "Punctuation: removed, right?", "Trailing underscore_", "__dunder__", "Ünïcode wörds", "Notes", "Notes"}
	var body strings.Builder
	for _, h := range texts {
		body.WriteString("## " + h + "\n\n")
	}
	for _, id := range headingIDs(texts) {
		body.WriteString("[to](#" + id + ")\n\n")
	}
	_, dir := fixture(t, map[string]string{
		"index.md": "---\nokf_version: \"0.1\"\n---\n[a](a.md)\n",
		"a.md":     note(body.String()),
	})
	out, _ := exec.Command(bin, "--root", dir, "check").CombinedOutput()
	if strings.Contains(string(out), "anchor not found") {
		t.Errorf("the engine disagrees with an id:\n%s", out)
	}
}
