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
	if w.watcher, err = fsnotify.NewWatcher(); err != nil {
		return nil, err
	}
	refollow := make(chan struct{}, 1)
	follow = func() {
		select {
		case refollow <- struct{}{}:
		default: // one is pending already
		}
	}
	w.follow(projects())
	go func() {
		defer w.watcher.Close()
		dirty := map[string]bool{}
		var timer <-chan time.Time
		for {
			select {
			case <-ctx.Done():
				return
			case <-refollow:
				w.follow(projects())
			case err := <-w.watcher.Errors:
				slog.Warn("wiki watcher", "err", err)
			case e := <-w.watcher.Events:
				// Events outside every wiki come from the folder that holds one,
				// watched only so the wiki can appear: nothing else there is followed.
				project, s := w.owner(e.Name)
				if s == nil {
					continue
				}
				// A folder that moved or went takes its watches with it. fsnotify
				// drops only the folder's own, and on macOS the ones below it
				// would stay open under the new name, beside a second set.
				if e.Has(fsnotify.Rename) || e.Has(fsnotify.Remove) {
					w.unwatch(e.Name, nil)
				}
				// fsnotify isn't recursive: a new folder is watched as it appears,
				// and read, since files may land in it before its watch does.
				folder := e.Has(fsnotify.Create) && isDir(e.Name)
				if folder && s.follows(e.Name) {
					w.addTree(e.Name, s.follows)
				}
				if folder || relevant(e.Name) {
					dirty[project] = true
					if timer == nil {
						timer = time.After(debounce)
					}
				}
			case <-timer:
				timer = nil
				for project := range dirty {
					c, ok := w.rebuild(project)
					if !ok {
						continue
					}
					if slices.Contains(c.Paths, "wiki.toml") {
						w.rewatch(project) // its ignore may have changed
					}
					publish(c)
				}
				clear(dirty)
			}
		}
	}()
	return follow, nil
}

// follow watches the wikis of these projects, and forgets those that went,
// with their watches.
func (w *Wikis) follow(projects []string) {
	w.mu.Lock()
	var gone []string
	for project, s := range w.watched {
		if !slices.Contains(projects, project) {
			delete(w.watched, project)
			gone = append(gone, s.dir)
		}
	}
	w.mu.Unlock()
	for _, dir := range gone {
		w.unwatch(dir, nil)
		_ = w.watcher.Remove(filepath.Dir(dir))
	}
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
		_ = w.watcher.Add(filepath.Dir(dir))
		s := snapshot(dir, idx)
		w.addTree(dir, s.follows)
		w.mu.Lock()
		w.watched[project] = s
		w.mu.Unlock()
	}
}

// rewatch brings a wiki's watches in line with its config: added first, then
// the ones it no longer follows dropped, so no change slips through between.
func (w *Wikis) rewatch(project string) {
	w.mu.Lock()
	s := w.watched[project]
	w.mu.Unlock()
	if s == nil {
		return
	}
	w.addTree(s.dir, s.follows)
	w.unwatch(s.dir, s.follows)
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

// owner is the followed project whose wiki holds file, and the wiki as of its
// last rebuild, or "" and nil.
func (w *Wikis) owner(file string) (string, *watched) {
	w.mu.Lock()
	defer w.mu.Unlock()
	for project, s := range w.watched {
		if within(s.dir, file) {
			return project, s
		}
	}
	return "", nil
}

// follows says whether a folder of this wiki is watched: the wiki's own
// folder, always (it holds wiki.toml), and the folders below it that the
// engine's Build enters (Index.SkipsFolder: not hidden ones like .git, nor
// those wiki.toml's ignore covers). Only those can hold an entry.
//
// One edge: a link to an ignored file still resolves, so an image an entry
// shows may sit in an ignored folder, and that image appearing or going sends
// no wiki.changed until the next change elsewhere in the wiki.
func (s *watched) follows(folder string) bool {
	if !within(s.dir, folder) {
		return false
	}
	r, _ := filepath.Rel(s.dir, folder)
	if r == "." {
		return true
	}
	idx := s.idx
	if idx == nil {
		idx = &noWiki
	}
	return !idx.SkipsFolder(r)
}

// noWiki stands in while a wiki can't be read (none yet, or caught mid-write):
// with no ignore to apply, its SkipsFolder still skips hidden folders.
var noWiki index.Index

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

// addTree watches root and every folder under it that follows says to.
func (w *Wikis) addTree(root string, follows func(folder string) bool) {
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || !d.IsDir() {
			return nil // watch what can be watched
		}
		if !follows(p) {
			return fs.SkipDir
		}
		_ = w.watcher.Add(p)
		return nil
	})
}

// unwatch drops the watches at root and below it, but those keep says to keep
// (all of them when keep is nil).
func (w *Wikis) unwatch(root string, keep func(folder string) bool) {
	for _, p := range w.watcher.WatchList() {
		if within(root, p) && (keep == nil || !keep(p)) {
			_ = w.watcher.Remove(p)
		}
	}
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
