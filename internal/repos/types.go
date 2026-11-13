package repos

import (
	"github.com/brickpop/foreman/internal/git"
	"github.com/brickpop/foreman/internal/wiki"
)

// Repo is one of a project's repos, with its branches.
type Repo struct {
	Name     string   `json:"name"`
	Project  bool     `json:"project"` // the project repo (project/), listed last
	Default  string   `json:"defaultBranch"`
	Branches []Branch `json:"branches"` // the default first, then newest commit first
	Idle     []Idle   `json:"idle"`     // the agents' worktrees of it on no branch, by agent
}

// Idle is an agent's worktree of a repo that sits on no branch: opened, and no
// topic yet (wiki/product/architecture/sandboxes-and-git.md#opening-a-repo).
type Idle struct {
	Agent       string `json:"agent"`
	Uncommitted bool   `json:"uncommitted"` // it has changes not committed yet
}

// Branch is one branch of a repo.
type Branch struct {
	Name        string `json:"name"`
	Ahead       int    `json:"ahead"`       // commits the default branch doesn't have
	Uncommitted bool   `json:"uncommitted"` // its worktree has changes not committed yet
	Agent       string `json:"agent"`       // whose worktree has it checked out (an agent id); "" for none
}

// File is a file at a ref: its text, or why there is none to show.
type File struct {
	Path     string `json:"path"`
	Size     int64  `json:"size"`
	Text     string `json:"text"`     // "" when binary or too large
	Binary   bool   `json:"binary"`   // has a NUL byte: not text
	TooLarge bool   `json:"tooLarge"` // over FileLimit
	// A Markdown file (.md, .markdown) has its frontmatter apart, and Body is
	// its text without it; Text stays whole, for its source.
	Markdown         bool         `json:"markdown"`
	Frontmatter      []wiki.Field `json:"frontmatter"`
	FrontmatterError string       `json:"frontmatterError,omitempty"` // why the frontmatter couldn't be read
	Body             string       `json:"body"`
	// Its body's links, resolved, and its headings' ids (Markdown only).
	Links    []Link         `json:"links"`
	Headings []wiki.Heading `json:"headings"`
}

// LogPage is a page of a ref's history, newest first; Next is the skip of the
// following page, 0 when there is none.
type LogPage struct {
	Commits []git.Commit `json:"commits"`
	Next    int          `json:"next"`
	Total   int          `json:"total"` // how many commits the history holds in all
}

// Link is one link of a Markdown file's body, resolved against the commit, so
// the browser looks it up and never works the rules out itself. Raw is the
// lookup key: the href exactly as written. To is a path in the repo ("" for
// its root); Outside is a link out of the repo, shown as text.
type Link struct {
	Raw     string `json:"raw"`
	To      string `json:"to"`
	Anchor  string `json:"anchor"`  // the fragment without '#'
	Exists  bool   `json:"exists"`  // the commit has it
	Folder  bool   `json:"folder"`  // a folder: opened as its page
	Image   bool   `json:"image"`   // a file served by …/image
	Outside bool   `json:"outside"` // climbs out of the repo
}
