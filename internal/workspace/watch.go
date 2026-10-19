package workspace

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

// Watch follows the workspace on disk until ctx ends, calling notify with
// each change, from saves too (wiki/product/architecture/persistence.md#file-watching).
// notify must not block, nor call the workspace.
func (w *Workspace) Watch(ctx context.Context, notify func(Change)) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	w.mu.Lock()
	w.notify = notify
	w.mu.Unlock()
	w.watchDirs(watcher)
	go func() {
		defer watcher.Close()
		var timer <-chan time.Time
		for {
			select {
			case <-ctx.Done():
				return
			case err := <-watcher.Errors:
				slog.Warn("workspace watcher", "err", err)
			case e := <-watcher.Events:
				if timer == nil && w.relevant(e.Name) {
					timer = time.After(debounce)
				}
			case <-timer:
				timer = nil
				// Watch new folders (new projects) before reading them: a change made
				// in between would otherwise go unnoticed.
				w.watchDirs(watcher)
				w.reload()
			}
		}
	}()
	return nil
}

func (w *Workspace) reload() {
	w.mu.Lock()
	defer w.unlock()
	old := w.snap
	w.snap = w.scan(old)
	w.publish(diff(old, w.snap))
}

// watchDirs watches every folder whose entries make up the snapshot. fsnotify
// watches a folder's direct entries, and adding one twice is harmless.
func (w *Workspace) watchDirs(watcher *fsnotify.Watcher) {
	dirs := []string{w.root, w.Path(), w.Path(teamsDir)}
	entries, _ := os.ReadDir(w.root)
	for _, e := range entries {
		if e.IsDir() && e.Name() != DirName {
			project := filepath.Join(w.root, e.Name())
			dirs = append(dirs, project, filepath.Join(project, projectRepo))
		}
	}
	for _, d := range dirs {
		if _, err := os.Stat(d); err == nil {
			_ = watcher.Add(d)
		}
	}
}

// relevant says whether a file event can change the snapshot. .foreman/ also
// holds state.db and the logs, which change all the time.
func (w *Workspace) relevant(path string) bool {
	rel, err := filepath.Rel(w.root, path)
	if err != nil {
		return false
	}
	parts := strings.Split(rel, string(filepath.Separator))
	if parts[0] == DirName {
		return rel == filepath.Join(DirName, settingsFile) || (len(parts) > 1 && parts[1] == teamsDir)
	}
	switch len(parts) {
	case 1, 2: // a project folder, or one of its entries (a repo, project/)
		return true
	case 3:
		return parts[1] == projectRepo && parts[2] == "project.toml"
	}
	return false
}
