package home

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

// debounce gathers a burst of file events (an editor's save, a git checkout)
// into one reload.
const debounce = 200 * time.Millisecond

// Watch follows the home on disk until ctx ends, calling notify with
// each change, from saves too (wiki/product/architecture/persistence.md#file-watching).
// notify must not block, nor call the home.
func (h *Home) Watch(ctx context.Context, notify func(Change)) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	h.mu.Lock()
	h.notify = notify
	h.mu.Unlock()
	h.watchDirs(watcher)
	go func() {
		defer watcher.Close()
		var timer <-chan time.Time
		for {
			select {
			case <-ctx.Done():
				return
			case err := <-watcher.Errors:
				slog.Warn("home watcher", "err", err)
			case e := <-watcher.Events:
				if timer == nil && h.relevant(e.Name) {
					timer = time.After(debounce)
				}
			case <-timer:
				timer = nil
				// Watch new folders (a new project repo) before reading them: a change made
				// in between would otherwise go unnoticed.
				h.watchDirs(watcher)
				h.reload()
			}
		}
	}()
	return nil
}

func (h *Home) reload() {
	h.mu.Lock()
	defer h.unlock()
	old := h.snap
	h.snap = h.scan(old)
	h.publish(diff(old, h.snap))
}

// watchDirs watches every folder whose entries make up the snapshot: the
// home (config/ and projects/ appearing), the global repo, teams/, projects/
// and each project repo. A project's code repos are
// anywhere, so they are checked on every reload and start, never watched.
// fsnotify watches a folder's direct entries; adding one twice is harmless.
func (h *Home) watchDirs(watcher *fsnotify.Watcher) {
	dirs := []string{h.root, h.config(), h.config(teamsDir), h.Path(projectsDir)}
	entries, _ := os.ReadDir(h.Path(projectsDir))
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, h.Path(projectsDir, e.Name()))
		}
	}
	for _, d := range dirs {
		if _, err := os.Stat(d); err == nil {
			_ = watcher.Add(d)
		}
	}
}

// relevant says whether a file event can change the snapshot. The home also
// holds state.db and the logs, which change all the time.
func (h *Home) relevant(path string) bool {
	rel, err := filepath.Rel(h.root, path)
	if err != nil {
		return false
	}
	parts := strings.Split(rel, string(filepath.Separator))
	switch {
	case parts[0] == configDir:
		// config/, its settings.toml, teams/ and its files
		return len(parts) == 1 || rel == filepath.Join(configDir, settingsFile) || parts[1] == teamsDir
	case parts[0] == projectsDir:
		// projects/, a project repo appearing or going, its project.toml
		return len(parts) <= 2 || (len(parts) == 3 && parts[2] == "project.toml")
	}
	return false
}
