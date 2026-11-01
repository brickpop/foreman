package repos

import (
	"github.com/brickpop/foreman/internal/git"
	"github.com/brickpop/foreman/internal/wiki"
)

// Repo is one of a project's repos, with its branches.
type Repo struct {
	Name     string   `json:"name"`
	Project  bool     `json:"project"` // the project repo (project/), listed last
	Main     string   `json:"main"`
	Branches []Branch `json:"branches"` // main first, then newest commit first
}

// Branch is one branch of a repo.
type Branch struct {
	Name        string `json:"name"`
	Ahead       int    `json:"ahead"`       // commits main doesn't have
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
	Markdown    bool         `json:"markdown"`
	Frontmatter []wiki.Field `json:"frontmatter"`
	Body        string       `json:"body"`
}

// LogPage is a page of a ref's history, newest first; Next is the skip of the
// following page, 0 when there is none.
type LogPage struct {
	Commits []git.Commit `json:"commits"`
	Next    int          `json:"next"`
	Total   int          `json:"total"` // how many commits the history holds in all
}
