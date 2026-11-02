package wiki

import (
	"os"
	"slices"
	"strings"
	"testing"
)

const boards = `spec = "0.1"

[[tool.wikiview.board]]
id      = "sprint"
path    = "/backlog"
name    = "Sprint 07"
columns = ["todo", "done"]
lane    = "priority"
lanes   = ["high"]

[[tool.wikiview.board]]
id    = "findings"
path  = "/backlog"
where = ["type=task", "kind=finding"]

[[tool.wikiview.board]]
path = "/backlog"
colour = "red"

[[tool.wikiview.board]]
id    = "findings"
path  = "/backlog"
where = ["nope"]

[other]
x = 1
`

func task(fields string) string { return "---\ntype: task\n" + fields + "---\n" }

func TestBoards(t *testing.T) {
	w, _ := fixture(t, map[string]string{
		"wiki.toml":                boards,
		"backlog/b-review.md":      task("status: review\npriority: low\ntitle: Review me\n"),
		"backlog/a-todo.md":        task("status: todo\npriority: high\nassignee: tove\n"),
		"backlog/c-none.md":        task("priority: high\n"),
		"backlog/d-weird.md":       task("status: zzz\n"),
		"backlog/e-blocked.md":     task("status: blocked\npriority: medium\nkind: finding\n"),
		"backlog/notes.md":         note(""),
		"backlogs/elsewhere.md":    task("status: todo\n"),
		"backlog/archive/f-old.md": task("status: done\n"),
	})
	got, err := w.Boards("p")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 4 {
		t.Fatalf("%d boards, want every declaration listed: %+v", len(got), got)
	}
	sprint := got[0]
	if sprint.ID != "sprint" || sprint.Name != "Sprint 07" || sprint.StatusField != "status" || sprint.LaneField != "priority" {
		t.Errorf("sprint = %+v", sprint)
	}
	// Declared first (even empty), then known statuses in their order, then the
	// rest alphabetically, then the cards with none.
	if want := []string{"todo", "done", "review", "blocked", "zzz", ""}; !slices.Equal(sprint.Columns, want) {
		t.Errorf("columns = %q, want %q", sprint.Columns, want)
	}
	if want := []string{"high", "medium", "low", ""}; !slices.Equal(sprint.Lanes, want) {
		t.Errorf("lanes = %q, want %q", sprint.Lanes, want)
	}
	var paths []string
	for _, c := range sprint.Cards {
		paths = append(paths, c.Path)
	}
	// Only the folder's tasks (its subfolders too), never a folder that merely starts alike.
	if want := []string{"backlog/a-todo.md", "backlog/archive/f-old.md", "backlog/b-review.md", "backlog/c-none.md", "backlog/d-weird.md", "backlog/e-blocked.md"}; !slices.Equal(paths, want) {
		t.Errorf("cards = %q, want %q", paths, want)
	}
	a := sprint.Cards[0]
	if a.ID != "a-todo" || a.Title != "A todo" || a.Column != "todo" || a.Lane != "high" || a.Priority != "high" || a.Assignee != "tove" || a.Version == "" {
		t.Errorf("card = %+v", a)
	}
	if sprint.Cards[2].Title != "Review me" {
		t.Errorf("a card's title field: %+v", sprint.Cards[2])
	}
	if len(sprint.Problems) != 0 || sprint.Problems == nil {
		t.Errorf("problems = %#v, want an empty list", sprint.Problems)
	}

	// Two boards over one folder: each shows what its filter selects; no lanes.
	findings := got[1]
	if len(findings.Cards) != 1 || findings.Cards[0].Path != "backlog/e-blocked.md" || len(findings.Lanes) != 0 || findings.Name != "Backlog" {
		t.Errorf("findings = %+v", findings)
	}

	// Without an id, or with a taken one, a board is still listed with its
	// problems, under an id no move can address.
	if got[2].ID != "#3" || !slices.Contains(got[2].Problems, "id is required") || !slices.Contains(got[2].Problems, `unknown key "colour"`) {
		t.Errorf("no id: %+v", got[2])
	}
	if got[3].ID != "#4" || len(got[3].Problems) != 2 || !strings.Contains(got[3].Problems[0], "used by another board") {
		t.Errorf("taken id, bad filter: %+v", got[3].Problems)
	}
}

func TestBoardsOfAWrongType(t *testing.T) {
	w, _ := fixture(t, map[string]string{
		"wiki.toml": "spec = \"0.1\"\n\n[[tool.wikiview.board]]\nid = \"b\"\npath = \"/\"\ncolumns = \"todo\"\n",
	})
	got, err := w.Boards("p")
	if err != nil || len(got) != 1 || len(got[0].Problems) != 1 || !strings.Contains(got[0].Problems[0], "wiki.toml") {
		t.Errorf("boards %+v %v, want the decoding problem shown", got, err)
	}
}

func TestBoardsNoneDeclared(t *testing.T) {
	w, _ := fixture(t, map[string]string{"index.md": "# Home\n"})
	if got, err := w.Boards("p"); err != nil || got == nil || len(got) != 0 {
		t.Errorf("boards %#v %v, want an empty list", got, err)
	}
}

// The starter's two task boards (internal/home/starter): a task moved to
// todo on Backlog stays there and shows on Board too; one at work only on
// Board; an unscheduled one only on Backlog (the CEO, 2026-10-17).
func TestTheStartersTaskBoards(t *testing.T) {
	toml, err := os.ReadFile("../home/starter/wiki/wiki.toml")
	if err != nil {
		t.Fatal(err)
	}
	w, _ := fixture(t, map[string]string{
		"wiki.toml":            string(toml),
		"backlog/idea.md":      task("status: backlog\npriority: high\n"),
		"backlog/scheduled.md": task("status: todo\npriority: high\n"),
		"backlog/working.md":   task("status: in-progress\npriority: high\n"),
		"backlog/stuck.md":     task("status: blocked\npriority: high\n"),
		"backlog/stray.md":     task("status: review\npriority: high\n"), // not ours: off Backlog
		"backlog/epics/a.md":   "---\ntype: epic\nstatus: todo\n---\n",
	})
	got, err := w.Boards("p")
	if err != nil {
		t.Fatal(err)
	}
	cards := func(id string) []string {
		for _, b := range got {
			if b.ID == id {
				if id == "board" && !slices.Equal(b.Columns, []string{"todo", "blocked", "in-progress", "done", "review"}) {
					t.Errorf("Board's columns %v", b.Columns)
				}
				if len(b.Problems) > 0 {
					t.Errorf("%s: %v", id, b.Problems)
				}
				var list []string
				for _, c := range b.Cards {
					list = append(list, c.Path+":"+c.Column)
				}
				slices.Sort(list)
				return list
			}
		}
		t.Fatalf("no board %s", id)
		return nil
	}
	if b, want := cards("board"), []string{"backlog/scheduled.md:todo", "backlog/stray.md:review", "backlog/stuck.md:blocked", "backlog/working.md:in-progress"}; !slices.Equal(b, want) {
		t.Errorf("Board %v, want %v", b, want)
	}
	if b, want := cards("backlog"), []string{"backlog/idea.md:backlog", "backlog/scheduled.md:todo"}; !slices.Equal(b, want) {
		t.Errorf("Backlog %v, want %v", b, want)
	}
}
