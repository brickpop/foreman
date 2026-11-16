package home

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Folder is one folder of the repo browser (wiki/product/features/add-project.md#picking-repos):
// whether it is a git repo, and its subfolders, the git repos among them marked.
type Folder struct {
	Path    string        `json:"path"`              // a real path
	Home    string        `json:"home"`              // the CEO's home folder (real path): the browser never goes above it
	Repo    bool          `json:"repo"`              // a git repo's top folder: one that can be added
	Added   string        `json:"added,omitempty"`   // the project's repo this folder is already, when browsed for a project
	Foreman string        `json:"foreman,omitempty"` // Foreman's own folder (reserved): what it is; nothing is added there
	Entries []FolderEntry `json:"entries"`           // its subfolders by name, hidden ones left out
}

// FolderEntry is a subfolder in the browser.
type FolderEntry struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Repo bool   `json:"repo"` // a git repo's top folder (its own .git folder): one that can be added
	// Foreman's own folder (its HQ, projects/, worktrees/): what it is. Not
	// opened: its repos (HQs, worktrees) aren't code repos.
	Foreman string `json:"foreman,omitempty"`
}

// Browse lists a folder for the repo browser: path, or Start when empty. It
// must be inside the CEO's home folder, and each subfolder listed usable
// (Home.usable): a symlink leading out, or Foreman's home, is left out. For a
// project ("" for none), it says which of its repos the folder already is,
// by real path, as AddRepo refuses it.
func (h *Home) Browse(path, project string) (Folder, error) {
	var repos []Repo
	if project != "" {
		p, err := h.Project(project)
		if err != nil {
			return Folder{}, err
		}
		repos = p.Repos
	}
	if path == "" {
		path = h.start
	}
	if !filepath.IsAbs(path) {
		return Folder{}, fmt.Errorf("%w: %s is not an absolute path", ErrInvalid, path)
	}
	real, err := filepath.EvalSymlinks(path)
	if err != nil {
		return Folder{}, fmt.Errorf("%w: %s is not a folder", ErrInvalid, path)
	}
	if !inside(real, h.allowed) {
		return Folder{}, fmt.Errorf("%w: %s is outside your home folder (%s)", ErrInvalid, path, h.allowed)
	}
	entries, err := os.ReadDir(real)
	if err != nil {
		return Folder{}, fmt.Errorf("%w: %s can't be read: %s", ErrInvalid, path, err.Error())
	}
	h.mu.Lock()
	reserved := h.reserved()
	h.mu.Unlock()
	f := Folder{Path: real, Home: h.allowed, Repo: topOfRepo(real), Foreman: reserved[real], Entries: []FolderEntry{}}
	for _, r := range repos {
		if other, err := filepath.EvalSymlinks(r.Path); err == nil && other == real {
			f.Added = r.Name
		}
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".") {
			continue
		}
		child, err := filepath.EvalSymlinks(filepath.Join(real, e.Name()))
		// Out of bounds (a symlink leading out, Foreman's home): left out.
		if err != nil || h.usable(child, child) != nil {
			continue
		}
		if info, err := os.Stat(child); err != nil || !info.IsDir() {
			continue
		}
		f.Entries = append(f.Entries, FolderEntry{Name: e.Name(), Path: filepath.Join(real, e.Name()), Repo: topOfRepo(child), Foreman: reserved[child]})
	}
	sort.Slice(f.Entries, func(i, j int) bool { return strings.ToLower(f.Entries[i].Name) < strings.ToLower(f.Entries[j].Name) })
	return f, nil
}

// reserved maps Foreman's own folders, by real path, to what each is: those
// of its folder that are Foreman's, all but repos/ (where the code repos it
// clones go). Called with h.mu held.
func (h *Home) reserved() map[string]string {
	return map[string]string{
		h.HQ():                                "Foreman's HQ",
		filepath.Join(h.folder, projectsDir):  "Foreman's projects",
		filepath.Join(h.folder, worktreesDir): "the agents' worktrees",
	}
}

// outsideForemans refuses a code repo's folder (real) in one of Foreman's own
// (Home.reserved): the browser never opens them, and a pasted or scripted
// path is refused the same. Called with h.mu held.
func (h *Home) outsideForemans(real string) error {
	for folder, what := range h.reserved() {
		if inside(real, folder) {
			return fmt.Errorf("%w: %s is in %s, Foreman's own: no code repo goes there", ErrInvalid, real, what)
		}
	}
	return nil
}

// Branches are a repo's branches before it is added, to pick its main branch
// when it has no main.
type Branches struct {
	Branches []string `json:"branches"` // by name
	Current  string   `json:"current"`  // its checkout's; "" when detached
}

// RepoBranches lists the branches of a folder to add as a repo, one Foreman
// may use (Home.usable).
func (h *Home) RepoBranches(ctx context.Context, path string) (Branches, error) {
	real, err := h.repoFolder(path)
	if err != nil {
		return Branches{}, fmt.Errorf("%w: %s", ErrInvalid, err.Error())
	}
	if err := h.usable(path, real); err != nil {
		return Branches{}, err
	}
	list, err := h.git.Branches(ctx, real, "")
	if err != nil {
		return Branches{}, err
	}
	if list == nil {
		list = []string{} // a repo without a commit has none: [], never null to the UI
	}
	current, _ := h.git.Branch(ctx, real)
	return Branches{Branches: list, Current: current}, nil
}
