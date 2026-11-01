// Package wiki reads project wikis through the agentic-wiki engine and makes
// Foreman's few writes (wiki/product/architecture/wiki-engine.md). Paths on
// the wire are relative to the wiki, e.g. backlog/LED-44.md.
package wiki

// Node is one folder or entry of a wiki's tree, in the reader's order:
// index.md first, then subfolders, then entries. A folder's own index.md
// isn't among its children: opening the folder opens it.
type Node struct {
	Path     string `json:"path"` // relative to the wiki, e.g. backlog/LED-44.md
	Title    string `json:"title"`
	Folder   bool   `json:"folder"`
	HasIndex bool   `json:"hasIndex"` // a folder that opens (it has an index.md)
	Changed  bool   `json:"changed"`  // changed since the CEO last looked
	Children []Node `json:"children"`
}

// Entry is one entry, for the reader: its Markdown exactly as on disk, and
// lookup tables for what the browser must not work out itself (the engine's
// rules), as wikiview serves it.
type Entry struct {
	Path        string     `json:"path"`
	Title       string     `json:"title"`
	Version     string     `json:"version"`     // guards the writes
	Frontmatter []Field    `json:"frontmatter"` // by key
	Body        string     `json:"body"`        // frontmatter stripped
	Links       []Link     `json:"links"`
	Headings    []Heading  `json:"headings"`
	Checkboxes  []Checkbox `json:"checkboxes"`
	Backlinks   []Backlink `json:"backlinks"`
}

// Field is one frontmatter field, as the reader shows it: its values in
// order, a scalar being a list of one, none for an empty one.
type Field struct {
	Key    string  `json:"key"`
	Values []Value `json:"values"`
}

// Value is one value of a field. To is the entry it names (a value ending in
// .md, as the engine reads references), with Title its title; both "" otherwise.
type Value struct {
	Text  string `json:"text"`
	To    string `json:"to"`
	Title string `json:"title"`
}

// Link is one link of the body, resolved by the engine. Raw is the lookup
// key: the href exactly as the Markdown renderer meets it.
type Link struct {
	Raw     string `json:"raw"`
	To      string `json:"to"`      // the wiki path; empty when Outside
	Anchor  string `json:"anchor"`  // the fragment without '#'
	Exists  bool   `json:"exists"`  // an entry; false for one not written yet
	File    bool   `json:"file"`    // a file the wiki carries, not an entry
	Image   bool   `json:"image"`   // a file the reader shows (served by /image)
	Outside bool   `json:"outside"` // resolves above the wiki: not navigable
}

// Heading is the id one heading takes, which its anchor links resolve against.
// BodyLine counts the body as served: what the renderer matches by.
type Heading struct {
	ID       string `json:"id"`
	BodyLine int    `json:"bodyLine"`
}

// Checkbox is one `- [ ]` line the CEO can tick. Line counts the whole file,
// frontmatter included (what a write is addressed by); BodyLine counts the
// body (what the renderer matches by).
type Checkbox struct {
	Line     int    `json:"line"`
	BodyLine int    `json:"bodyLine"`
	Done     bool   `json:"done"`
	Text     string `json:"text"`
}

// Backlink is an entry linking to this one.
type Backlink struct {
	From  string `json:"from"`
	Title string `json:"title"` // the linking entry's title
}

// Changed is the wiki.changed event: what a rebuild of a project's wiki
// found different (wiki/product/architecture/api.md#events).
type Changed struct {
	Project string `json:"project"`
	// Paths are the entries added, edited or removed, and wiki.toml when it
	// changed. Empty when only a file the entries link to (an image) came or went.
	Paths    []string          `json:"paths"`
	Versions map[string]string `json:"versions"` // each path's new version, "" once it is gone
}

// Board is a kanban over entries, declared in wiki.toml
// (wiki/product/features/board.md).
type Board struct {
	ID          string   `json:"id"` // "#n" when the declaration has none (or a taken one): moves refused
	Name        string   `json:"name"`
	Columns     []string `json:"columns"`     // declared, then found; "" (no status) last when a card has none
	Lanes       []string `json:"lanes"`       // as Columns; empty without a lane field
	StatusField string   `json:"statusField"` // the field the columns come from, e.g. status
	LaneField   string   `json:"laneField"`   // the field the lanes come from, e.g. priority
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
	Assignee string `json:"assignee"` // the field as written; the UI shows the agent it names
	Version  string `json:"version"`
}
