// Package wiki reads project wikis through the agentic-wiki engine and makes
// Foreman's few writes (wiki/product/architecture/wiki-engine.md). These are
// its wire types; the engine itself joins with the projects-and-wiki card.
package wiki

// Node is one folder or entry of a wiki's tree, in the reader's order:
// index.md first, then subfolders, then entries.
type Node struct {
	Path     string `json:"path"` // relative to the wiki, e.g. backlog/LED-44.md
	Title    string `json:"title"`
	Folder   bool   `json:"folder"`
	HasIndex bool   `json:"hasIndex"` // a folder that opens (it has an index.md)
	Changed  bool   `json:"changed"`  // changed since the CEO last looked
	Children []Node `json:"children"`
}

// Entry is one rendered entry, for the reader.
type Entry struct {
	Path        string            `json:"path"`
	Title       string            `json:"title"`
	Version     string            `json:"version"` // guards the writes
	Frontmatter map[string]string `json:"frontmatter"`
	HTML        string            `json:"html"` // rendered server-side, raw HTML off
	Headings    []Heading         `json:"headings"`
	Checkboxes  []Checkbox        `json:"checkboxes"`
	Backlinks   []string          `json:"backlinks"`
}

// Heading is an H2 or H3, for "On this page".
type Heading struct {
	Level  int    `json:"level"`
	Text   string `json:"text"`
	Anchor string `json:"anchor"`
}

// Checkbox is one `- [ ]` line the CEO can tick.
type Checkbox struct {
	Line int    `json:"line"`
	Done bool   `json:"done"`
	Text string `json:"text"`
}

// Board is a kanban over entries, declared in wiki.toml
// (wiki/product/features/board.md).
type Board struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Columns     []string `json:"columns"` // "Other" last when an entry fits none
	Lanes       []string `json:"lanes"`   // empty without a lane field
	Cards       []Card   `json:"cards"`
	Declaration string   `json:"declaration"` // quoted from wiki.toml
	Problems    []string `json:"problems"`
}

// Card is one entry on a board.
type Card struct {
	Path     string `json:"path"`
	ID       string `json:"id"` // the file's basename
	Title    string `json:"title"`
	Column   string `json:"column"`
	Lane     string `json:"lane"`
	Priority string `json:"priority"`
	Assignee string `json:"assignee"` // an agent id, when it matches one
	Version  string `json:"version"`
}
