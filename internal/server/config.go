package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/brickpop/foreman/internal/events"
	"github.com/brickpop/foreman/internal/prompt"
	"github.com/brickpop/foreman/internal/workspace"
)

// Config is the workspace's config files, as loaded (implemented by
// workspace.Workspace).
type Config interface {
	Projects() []workspace.Project
	Project(id string) (workspace.Project, error) // workspace.ErrNotFound
	Teams() []workspace.Team
	Team(id string) (workspace.Team, error) // workspace.ErrNotFound
	Settings() workspace.SettingsFile
	// SaveSettings writes settings.toml if still at version; returns the new version.
	SaveSettings(ctx context.Context, s workspace.Settings, version string) (string, error)
}

// Events is what the server streams to the browser (implemented by events.Bus).
type Events interface {
	Subscribe() (<-chan events.Event, func())
}

func configRoutes(mux *http.ServeMux, c Config) {
	mux.HandleFunc("GET /api/projects", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, c.Projects())
	})
	mux.HandleFunc("GET /api/projects/{id}", func(w http.ResponseWriter, r *http.Request) {
		p, err := c.Project(r.PathValue("id"))
		writeFound(w, p, err, "project")
	})
	mux.HandleFunc("GET /api/teams", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, c.Teams())
	})
	mux.HandleFunc("GET /api/teams/{id}", func(w http.ResponseWriter, r *http.Request) {
		t, err := c.Team(r.PathValue("id"))
		writeFound(w, t, err, "team")
	})
	mux.HandleFunc("GET /api/teams/{id}/agents/{agent}/prompt", func(w http.ResponseWriter, r *http.Request) {
		t, err := c.Team(r.PathValue("id"))
		if _, ok := t.Config.Agents[r.PathValue("agent")]; err != nil || !ok {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "No such agent")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"prompt": prompt.Template(t.Config, r.PathValue("agent"))})
	})
	mux.HandleFunc("PUT /api/settings", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Settings workspace.Settings `json:"settings"`
			Version  string             `json:"version"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "BAD_REQUEST", "expected {settings, version}")
			return
		}
		_, err := c.SaveSettings(r.Context(), req.Settings, req.Version)
		var invalid *workspace.InvalidError
		switch {
		case errors.Is(err, workspace.ErrConflict):
			writeError(w, http.StatusConflict, "CONFLICT", "settings.toml changed since you read it")
		case errors.As(err, &invalid):
			writeError(w, http.StatusBadRequest, "INVALID", invalid.Error())
		case err != nil:
			writeError(w, http.StatusInternalServerError, "INTERNAL", err.Error())
		default:
			writeJSON(w, http.StatusOK, c.Settings())
		}
	})
	mux.HandleFunc("GET /api/settings", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, c.Settings())
	})
}

func writeFound(w http.ResponseWriter, v any, err error, what string) {
	switch {
	case errors.Is(err, workspace.ErrNotFound):
		writeError(w, http.StatusNotFound, "NOT_FOUND", "No such "+what)
	case err != nil:
		writeError(w, http.StatusInternalServerError, "INTERNAL", err.Error())
	default:
		writeJSON(w, http.StatusOK, v)
	}
}

// writeError writes the API's error format
// (wiki/product/architecture/api.md#error-format).
func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]string{"error": message, "code": code})
}

// keepAlive is how often an idle event stream sends a comment, so proxies
// and the browser don't take a quiet stream for a dead one.
const keepAlive = 25 * time.Second

// handleEvents streams events as server-sent events, each one `data:` line
// holding the whole event as JSON ({"type": …, "data": …}), so the browser needs
// one handler, not a list of every type. The stream ends when the client
// leaves or falls behind; the browser then reconnects and refetches what it
// shows.
func handleEvents(bus Events) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			writeError(w, http.StatusInternalServerError, "INTERNAL", "streaming unsupported")
			return
		}
		ch, cancel := bus.Subscribe()
		defer cancel()
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = fmt.Fprint(w, ": connected\n\n")
		flusher.Flush()
		ticker := time.NewTicker(keepAlive)
		defer ticker.Stop()
		for {
			select {
			case <-r.Context().Done():
				return
			case <-ticker.C:
				_, _ = fmt.Fprint(w, ": keep-alive\n\n")
			case e, ok := <-ch:
				if !ok {
					return
				}
				data, err := json.Marshal(e)
				if err != nil {
					continue
				}
				_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
			}
			flusher.Flush()
		}
	}
}
