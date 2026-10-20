package home

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Folder is one folder of the repo browser (wiki/product/features/add-project.md#picking-repos):
// its subfolders, the git repos among them marked.
type Folder struct {
	Path    string        `json:"path"`             // a real path
	Parent  string        `json:"parent,omitempty"` // "" at the top: the browser never goes above Start
	Entries []FolderEntry `json:"entries"`          // its subfolders by name, hidden ones left out
}

// FolderEntry is a subfolder in the browser.
type FolderEntry struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Repo bool   `json:"repo"` // a git repo's top folder (its own .git folder): one that can be added
}

// Browse lists a folder for the repo browser: path, or Start when empty. It
// must be inside Start, and so must each subfolder listed (a symlink leading
// out is left out).
func (h *Home) Browse(path string) (Folder, error) {
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
	if !inside(real, h.start) {
		return Folder{}, fmt.Errorf("%w: %s", ErrInvalid, h.outsideStart(path))
	}
	entries, err := os.ReadDir(real)
	if err != nil {
		return Folder{}, fmt.Errorf("%w: %s can't be read: %s", ErrInvalid, path, err.Error())
	}
	f := Folder{Path: real, Entries: []FolderEntry{}}
	if real != h.start {
		f.Parent = filepath.Dir(real)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".") {
			continue
		}
		child, err := filepath.EvalSymlinks(filepath.Join(real, e.Name()))
		if err != nil || !inside(child, h.start) {
			continue
		}
		if info, err := os.Stat(child); err != nil || !info.IsDir() {
			continue
		}
		f.Entries = append(f.Entries, FolderEntry{Name: e.Name(), Path: filepath.Join(real, e.Name()), Repo: topOfRepo(child)})
	}
	sort.Slice(f.Entries, func(i, j int) bool { return strings.ToLower(f.Entries[i].Name) < strings.ToLower(f.Entries[j].Name) })
	return f, nil
}
