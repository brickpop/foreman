package wiki

import (
	"fmt"
	"path"
	"regexp"
	"slices"
	"strings"

	"github.com/agentic-wiki/wiki/index"

	"github.com/brickpop/foreman/internal/home"
)

// declaration is one [[board]] of wiki.toml, the keys of the standard's
// board that Foreman shows (wiki/product/features/board.md#board-declaration).
type declaration struct {
	ID      string   `toml:"id"`
	Path    string   `toml:"path"`
	Name    string   `toml:"name"`
	Where   []string `toml:"where"`
	Status  string   `toml:"status"`
	Columns []string `toml:"columns"`
	Lane    string   `toml:"lane"`
	Lanes   []string `toml:"lanes"`

	filters []index.PropFilter
}

// knownKeys are the standard's board keys: Foreman's, and blockers, which
// Foreman doesn't show.
var knownKeys = []string{"id", "path", "name", "where", "status", "columns", "lane", "lanes", "blockers"}

// Boards are the boards declared in the project wiki's wiki.toml, with their
// cards. A declaration with problems is still shown when it can be, with them.
func (w *Wikis) Boards(project string) ([]Board, error) {
	idx, err := w.open(project)
	if err != nil {
		return nil, err
	}
	w.mu.Lock()
	failed := w.unrenamed[project]
	w.mu.Unlock()
	// wiki.toml's own problems show on every board, whichever is open.
	config := configProblems(idx, failed)
	decls, problems, err := declarations(idx)
	if err != nil {
		// A value of the wrong type: shown where the boards would be.
		return []Board{problemBoard(append(config, err.Error()))}, nil
	}
	if len(decls) == 0 && len(config) > 0 {
		return []Board{problemBoard(config)}, nil
	}
	out := []Board{}
	for i, d := range decls {
		b := Board{ID: d.ID, Name: d.Name, Columns: []string{}, Lanes: []string{}, Cards: []Card{}, Unread: []Unread{}}
		if d.Path != "" {
			b = build(idx, d)
		}
		// One that can't be addressed is still listed, so its problems show;
		// moves to it are refused, since no declaration has that id.
		if d.ID == "" || slices.ContainsFunc(out, func(o Board) bool { return o.ID == d.ID }) {
			b.ID = fmt.Sprintf("#%d", i+1)
		}
		b.Problems = append(append([]string{}, config...), problems[i]...)
		out = append(out, b)
	}
	return out, nil
}

// problemBoard stands where the boards would be, to show wiki.toml's problems.
func problemBoard(problems []string) Board {
	return Board{ID: "#1", Name: "wiki.toml", Columns: []string{}, Lanes: []string{}, Cards: []Card{}, Unread: []Unread{}, Problems: problems}
}

// validID is the standard's board id: lowercase letters, digits, _ and -,
// starting with a letter or a digit.
var validID = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)

// declarations decodes and checks the boards, with each one's problems.
func declarations(idx *index.Index) ([]declaration, [][]string, error) {
	// No lock of Foreman's: the bundle's own makes these decodes safe at once.
	var decls []declaration
	var loose []map[string]any
	if _, err := idx.Bundle.DecodeBoards(&decls); err != nil {
		return nil, nil, fmt.Errorf("wiki.toml: %w", err)
	}
	_, _ = idx.Bundle.DecodeBoards(&loose)
	problems := make([][]string, len(decls))
	seen := map[string]bool{}
	for i := range decls {
		d := &decls[i]
		add := func(format string, a ...any) { problems[i] = append(problems[i], fmt.Sprintf(format, a...)) }
		for key := range loose[i] {
			if !slices.Contains(knownKeys, key) {
				add("unknown key %q", key)
			}
		}
		switch {
		case d.ID == "":
			add("id is required")
		case !validID.MatchString(d.ID):
			add("id %q may hold only lowercase letters, digits, _ and -, and starts with a letter or a digit", d.ID)
		case seen[d.ID]:
			add("id %q is used by another board", d.ID)
		}
		seen[d.ID] = true
		if d.Path == "" {
			add("path is required")
		}
		if d.Status == "" {
			d.Status = "status"
		}
		if d.Where == nil {
			d.Where = []string{"type=task"}
		}
		for _, where := range d.Where {
			f, err := index.ParseFilter(where)
			if err != nil {
				add("%v", err)
				continue
			}
			d.filters = append(d.filters, f)
		}
		if d.Name == "" {
			d.Name = "Wiki" // over the whole wiki
			if folder := strings.Trim(d.Path, "/"); folder != "" {
				d.Name = folderTitle(folder)
			}
		}
	}
	return decls, problems, nil
}

// build lays a board out as WikiShell does: the declared columns in order,
// then every other status found, each its own column, then the cards with
// none ("") when there are some. Lanes alike. No card is ever left out.
func build(idx *index.Index, d declaration) Board {
	b := Board{ID: d.ID, Name: d.Name, StatusField: d.Status, LaneField: d.Lane, Columns: []string{}, Lanes: []string{}, Cards: []Card{}, Unread: []Unread{}}
	for _, e := range idx.Filter(d.prefix(), nil) {
		if reason := frontmatterError(e); reason != "" {
			b.Unread = append(b.Unread, Unread{Path: rel(e.Path), Label: title(e), Error: reason})
		}
	}
	slices.SortFunc(b.Unread, func(a, c Unread) int { return strings.Compare(a.Path, c.Path) })
	var statuses, lanes []string
	for _, e := range cards(idx, d) {
		raw, err := e.Raw()
		if err != nil {
			continue
		}
		c := Card{
			Path:     rel(e.Path),
			ID:       strings.TrimSuffix(path.Base(e.Path), ".md"),
			Title:    title(e),
			Column:   e.Field(d.Status),
			Priority: e.Field("priority"),
			Assignee: e.Field("assignee"),
			Version:  home.Version([]byte(raw)),
		}
		if d.Lane != "" {
			c.Lane = e.Field(d.Lane)
			lanes = append(lanes, c.Lane)
		}
		statuses = append(statuses, c.Column)
		b.Cards = append(b.Cards, c)
	}
	slices.SortFunc(b.Cards, func(a, c Card) int { return strings.Compare(a.Path, c.Path) })
	b.Columns = ordered(d.Status, d.Columns, statuses)
	if d.Lane != "" {
		b.Lanes = ordered(d.Lane, d.Lanes, lanes)
	}
	return b
}

// knownOrder is how common vocabularies read, for values no declaration
// orders (WikiShell's table): "high, low, medium" isn't a priority order.
var knownOrder = map[string][]string{
	"status":   {"backlog", "todo", "next", "in-progress", "doing", "in-review", "review", "blocked", "done", "cancelled", "archived"},
	"priority": {"urgent", "high", "medium", "low"},
	"severity": {"critical", "major", "minor", "trivial"},
	"size":     {"xs", "s", "m", "l", "xl"},
}

// ordered arranges an axis: the declared values (even empty ones), then the
// values found in the field's known order, then the rest alphabetically,
// then "" (no value) when some card has none.
func ordered(field string, declared, found []string) []string {
	out := []string{}
	take := func(v string) {
		if v != "" && !slices.Contains(out, v) {
			out = append(out, v)
		}
	}
	for _, v := range declared {
		take(v)
	}
	for _, v := range knownOrder[field] {
		if slices.Contains(found, v) {
			take(v)
		}
	}
	rest := slices.Clone(found)
	slices.Sort(rest)
	for _, v := range rest {
		take(v)
	}
	if slices.Contains(found, "") {
		out = append(out, "")
	}
	return out
}

// onBoard finds the card at p on the board: the board says which fields a move writes.
func onBoard(idx *index.Index, d declaration, p string) (*index.Entry, bool) {
	for _, e := range cards(idx, d) {
		if rel(e.Path) == p {
			return e, true
		}
	}
	return nil, false
}

// cards are the entries the board shows as cards: those under its path that
// match its where, less any whose frontmatter couldn't be read. That one has
// no fields, so it would match an empty where, or one of only != filters, as
// a card with no status; the board lists it in Unread instead.
func cards(idx *index.Index, d declaration) []*index.Entry {
	return slices.DeleteFunc(idx.Filter(d.prefix(), d.filters), func(e *index.Entry) bool { return e.FrontmatterErr() != nil })
}

// prefix is the folder the board's cards come from, as Filter takes it ("" for
// the whole wiki). With its trailing slash: Filter matches a plain string
// prefix, so "/backlog" would also take "/backlogs/".
func (d declaration) prefix() string {
	if folder := strings.Trim(d.Path, "/"); folder != "" {
		return "/" + folder + "/"
	}
	return ""
}
