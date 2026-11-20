package wiki

// The move of a wiki's boards from [[tool.wikiview.board]] to the standard's
// [[board]] (and graphs to [[graph]]), done once by rewriting wiki.toml: see
// the HQ's research/board-graph-adr.md, "Migration". It is removed in Foreman's
// first release on or after 2027-01-01.

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/agentic-wiki/wiki/index"
)

// renameMessage is the commit message of the rewrite.
const renameMessage = "Rename [tool.wikiview] tables in wiki.toml to [[board]] and [[graph]]"

var (
	// oldView is the header of an old board or graph table, keeping its indent and what follows it.
	oldView = regexp.MustCompile(`^(\s*)\[\[\s*tool\.wikiview\.(board|graph)\s*\]\](.*)$`)
	// oldTool is a bare [tool.wikiview] header.
	oldTool = regexp.MustCompile(`^\s*\[\s*tool\.wikiview\s*\]\s*(#.*)?$`)
)

// renameTables renames the old board and graph headers, and deletes a bare
// [tool.wikiview] header whose table is empty. Every other byte stays, line
// endings and comments included. A file that also has [[board]] tables is
// renamed too: arrays of tables join, in file order.
func renameTables(text string) string {
	lines := strings.SplitAfter(text, "\n")
	var out strings.Builder
	for i, line := range lines {
		body := strings.TrimSuffix(line, "\n")
		switch m := oldView.FindStringSubmatch(body); {
		case m != nil:
			out.WriteString(m[1] + "[[" + m[2] + "]]" + m[3] + line[len(body):])
		case oldTool.MatchString(body) && emptyTable(lines[i+1:]):
			// deleted
		default:
			out.WriteString(line)
		}
	}
	return out.String()
}

// emptyTable says whether a table holds only blank lines and comments, up to
// the next header or the end of the file.
func emptyTable(rest []string) bool {
	for _, line := range rest {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "[") {
			return true
		}
		if t != "" && !strings.HasPrefix(t, "#") {
			return false
		}
	}
	return true
}

// migrate renames the old tables in the wiki.toml in dir, if it has any. It
// saves the file atomically, then commits it alone, as the CEO, unless it
// already had changes nobody committed (the lead's, mid-edit): then the
// rewrite goes with their next commit, as write's changes do. Called under
// writing. A commit that fails is only logged: the file is renamed already,
// and is committed with the next commit that takes it.
func (w *Wikis) migrate(dir string) error {
	file := filepath.Join(dir, "wiki.toml")
	raw, err := os.ReadFile(file)
	if err != nil {
		return nil // no wiki here: build says so
	}
	renamed := renameTables(string(raw))
	if renamed == string(raw) {
		return nil
	}
	// A symlink would have Foreman write wherever it points.
	info, err := os.Lstat(file)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return errors.New("wiki.toml is not a regular file")
	}
	ctx := context.Background()
	pending, err := w.git.Changed(ctx, dir, "wiki.toml")
	if err != nil {
		return err
	}
	if err := replace(file, []byte(renamed), info.Mode().Perm()); err != nil {
		return err
	}
	if !pending {
		if err := w.git.Commit(ctx, dir, renameMessage, "wiki.toml"); err != nil {
			slog.Warn("wiki: committing the renamed wiki.toml", "dir", dir, "err", err)
		}
	}
	return nil
}

// replace writes data to file through a temporary file and a rename, so no
// reader ever sees half of it. The temporary file is hidden, so the watcher
// ignores it.
func replace(file string, data []byte, perm fs.FileMode) error {
	tmp, err := os.CreateTemp(filepath.Dir(file), "."+filepath.Base(file)+"-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // after the rename, there is nothing left to remove
	_, err = tmp.Write(data)
	if err == nil {
		err = tmp.Chmod(perm)
	}
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	return os.Rename(tmp.Name(), file)
}

// configProblems are wiki.toml's problems that belong to no one board: the
// old tables couldn't be renamed (failed is why), or keys are left behind in
// [tool.wikiview], which nothing reads any more.
func configProblems(idx *index.Index, failed error) []string {
	if failed != nil {
		return []string{fmt.Sprintf("wiki.toml: could not rename the [tool.wikiview] tables to [[board]] and [[graph]]: %v", failed)}
	}
	if !slices.Contains(idx.Bundle.Tools(), "wikiview") {
		return nil
	}
	var left map[string]any
	_, _ = idx.Bundle.DecodeTool("wikiview", &left)
	return []string{"wiki.toml: [tool.wikiview] is no longer read, and still holds: " + strings.Join(slices.Sorted(maps.Keys(left)), ", ")}
}
