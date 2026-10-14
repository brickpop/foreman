// Package server is Foreman's HTTP surface: the API, the event and PTY
// streams, and the web UI. It depends only on the interfaces below, so every
// piece behind them can be swapped in cmd/foreman, the composition root.
package server

import (
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
)

// Guard decides which requests reach the server (implemented by access.TokenGuard).
type Guard interface {
	Wrap(next http.Handler) http.Handler
}

// Workspace is what the server needs to know about the workspace it serves.
type Workspace interface {
	Root() string
}

// Options are the parts the server is assembled from.
type Options struct {
	Guard     Guard
	Workspace Workspace
	Config    Config
	Events    Events
	Terminals Terminals
	UI        fs.FS // the built web app, index.html at its root
}

// New returns the HTTP handler, every route behind the guard.
func New(o Options) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/workspace", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, workspaceSummary{Path: o.Workspace.Root()})
	})
	mux.HandleFunc("GET /api/instances", func(w http.ResponseWriter, r *http.Request) {
		list := []instanceSummary{}
		for _, id := range o.Terminals.List() {
			list = append(list, instanceSummary{ID: id})
		}
		writeJSON(w, http.StatusOK, list)
	})
	configRoutes(mux, o.Config)
	mux.HandleFunc("GET /api/events", handleEvents(o.Events))
	mux.HandleFunc("GET /ws/pty/{instance}", handlePTY(o.Terminals))
	mux.Handle("/", ui(o.UI))
	return o.Guard.Wrap(mux)
}

// workspaceSummary is the body of GET /api/workspace.
type workspaceSummary struct {
	Path string `json:"path"`
}

// instanceSummary is one entry of GET /api/instances. It grows into the
// instance model (status, task, last line) with deployments.
type instanceSummary struct {
	ID string `json:"id"`
}

// ui serves the built SPA: real files as they are, every other path as
// index.html so client-side routes survive a reload.
func ui(files fs.FS) http.Handler {
	fileServer := http.FileServerFS(files)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := fs.Stat(files, "index.html"); errors.Is(err, fs.ErrNotExist) {
			http.Error(w, "The web UI isn't built. Run `just build`.", http.StatusServiceUnavailable)
			return
		}
		path := r.URL.Path[1:]
		if path != "" {
			if _, err := fs.Stat(files, path); err == nil {
				fileServer.ServeHTTP(w, r)
				return
			}
		}
		http.ServeFileFS(w, r, files, "index.html")
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
