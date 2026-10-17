package repos

import "github.com/brickpop/foreman/internal/git"

// Repo is one of a project's repos, with its branches.
type Repo struct {
	Name     string   `json:"name"`
	Project  bool     `json:"project"` // the project repo (project/), listed last
	Main     string   `json:"main"`
	Branches []Branch `json:"branches"` // main first, then by name
}

// Branch is one branch of a repo.
type Branch struct {
	Name        string `json:"name"`
	Ahead       int    `json:"ahead"`       // commits main doesn't have
	Uncommitted bool   `json:"uncommitted"` // its worktree has changes not committed yet
}

// File is a file at a ref: its text, or why there is none to show.
type File struct {
	Path     string `json:"path"`
	Size     int64  `json:"size"`
	Text     string `json:"text"`     // "" when binary or too large
	Binary   bool   `json:"binary"`   // has a NUL byte: not text
	TooLarge bool   `json:"tooLarge"` // over FileLimit
}

// LogPage is a page of a ref's history, newest first; Next is the skip of the
// following page, 0 when there is none.
type LogPage struct {
	Commits []git.Commit `json:"commits"`
	Next    int          `json:"next"`
}
