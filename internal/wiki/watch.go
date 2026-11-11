package wiki

import (
	"context"
	"io/fs"
	"log/slog"
	"maps"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/agentic-wiki/wiki/index"
	"github.com/fsnotify/fsnotify"

	"github.com/brickpop/foreman/internal/home"
)

// debounce gathers a burst of file events (a save, a `wiki tidy`, a git
// checkout) into one rebuild.
const debounce = 200 * time.Millisecond

// watched is one followed wiki as of its last rebuild. Never changed once
// made: a rebuild replaces it, so a read holding one stays consistent.
type watched struct {
	dir      string
	idx      *index.Index      // nil while the project has no wiki
	versions map[string]string // each entry's version, and wiki.toml's
	files    []string          // the files the entries link to that exist
}

// Watch follows every project's wiki until ctx ends: it keeps each one's
// index and calls publish with what each change changed. The follow it
// returns tells it the projects changed (it never blocks, so it can be called
// from the home's own notify). publish must not block.
func (w *Wikis) Watch(ctx context.Context, projects func() []string, publish func(Changed)) (follow func(), err error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	refollow := make(chan struct{}, 1)
	follow = func() {
		select {
		case refollow <- struct{}{}:
		default: // one is pending already
		}
	}
	w.follow(watcher, projects())
	go func() {
		defer watcher.Close()
		dirty := map[string]bool{}
		var timer <-chan time.Time
		for {
			select {
			case <-ctx.Done():
				return
			case <-refollow:
				w.follow(watcher, projects())
			case err := <-watcher.Errors:
				slog.Warn("wiki watcher", "err", err)
			case e := <-watcher.Events:
				// fsnotify isn't recursive: a new folder is watched as it appears,
				// and read, since files may land in it before its watch does.
				folder := e.Has(fsnotify.Create) && isDir(e.Name)
				if folder {
					addTree(watcher, e.Name)
				}
				if project := w.owner(e.Name); project != "" && (folder || relevant(e.Name)) {
					dirty[project] = true
					if timer == nil {
						timer = time.After(debounce)
					}
				}
			case <-timer:
				timer = nil
				for project := range dirty {
					if c, ok := w.rebuild(project); ok {
						publish(c)
					}
				}
				clear(dirty)
			}
		}
	}()
	return follow, nil
}

// follow watches the wikis of these projects, and forgets those that went.
func (w *Wikis) follow(watcher *fsnotify.Watcher, projects []string) {
	w.mu.Lock()
	for project := range w.watched {
		if !slices.Contains(projects, project) {
			delete(w.watched, project)
		}
	}
	w.mu.Unlock()
	for _, project := range projects {
		w.mu.Lock()
		_, known := w.watched[project]
		w.mu.Unlock()
		if known {
			continue
		}
		dir, idx, err := w.build(project)
		if dir == "" {
			continue
		}
		if err != nil {
			idx = nil // no wiki yet: followed, so one appearing is noticed
		}
		// The folder holding the wiki too, so a wiki folder made later is seen.
		_ = watcher.Add(filepath.Dir(dir))
		addTree(watcher, dir)
		w.mu.Lock()
		w.watched[project] = snapshot(dir, idx)
		w.mu.Unlock()
	}
}

// rebuild re-reads a followed wiki, and says what changed, if anything did.
func (w *Wikis) rebuild(project string) (Changed, bool) {
	w.mu.Lock()
	old := w.watched[project]
	w.mu.Unlock()
	if old == nil {
		return Changed{}, false
	}
	_, idx, err := w.build(project)
	if err != nil {
		idx = nil // gone, or mid-write: served as no wiki until the next change
	}
	now := snapshot(old.dir, idx)
	w.mu.Lock()
	// Compared with what is there now: a write's refresh may have swapped in
	// another index meanwhile, but it keeps the versions this compares.
	if old = w.watched[project]; old == nil { // forgotten meanwhile
		w.mu.Unlock()
		return Changed{}, false
	}
	w.watched[project] = now
	w.mu.Unlock()

	c := Changed{Project: project, Paths: []string{}, Versions: map[string]string{}}
	for p := range mergedKeys(old.versions, now.versions) {
		if old.versions[p] != now.versions[p] {
			c.Paths = append(c.Paths, p)
			c.Versions[p] = now.versions[p]
		}
	}
	slices.Sort(c.Paths)
	return c, len(c.Paths) > 0 || !slices.Equal(old.files, now.files)
}

func snapshot(dir string, idx *index.Index) *watched {
	s := &watched{dir: dir, idx: idx, versions: map[string]string{}}
	if idx == nil {
		return s
	}
	for _, e := range idx.Entries {
		if raw, err := e.Raw(); err == nil {
			s.versions[rel(e.Path)] = home.Version([]byte(raw))
		}
	}
	if raw, err := os.ReadFile(filepath.Join(dir, "wiki.toml")); err == nil {
		s.versions["wiki.toml"] = home.Version(raw)
	}
	for _, e := range idx.Entries {
		for _, l := range e.Links {
			if t := l.Target; !strings.HasSuffix(t, ".md") && idx.FileExists(t) && !slices.Contains(s.files, t) {
				s.files = append(s.files, t)
			}
		}
	}
	slices.Sort(s.files)
	return s
}

// owner is the followed project whose wiki holds file, or "".
func (w *Wikis) owner(file string) string {
	w.mu.Lock()
	defer w.mu.Unlock()
	for project, s := range w.watched {
		if r, err := filepath.Rel(s.dir, file); err == nil && r != ".." && !strings.HasPrefix(r, ".."+string(filepath.Separator)) {
			return project
		}
	}
	return ""
}

// relevant says whether a file can change what a wiki serves: its entries,
// its config, and the images they show. Hidden files (an editor's swap file,
// an atomic write's temp file) never do.
func relevant(file string) bool {
	base := filepath.Base(file)
	if strings.HasPrefix(base, ".") {
		return false
	}
	ext := strings.ToLower(path.Ext(base))
	return ext == ".md" || base == "wiki.toml" || images[ext] != ""
}

// addTree watches root and every folder under it but hidden ones (.git).
func addTree(watcher *fsnotify.Watcher, root string) {
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || !d.IsDir() {
			return nil // watch what can be watched
		}
		if p != root && strings.HasPrefix(d.Name(), ".") {
			return fs.SkipDir
		}
		_ = watcher.Add(p)
		return nil
	})
}

func isDir(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.IsDir()
}

func mergedKeys(a, b map[string]string) map[string]string {
	out := maps.Clone(a)
	maps.Copy(out, b)
	return out
}
