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

// Home is what the server needs to know about Foreman's home.
type Home interface {
	Root() string
	// Folder is Foreman's folder: its HQ, the projects, the worktrees.
	Folder() string
}

// Options are the parts the server is assembled from.
type Options struct {
	Guard        Guard
	Home         Home
	Config       Config
	Events       Events
	Agents       Agents
	Messages     Messages
	Activity     Activity
	Repos        Repos
	Pipeline     Pipeline
	Reviews      Reviews
	Requests     Requests
	Harnesses    Harnesses
	Profiles     Profiles
	Wiki         Wiki
	Setup        Setup
	Maintenance  Maintenance
	Terminals    Terminals
	ChiefOfStaff ChiefOfStaff
	Desktop      Desktop
	UI           fs.FS // the built web app, index.html at its root
}

// New returns the HTTP handler, every route behind the guard.
func New(o Options) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/home", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, homeSummary{Path: o.Home.Root(), Folder: o.Home.Folder()})
	})
	agentRoutes(mux, o.Agents)
	messageRoutes(mux, o.Messages)
	activityRoutes(mux, o.Activity)
	repoRoutes(mux, o.Repos)
	pipelineRoutes(mux, o.Pipeline)
	reviewRoutes(mux, o.Reviews)
	requestRoutes(mux, o.Requests)
	harnessRoutes(mux, o.Harnesses)
	profileRoutes(mux, o.Profiles)
	maintenanceRoutes(mux, o.Maintenance)
	setupRoutes(mux, o.Setup)
	configRoutes(mux, o.Config)
	wikiRoutes(mux, o.Wiki)
	cosRoutes(mux, o.ChiefOfStaff)
	desktopRoutes(mux, o.Desktop, o.Agents, o.Config)
	mux.HandleFunc("GET /api/events", handleEvents(o.Events))
	mux.HandleFunc("GET /ws/pty/{instance}", handlePTY(o.Terminals))
	// An endpoint the server doesn't have is an API error, never the web app.
	noEndpoint := func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "No such endpoint: "+r.Method+" "+r.URL.Path)
	}
	mux.HandleFunc("/api/", noEndpoint)
	mux.HandleFunc("/ws/", noEndpoint)
	mux.Handle("/", ui(o.UI))
	return o.Guard.Wrap(mux)
}

// homeSummary is the body of GET /api/home.
type homeSummary struct {
	Path   string `json:"path"`   // Foreman's home: its machinery
	Folder string `json:"folder"` // Foreman's folder: what the CEO owns
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
