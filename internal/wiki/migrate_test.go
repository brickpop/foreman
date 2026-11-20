package wiki

import (
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
)

func TestRenameTables(t *testing.T) {
	for _, c := range []struct{ name, in, want string }{
		{
			name: "boards and graphs, with indents, spaces and comments kept",
			in:   "spec = \"0.1\"\n\n# The sprint\n[[tool.wikiview.board]] # ours\nid = \"s\"\n\n  [[ tool.wikiview.graph ]]\nid = \"g\"\n",
			want: "spec = \"0.1\"\n\n# The sprint\n[[board]] # ours\nid = \"s\"\n\n  [[graph]]\nid = \"g\"\n",
		},
		{
			name: "CRLF line endings stay",
			in:   "spec = \"0.1\"\r\n[[tool.wikiview.board]]\r\nid = \"s\"\r\n",
			want: "spec = \"0.1\"\r\n[[board]]\r\nid = \"s\"\r\n",
		},
		{
			name: "old and new tables mixed: all renamed, in file order",
			in:   "[[board]]\nid = \"a\"\n\n[[tool.wikiview.board]]\nid = \"b\"\n",
			want: "[[board]]\nid = \"a\"\n\n[[board]]\nid = \"b\"\n",
		},
		{
			name: "an empty bare table goes, its comments stay",
			in:   "[tool.wikiview]\n# about the board\n\n[[tool.wikiview.board]]\nid = \"s\"\n",
			want: "# about the board\n\n[[board]]\nid = \"s\"\n",
		},
		{
			name: "an empty bare table at the end goes",
			in:   "spec = \"0.1\"\n[tool.wikiview]  # nothing left\n\n",
			want: "spec = \"0.1\"\n\n",
		},
		{
			name: "a bare table with keys stays",
			in:   "[tool.wikiview]\ndefault_board = \"/backlog\"\n\n[[tool.wikiview.board]]\nid = \"s\"\n",
			want: "[tool.wikiview]\ndefault_board = \"/backlog\"\n\n[[board]]\nid = \"s\"\n",
		},
		{
			name: "other tables and look-alikes stay",
			in:   "[tool.wikiviewer]\nx = 1\n[[tool.other.board]]\nid = \"o\"\n# [[tool.wikiview.board]]\nno_newline_at_end = 1",
			want: "[tool.wikiviewer]\nx = 1\n[[tool.other.board]]\nid = \"o\"\n# [[tool.wikiview.board]]\nno_newline_at_end = 1",
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := renameTables(c.in)
			if got != c.want {
				t.Errorf("got  %q\nwant %q", got, c.want)
			}
			if again := renameTables(got); again != got {
				t.Errorf("a second run changed it: %q", again)
			}
		})
	}
}

const oldBoards = "spec = \"0.1\"\n\n[tool.wikiview]\n\n# The sprint\n[[tool.wikiview.board]]\nid = \"sprint\"\npath = \"/backlog\"\n"

// The rewrite is saved, committed alone with the ADR's message, and read at
// once; the next read finds nothing to change and commits nothing.
func TestMigrationCommits(t *testing.T) {
	w, dir := fixture(t, map[string]string{"wiki.toml": oldBoards, "backlog/x.md": task("status: todo\n")})
	got, err := w.Boards("p")
	if err != nil || len(got) != 1 || got[0].ID != "sprint" || len(got[0].Cards) != 1 || len(got[0].Problems) != 0 {
		t.Fatalf("boards %+v %v, want the renamed board", got, err)
	}
	if toml := read(t, filepath.Join(dir, "wiki.toml")); toml != "spec = \"0.1\"\n\n\n# The sprint\n[[board]]\nid = \"sprint\"\npath = \"/backlog\"\n" {
		t.Errorf("wiki.toml = %q", toml)
	}
	if log := gitOut(t, dir, "log", "-1", "--format=%s", "--name-only"); log != renameMessage+"\n\nwiki/wiki.toml" {
		t.Errorf("commit = %q", log)
	}
	if _, err := w.Boards("p"); err != nil {
		t.Fatal(err)
	}
	if n := gitOut(t, dir, "rev-list", "--count", "HEAD"); n != "2" {
		t.Errorf("%s commits, want the fixture's and the rename's only", n)
	}
}

// wiki.toml mid-edit: the rewrite goes into the file, but waits for the
// lead's next commit rather than committing their edits as the CEO's.
func TestMigrationWaitsForUncommittedChanges(t *testing.T) {
	w, dir := fixture(t, map[string]string{"wiki.toml": "spec = \"0.1\"\n"})
	write(t, dir, map[string]string{"wiki.toml": oldBoards})
	if _, err := w.Boards("p"); err != nil {
		t.Fatal(err)
	}
	if toml := read(t, filepath.Join(dir, "wiki.toml")); !strings.Contains(toml, "\n[[board]]\n") {
		t.Errorf("wiki.toml = %q, want it renamed", toml)
	}
	if n := gitOut(t, dir, "rev-list", "--count", "HEAD"); n != "1" {
		t.Errorf("%s commits, want no new one", n)
	}
}

func TestMigrationReportsKeysLeftBehind(t *testing.T) {
	left := "wiki.toml: [tool.wikiview] is no longer read, and still holds: default_board, theme"
	w, _ := fixture(t, map[string]string{"wiki.toml": "spec = \"0.1\"\n\n[tool.wikiview]\ntheme = \"dark\"\ndefault_board = \"/backlog\"\n\n[[tool.wikiview.board]]\nid = \"a\"\npath = \"/\"\n\n[[tool.wikiview.board]]\nid = \"b\"\npath = \"/\"\n"})
	got, err := w.Boards("p")
	if err != nil || len(got) != 2 {
		t.Fatalf("boards %+v %v", got, err)
	}
	for _, b := range got {
		if !slices.Equal(b.Problems, []string{left}) {
			t.Errorf("%s: problems %q, want the keys left behind on every board", b.ID, b.Problems)
		}
	}

	// With no board left to show it on, it stands where the boards would be.
	w, _ = fixture(t, map[string]string{"wiki.toml": "spec = \"0.1\"\ntool.wikiview.default_board = \"/backlog\"\n"})
	got, _ = w.Boards("p")
	if len(got) != 1 || got[0].Name != "wiki.toml" || !slices.Equal(got[0].Problems, []string{"wiki.toml: [tool.wikiview] is no longer read, and still holds: default_board"}) {
		t.Errorf("boards %+v", got)
	}
}

// Board requests at once on one kept index: the decodes are the bundle's to
// make safe, and two at once without a lock crash the process.
func TestBoardsInParallel(t *testing.T) {
	ws := watch(t, "p")
	write(t, ws.dir("p"), map[string]string{"wiki.toml": sprint, "backlog/x.md": task("status: todo\n")})
	ws.next(t) // the index is kept from now on
	var wg sync.WaitGroup
	for range 32 {
		wg.Go(func() {
			if got, err := ws.wikis.Boards("p"); err != nil || len(got) != 2 {
				t.Errorf("boards %d %v", len(got), err)
			}
		})
	}
	wg.Wait()
}
