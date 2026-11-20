package wiki

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// badHeader isn't valid YAML: the wiki library reads it with no fields.
const badHeader = "---\ntitle: a: b\n---\n# Bad\n\nThe body still reads.\n"

const badReason = "line 2: mapping values are not allowed in this context"

// An entry whose frontmatter can't be read says why, on its page and in the
// tree, and its body still reads. An entry that reads carries no reason.
func TestUnreadFrontmatterOnEntryAndTree(t *testing.T) {
	w, _ := fixture(t, map[string]string{
		"notes/bad.md":  badHeader,
		"notes/good.md": note("# Good\n"),
	})
	bad, err := w.Entry("p", "notes/bad.md")
	if err != nil {
		t.Fatal(err)
	}
	if bad.FrontmatterError != badReason || len(bad.Frontmatter) != 0 || !strings.Contains(bad.Body, "The body still reads.") {
		t.Errorf("bad.md: error %q, frontmatter %+v, body %q", bad.FrontmatterError, bad.Frontmatter, bad.Body)
	}
	if good, _ := w.Entry("p", "notes/good.md"); good.FrontmatterError != "" {
		t.Errorf("good.md: error %q", good.FrontmatterError)
	}
	tree, err := w.Tree("p")
	if err != nil {
		t.Fatal(err)
	}
	reasons := map[string]string{}
	var walk func([]Node)
	walk = func(nodes []Node) {
		for _, n := range nodes {
			reasons[n.Path] = n.FrontmatterError
			walk(n.Children)
		}
	}
	walk(tree)
	if reasons["notes/bad.md"] != badReason || reasons["notes/good.md"] != "" || reasons["notes"] != "" {
		t.Errorf("tree reasons %q", reasons)
	}
}

// An unread entry under a board's path is listed in Unread, with its title and
// reason, and is never a card: not on a task board, whose where it can't
// match, nor on a board over every entry (where = []), as a card with no
// status. One outside the path isn't listed, and a board with none, or one
// whose declaration couldn't be read, has an empty list, never null.
func TestUnreadEntriesLeaveBoardsVisibly(t *testing.T) {
	w, _ := fixture(t, map[string]string{
		"wiki.toml": "spec = \"0.1\"\n\n" +
			"[[board]]\nid = \"tasks\"\npath = \"/backlog\"\n\n" +
			"[[board]]\nid = \"all\"\npath = \"/backlog\"\nwhere = []\n\n" +
			"[[board]]\nid = \"notes\"\npath = \"/notes\"\n",
		"backlog/a.md":   task("status: todo\n"),
		"backlog/bad.md": badHeader,
		"elsewhere/x.md": badHeader,
		"notes/n.md":     note(""),
	})
	boards, err := w.Boards("p")
	if err != nil {
		t.Fatal(err)
	}
	want := []Unread{{Path: "backlog/bad.md", Label: "Bad", Error: badReason}}
	for _, b := range boards {
		if b.ID == "notes" {
			if data, _ := json.Marshal(b); !strings.Contains(string(data), `"unread":[]`) {
				t.Errorf("notes board: %s", data)
			}
			continue
		}
		if !slices.Equal(b.Unread, want) {
			t.Errorf("%s: unread %+v, want %+v", b.ID, b.Unread, want)
		}
		if slices.ContainsFunc(b.Cards, func(c Card) bool { return c.Path == "backlog/bad.md" }) {
			t.Errorf("%s: the unread entry is a card", b.ID)
		}
	}

	w, _ = fixture(t, map[string]string{"wiki.toml": "spec = \"0.1\"\n\n[[board]]\nid = \"b\"\npath = \"/\"\ncolumns = \"todo\"\n"})
	boards, _ = w.Boards("p")
	if data, _ := json.Marshal(boards); !strings.Contains(string(data), `"unread":[]`) {
		t.Errorf("a board whose declaration couldn't be read: %s", data)
	}
}

// A field's values leave out empty items, which the wiki treats as no value.
func TestFieldsDropEmptyItems(t *testing.T) {
	w, _ := fixture(t, map[string]string{"a.md": "---\ntype: note\ntags: [a, \"\", b]\nowners:\n  -\n---\n"})
	e, err := w.Entry("p", "a.md")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string][]string{}
	for _, f := range e.Frontmatter {
		for _, v := range f.Values {
			got[f.Key] = append(got[f.Key], v.Text)
		}
	}
	if !slices.Equal(got["tags"], []string{"a", "b"}) || len(got["owners"]) != 0 {
		t.Errorf("fields %q", got)
	}
}

// A Markdown file outside the wiki (a repo's) is read by the wiki's rules: its
// fields without empty items, its body, its headings' ids (a repeat numbered)
// with their lines in the body, and why its frontmatter couldn't be read.
func TestMarkdownReadsARepoFile(t *testing.T) {
	fm, body, heads, reason := Markdown("docs/plan.md", "---\ntags: [a, \"\"]\n---\n# Plan\n## Plan\n")
	if len(fm) != 1 || fm[0].Key != "tags" || len(fm[0].Values) != 1 || fm[0].Values[0].Text != "a" {
		t.Errorf("fields %+v", fm)
	}
	if body != "# Plan\n## Plan\n" || reason != "" {
		t.Errorf("body %q, reason %q", body, reason)
	}
	if !slices.Equal(heads, []Heading{{ID: "plan", BodyLine: 1}, {ID: "plan-1", BodyLine: 2}}) {
		t.Errorf("headings %+v", heads)
	}
	if fm, body, _, reason := Markdown("docs/bad.md", badHeader); len(fm) != 0 || reason != badReason || !strings.Contains(body, "The body still reads.") {
		t.Errorf("bad: fields %+v, reason %q, body %q", fm, reason, body)
	}
}

// A card move onto a frontmatter the wiki library can't edit safely (a flow
// map) is refused with the library's reason, and the file and the history are
// left as they were.
func TestMoveCardRefused(t *testing.T) {
	const flow = "---\n{type: task, status: todo}\n---\nFlow\n"
	w, dir := fixture(t, map[string]string{"wiki.toml": sprint, "backlog/flow.md": flow})
	e, _ := w.Entry("p", "backlog/flow.md")
	before := gitOut(t, dir, "log", "--format=%H")
	err := w.MoveCard(ctx, "p", "flat", "backlog/flow.md", "done", "", e.Version)
	if !errors.Is(err, ErrRefused) || !strings.Contains(err.Error(), "frontmatter of /backlog/flow.md can't be edited safely") {
		t.Errorf("MoveCard = %v, want ErrRefused with the reason", err)
	}
	if got := read(t, filepath.Join(dir, "backlog", "flow.md")); got != flow {
		t.Errorf("flow.md changed: %q", got)
	}
	if after := gitOut(t, dir, "log", "--format=%H"); after != before {
		t.Error("a commit was made")
	}
}

// A card with no frontmatter (an index.md under a board over every entry) can't
// be given a status: a move is ErrInvalid with the library's reason, and the
// file is left as it was.
func TestMoveCardOntoAReservedFile(t *testing.T) {
	const home = "# Backlog\n"
	w, dir := fixture(t, map[string]string{
		"wiki.toml":        "spec = \"0.1\"\n\n[[board]]\nid = \"all\"\npath = \"/backlog\"\nwhere = []\n",
		"backlog/index.md": home,
	})
	e, _ := w.Entry("p", "backlog/index.md")
	err := w.MoveCard(ctx, "p", "all", "backlog/index.md", "done", "", e.Version)
	if !errors.Is(err, ErrInvalid) || !strings.Contains(err.Error(), "reserved file carries no frontmatter") {
		t.Errorf("MoveCard = %v, want ErrInvalid with the reason", err)
	}
	if got := read(t, filepath.Join(dir, "backlog", "index.md")); got != home {
		t.Errorf("index.md changed: %q", got)
	}
}
