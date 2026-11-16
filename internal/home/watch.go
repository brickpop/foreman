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
	h.watcher = watcher
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
				// Watch new folders (a new project's HQ) before reading them: a change made
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
// home (settings.toml), Foreman's HQ and its teams/, projects/ and each
// project's folder and HQ. A project's code repos are anywhere, so they are
// checked on every reload and start, never watched. fsnotify watches a
// folder's direct entries; adding one twice is harmless.
func (h *Home) watchDirs(watcher *fsnotify.Watcher) {
	projects := filepath.Join(h.folder, projectsDir)
	dirs := []string{h.root, h.HQ(), h.HQ(teamsDir), projects}
	entries, _ := os.ReadDir(projects)
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, filepath.Join(projects, e.Name()))
		}
	}
	dirs = append(dirs, h.projectHQs()...)
	for _, d := range dirs {
		if _, err := os.Stat(d); err == nil {
			_ = watcher.Add(d)
		}
	}
}

// projectHQs are the projects' HQs.
func (h *Home) projectHQs() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	var dirs []string
	for _, p := range h.snap.projects {
		dirs = append(dirs, p.Path)
	}
	return dirs
}

// relevant says whether a file event can change the snapshot. The home also
// holds state.db and the logs, which change all the time.
func (h *Home) relevant(path string) bool {
	for _, hq := range h.projectHQs() {
		// an HQ going, its project.toml
		if path == hq || path == filepath.Join(hq, "project.toml") {
			return true
		}
	}
	switch path {
	case h.Path(settingsFile), h.HQ(), h.HQ(instructionsFile), h.HQ(teamsDir):
		return true
	}
	if filepath.Dir(path) == h.HQ(teamsDir) {
		return true
	}
	// projects/, a project's folder or its HQ appearing or going
	rel, err := filepath.Rel(filepath.Join(h.folder, projectsDir), path)
	return err == nil && !strings.HasPrefix(rel, "..") && len(strings.Split(rel, string(filepath.Separator))) <= 2
}
