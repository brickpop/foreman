package git

import "time"

// TreeEntry is one entry of a folder at a commit.
type TreeEntry struct {
	Name string `json:"name"`
	Kind string `json:"kind"` // dir, file, link (a symlink) or module (a submodule)
	Size int64  `json:"size"` // a file's bytes; 0 otherwise
}

// Commit is one commit of a log.
type Commit struct {
	Hash    string    `json:"hash"`
	Author  string    `json:"author"`
	Date    time.Time `json:"date"`
	Subject string    `json:"subject"`
	Parents []string  `json:"parents"` // more than one: a merge
	// Branches are the branches pointing at it.
	Branches []string `json:"branches"`
}
