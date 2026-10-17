package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"strings"
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
	// SaveTeam writes a team file if still at version; returns the saved team.
	SaveTeam(ctx context.Context, c workspace.TeamConfig, version string) (workspace.Team, error)
	// Candidates are the folders Add a project can turn into projects.
	Candidates() []workspace.Candidate
	// AddProject makes a folder a project, with an optional first (empty) repo.
	AddProject(ctx context.Context, folder, repo string) (workspace.Project, error)
	// NewProject creates the folder, then makes it a project as AddProject does.
	NewProject(ctx context.Context, name, repo string) (workspace.Project, error)
	// AddRepo creates an empty code repo in a project.
	AddRepo(ctx context.Context, project, name string) (workspace.Project, error)
	// Missing lists the scaffold files a project repo lacks (wiki.toml…), and
	// Scaffold creates them, never overwriting one.
	Missing(project string) ([]workspace.Missing, error)
	Scaffold(ctx context.Context, project string) error
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
	mux.HandleFunc("GET /api/candidates", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, c.Candidates())
	})
	mux.HandleFunc("POST /api/projects", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Folder string `json:"folder"`
			Repo   string `json:"repo"`
			New    bool   `json:"new"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<12)).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "BAD_REQUEST", "expected {folder, repo?, new?}")
			return
		}
		add := c.AddProject
		if req.New {
			add = c.NewProject
		}
		p, err := add(r.Context(), req.Folder, req.Repo)
		writeCreated(w, p, err)
	})
	mux.HandleFunc("POST /api/projects/{id}/repos", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<12)).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "BAD_REQUEST", "expected {name}")
			return
		}
		p, err := c.AddRepo(r.Context(), r.PathValue("id"), req.Name)
		writeCreated(w, p, err)
	})
	mux.HandleFunc("GET /api/projects/{id}/scaffold", func(w http.ResponseWriter, r *http.Request) {
		m, err := c.Missing(r.PathValue("id"))
		writeFound(w, m, err, "project")
	})
	mux.HandleFunc("POST /api/projects/{id}/scaffold", func(w http.ResponseWriter, r *http.Request) {
		if err := c.Scaffold(r.Context(), r.PathValue("id")); err != nil {
			writeFound(w, nil, err, "project")
			return
		}
		m, err := c.Missing(r.PathValue("id"))
		writeFound(w, m, err, "project")
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
		if _, err := c.SaveSettings(r.Context(), req.Settings, req.Version); err != nil {
			writeSaveError(w, err, "settings.toml")
			return
		}
		writeJSON(w, http.StatusOK, c.Settings())
	})
	mux.HandleFunc("PUT /api/teams/{id}/agents/{agent}", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Soul    workspace.Soul `json:"soul"`
			Version string         `json:"version"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "BAD_REQUEST", "expected {soul, version}")
			return
		}
		t, err := c.Team(r.PathValue("id"))
		agent := r.PathValue("agent")
		if _, ok := t.Config.Agents[agent]; err != nil || !ok {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "no such agent")
			return
		}
		// The team is the workspace's own: change a copy, so a refused save changes nothing.
		t.Config.Agents = maps.Clone(t.Config.Agents)
		t.Config.Agents[agent] = req.Soul
		saved, err := c.SaveTeam(r.Context(), t.Config, req.Version)
		if err != nil {
			writeSaveError(w, err, "teams/"+t.ID+".toml")
			return
		}
		writeJSON(w, http.StatusOK, saved)
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

// writeCreated answers a request that created a project or a repo.
func writeCreated(w http.ResponseWriter, p workspace.Project, err error) {
	switch {
	case errors.Is(err, workspace.ErrInvalid):
		writeError(w, http.StatusBadRequest, "INVALID", strings.TrimPrefix(err.Error(), workspace.ErrInvalid.Error()+": "))
	case errors.Is(err, workspace.ErrExists):
		writeError(w, http.StatusConflict, "EXISTS", strings.TrimPrefix(err.Error(), workspace.ErrExists.Error()+": ")+" already exists")
	default:
		writeFound(w, p, err, "project")
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

// writeSaveError answers a refused save of file: a stale version or invalid content.
func writeSaveError(w http.ResponseWriter, err error, file string) {
	var invalid *workspace.InvalidError
	switch {
	case errors.Is(err, workspace.ErrConflict):
		writeError(w, http.StatusConflict, "CONFLICT", file+" changed since you read it")
	case errors.As(err, &invalid):
		writeError(w, http.StatusBadRequest, "INVALID", invalid.Error())
	default:
		writeError(w, http.StatusInternalServerError, "INTERNAL", err.Error())
	}
}
