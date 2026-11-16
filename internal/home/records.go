package home

import (
	"context"
	"fmt"
	"maps"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/brickpop/foreman/internal/git"
)

// projects.toml, in Foreman's HQ: every project's portable record, which
// travels with the HQ between machines (wiki/product/models/project-config.md#the-project-record).
// What belongs to this machine (its profile, where each repo is) is in the
// database; a project's rules are in its own HQ.

// projectsFile is projects.toml's name in Foreman's HQ.
const projectsFile = "projects.toml"

// Record is a project as projects.toml keeps it, under its id.
type Record struct {
	DisplayName string `toml:"display_name,omitempty"` // "" shows the id readable
	Hidden      bool   `toml:"hidden,omitempty"`
	// HQ is its HQ's origin, the URL a new machine clones it from; "" when it
	// has none.
	HQ    string                `toml:"hq,omitempty"`
	Repos map[string]RepoRecord `toml:"repos,omitempty"`
}

// RepoRecord is one of a project's code repos, wherever it is.
type RepoRecord struct {
	// Remote is its origin when it was added, where a new machine can clone
	// it from; "" when it had none.
	Remote        string `toml:"remote,omitempty"`
	RootCommit    string `toml:"root_commit"` // which only this repo holds: the same repo on any machine
	DefaultBranch string `toml:"default_branch"`
}

// Records is projects.toml as read: the records by id, its version, and what
// is wrong with it. A file with problems keeps the last good records.
type Records struct {
	Records  map[string]Record
	Version  string
	Problems []string
}

var commitHash = regexp.MustCompile(`^[0-9a-f]{40}([0-9a-f]{24})?$`)

// ParseRecords parses projects.toml, reporting what Foreman can't use: an id
// or a repo name it couldn't make a folder of, two that differ only in case,
// a repo with no root commit or default branch.
func ParseRecords(data []byte) (map[string]Record, []string) {
	records := map[string]Record{}
	_, p := decode(data, &records)
	ids := map[string]string{}
	for _, id := range sortedKeys(records) {
		r := records[id]
		switch {
		case !validProjectName(id):
			p.add("%s: not a project id (lowercase letters, digits and single hyphens, at most 63)", id)
		case ids[strings.ToLower(id)] != "":
			p.add("%s: the same id as %s, in another case", id, ids[strings.ToLower(id)])
		}
		ids[strings.ToLower(id)] = id
		if utf8.RuneCountInString(r.DisplayName) > maxProjectName || strings.ContainsFunc(r.DisplayName, unicode.IsControl) {
			p.add("%s.display_name: one line of at most %d characters", id, maxProjectName)
		}
		names := map[string]string{}
		for _, name := range sortedKeys(r.Repos) {
			repo := r.Repos[name]
			at := fmt.Sprintf("%s.repos.%s", id, name)
			switch {
			case !validName(name):
				p.add("%s: a repo name is %s", at, nameRules)
			case strings.EqualFold(name, hqRepo):
				p.add("%s: %q is the HQ's name", at, name)
			case names[strings.ToLower(name)] != "":
				p.add("%s: the same name as %s, in another case", at, names[strings.ToLower(name)])
			}
			names[strings.ToLower(name)] = name
			if !commitHash.MatchString(repo.RootCommit) {
				p.add("%s.root_commit: a commit's full hash", at)
			}
			if repo.DefaultBranch == "" || git.AgentBranch(repo.DefaultBranch) {
				p.add("%s.default_branch: a branch of the CEO's (not wt/* or lead/*)", at)
			}
		}
		if r.Repos == nil {
			r.Repos = map[string]RepoRecord{}
			records[id] = r
		}
	}
	return records, normalize(p)
}

// changeRecords applies change to a copy of projects.toml's records and
// writes the file, committed to Foreman's HQ with message; the home is
// rescanned. Refused while the file has problems: the CEO fixes it first,
// as Foreman would write over what they meant. Called with h.mu held.
func (h *Home) changeRecords(ctx context.Context, message string, change func(map[string]Record)) error {
	if p := h.snap.records.Problems; len(p) > 0 {
		return fmt.Errorf("%w: %s has problems, fix it first: %s", ErrInvalid, projectsFile, strings.Join(p, "; "))
	}
	records := make(map[string]Record, len(h.snap.records.Records))
	for id, r := range h.snap.records.Records {
		r.Repos = maps.Clone(r.Repos)
		records[id] = r
	}
	change(records)
	data, err := encode(records)
	if err != nil {
		return err
	}
	return h.write(ctx, h.HQ(projectsFile), h.snap.records.Version, data, message)
}

// changeRecord changes the project's record (changeRecords) and returns the
// project as rescanned.
func (h *Home) changeRecord(ctx context.Context, id, message string, change func(*Record)) (Project, error) {
	err := h.changeRecords(ctx, message, func(records map[string]Record) {
		r := records[id]
		change(&r)
		records[id] = r
	})
	if err != nil {
		return Project{}, err
	}
	return h.found(id)
}
