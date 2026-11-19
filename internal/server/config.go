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
	"github.com/brickpop/foreman/internal/home"
	"github.com/brickpop/foreman/internal/prompt"
)

// Config is the home's config files and projects, as loaded (implemented by
// home.Home).
type Config interface {
	Projects() []home.Project
	Project(id string) (home.Project, error) // home.ErrNotFound
	Teams() []home.Team
	Team(id string) (home.Team, error) // home.ErrNotFound
	Settings() home.SettingsFile
	// SaveSettings writes settings.toml if still at version; returns the new version.
	SaveSettings(ctx context.Context, s home.Settings, version string) (string, error)
	// Instructions are the CEO's instructions for every agent;
	// SaveInstructions writes them if still at version, one commit in
	// Foreman's HQ, and returns them as saved.
	Instructions() home.Instructions
	SaveInstructions(ctx context.Context, text, version string) (home.Instructions, error)
	// SaveTeam writes a team file if still at version; returns the saved team.
	SaveTeam(ctx context.Context, c home.TeamConfig, version string) (home.Team, error)
	// NewProject creates a project with its HQ, new or cloned from clone (a
	// URL or a local path), and no code repo yet: id is its slug, name its
	// display name ("": shown by its id); created is false when it
	// took over an HQ left there. RemoveProject removes one and its history,
	// keeping its folder: its HQ only if deleteHQ, its agents' worktrees only
	// if deleteWorktrees.
	NewProject(ctx context.Context, id, name, clone string) (p home.Project, created bool, err error)
	RemoveProject(ctx context.Context, id string, deleteHQ, deleteWorktrees bool) error
	// BringHere clones the HQ of a project not on this machine from the
	// origin its record keeps.
	BringHere(ctx context.Context, id string) (home.Project, error)
	// AddRepo adds an existing repo, by its folder, with def as its default
	// branch ("": main, which it must have); NewRepo creates an empty one at
	// a new folder, on main, and adds it.
	AddRepo(ctx context.Context, project, name, path, def string) (home.Project, error)
	NewRepo(ctx context.Context, project, name, path string) (home.Project, error)
	// CloneRepo adds a repo cloned from remote into Foreman's repos/;
	// CloneMissingRepo locates one by cloning the remote its record keeps.
	CloneRepo(ctx context.Context, project, name, remote, def string) (home.Project, error)
	CloneMissingRepo(ctx context.Context, project, name string) (home.Project, error)
	// LocateRepo points a missing repo at its new folder; RemoveRepo takes a
	// repo out of the project (refused while a team is deployed on it).
	LocateRepo(ctx context.Context, project, name, path string) (home.Project, error)
	RemoveRepo(ctx context.Context, project, name string) (home.Project, error)
	// RenameRepo renames a repo, its agents' worktrees with it (refused while
	// a team is deployed on the project).
	RenameRepo(ctx context.Context, project, name, newName string) (home.Project, error)
	// SetRepoDefault sets a repo's default branch, one it has.
	SetRepoDefault(ctx context.Context, project, name, branch string) (home.Project, error)
	// Browse lists a folder for picking repos, inside the
	// CEO's home folder ("" is the folder Foreman was started from).
	Browse(path, project string) (home.Folder, error)
	// RepoBranches lists a folder's branches, to pick a repo's default branch.
	RepoBranches(ctx context.Context, path string) (home.Branches, error)
	// SetProfile names the profile the project's agents run on.
	SetProfile(ctx context.Context, project, profile string) (home.Project, error)
	// SetName gives the project its display name ("": shown by its id).
	SetName(ctx context.Context, project, name string) (home.Project, error)
	// SetHidden hides the project, or shows it again.
	SetHidden(ctx context.Context, project string, hidden bool) (home.Project, error)
	// Missing lists the scaffold files a project's HQ lacks (wiki.toml…), and
	// Scaffold creates them, never overwriting one.
	Missing(project string) ([]home.Missing, error)
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
	mux.HandleFunc("POST /api/projects", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID    string `json:"id"`
			Name  string `json:"name"`
			Clone string `json:"clone"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<12)).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "BAD_REQUEST", "expected {id, name?, clone?}")
			return
		}
		p, created, err := c.NewProject(r.Context(), req.ID, req.Name, req.Clone)
		if err == nil && created {
			// 201 tells a new HQ from one taken over (200).
			writeJSON(w, http.StatusCreated, p)
			return
		}
		writeProject(w, p, err)
	})
	mux.HandleFunc("DELETE /api/projects/{id}", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if err := c.RemoveProject(r.Context(), r.PathValue("id"), q.Get("deleteHQ") == "true", q.Get("deleteWorktrees") == "true"); err != nil {
			writeProject(w, nil, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /api/projects/{id}/bring", func(w http.ResponseWriter, r *http.Request) {
		p, err := c.BringHere(r.Context(), r.PathValue("id"))
		writeProject(w, p, err)
	})
	mux.HandleFunc("PUT /api/projects/{id}/repos/{repo}/name", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<12)).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "BAD_REQUEST", "expected {name}")
			return
		}
		p, err := c.RenameRepo(r.Context(), r.PathValue("id"), r.PathValue("repo"), req.Name)
		writeProject(w, p, err)
	})
	mux.HandleFunc("GET /api/folders", func(w http.ResponseWriter, r *http.Request) {
		f, err := c.Browse(r.URL.Query().Get("path"), r.URL.Query().Get("project"))
		writeProject(w, f, err)
	})
	mux.HandleFunc("POST /api/projects/{id}/repos", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Name    string `json:"name"`
			Path    string `json:"path"`
			Default string `json:"defaultBranch"`
			New     bool   `json:"new"`
			Clone   string `json:"clone"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<12)).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "BAD_REQUEST", "expected {name, path, defaultBranch?, new?} or {name, clone, defaultBranch?}")
			return
		}
		var p home.Project
		var err error
		switch {
		case req.Clone != "":
			p, err = c.CloneRepo(r.Context(), r.PathValue("id"), req.Name, req.Clone, req.Default)
		case req.New:
			p, err = c.NewRepo(r.Context(), r.PathValue("id"), req.Name, req.Path)
		default:
			p, err = c.AddRepo(r.Context(), r.PathValue("id"), req.Name, req.Path, req.Default)
		}
		writeProject(w, p, err)
	})
	mux.HandleFunc("GET /api/folders/branches", func(w http.ResponseWriter, r *http.Request) {
		b, err := c.RepoBranches(r.Context(), r.URL.Query().Get("path"))
		writeProject(w, b, err)
	})
	mux.HandleFunc("PUT /api/projects/{id}/repos/{repo}/default", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Branch string `json:"branch"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<12)).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "BAD_REQUEST", "expected {branch}")
			return
		}
		p, err := c.SetRepoDefault(r.Context(), r.PathValue("id"), r.PathValue("repo"), req.Branch)
		writeProject(w, p, err)
	})
	mux.HandleFunc("PUT /api/projects/{id}/repos/{repo}", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Path  string `json:"path"`
			Clone bool   `json:"clone"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<12)).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "BAD_REQUEST", "expected {path} or {clone: true}")
			return
		}
		var p home.Project
		var err error
		if req.Clone {
			p, err = c.CloneMissingRepo(r.Context(), r.PathValue("id"), r.PathValue("repo"))
		} else {
			p, err = c.LocateRepo(r.Context(), r.PathValue("id"), r.PathValue("repo"), req.Path)
		}
		writeProject(w, p, err)
	})
	mux.HandleFunc("DELETE /api/projects/{id}/repos/{repo}", func(w http.ResponseWriter, r *http.Request) {
		p, err := c.RemoveRepo(r.Context(), r.PathValue("id"), r.PathValue("repo"))
		writeProject(w, p, err)
	})
	mux.HandleFunc("PUT /api/projects/{id}/profile", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Profile string `json:"profile"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<12)).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "BAD_REQUEST", "expected {profile}")
			return
		}
		p, err := c.SetProfile(r.Context(), r.PathValue("id"), req.Profile)
		writeProject(w, p, err)
	})
	mux.HandleFunc("PUT /api/projects/{id}/name", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Name *string `json:"name"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<12)).Decode(&req); err != nil || req.Name == nil {
			writeError(w, http.StatusBadRequest, "BAD_REQUEST", "expected {name}")
			return
		}
		p, err := c.SetName(r.Context(), r.PathValue("id"), *req.Name)
		writeProject(w, p, err)
	})
	mux.HandleFunc("PUT /api/projects/{id}/hidden", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Hidden *bool `json:"hidden"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<12)).Decode(&req); err != nil || req.Hidden == nil {
			writeError(w, http.StatusBadRequest, "BAD_REQUEST", "expected {hidden}")
			return
		}
		p, err := c.SetHidden(r.Context(), r.PathValue("id"), *req.Hidden)
		writeProject(w, p, err)
	})
	mux.HandleFunc("GET /api/projects/{id}/scaffold", func(w http.ResponseWriter, r *http.Request) {
		m, err := c.Missing(r.PathValue("id"))
		writeFound(w, m, err, "project")
	})
	mux.HandleFunc("POST /api/projects/{id}/scaffold", func(w http.ResponseWriter, r *http.Request) {
		if err := c.Scaffold(r.Context(), r.PathValue("id")); err != nil {
			writeProject(w, nil, err)
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
		writeJSON(w, http.StatusOK, map[string]string{"prompt": prompt.Template(t.Config, r.PathValue("agent"), c.Instructions().Text)})
	})
	mux.HandleFunc("PUT /api/settings", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Settings home.Settings `json:"settings"`
			Version  string        `json:"version"`
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
			Soul    home.Soul `json:"soul"`
			Version string    `json:"version"`
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
		// The team is the home's own: change a copy, so a refused save changes nothing.
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
	mux.HandleFunc("GET /api/instructions", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, c.Instructions())
	})
	mux.HandleFunc("PUT /api/instructions", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Text    *string `json:"text"`
			Version string  `json:"version"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&req); err != nil || req.Text == nil {
			writeError(w, http.StatusBadRequest, "BAD_REQUEST", "expected {text, version}")
			return
		}
		saved, err := c.SaveInstructions(r.Context(), *req.Text, req.Version)
		if err != nil {
			writeSaveError(w, err, "instructions.md")
			return
		}
		writeJSON(w, http.StatusOK, saved)
	})
}

func writeFound(w http.ResponseWriter, v any, err error, what string) {
	switch {
	case errors.Is(err, home.ErrNotFound):
		writeError(w, http.StatusNotFound, "NOT_FOUND", "No such "+what)
	case err != nil:
		writeError(w, http.StatusInternalServerError, "INTERNAL", err.Error())
	default:
		writeJSON(w, http.StatusOK, v)
	}
}

// writeProject answers a request on a project or what it is made of (its
// repos, the folders they are picked from), with v; a refusal says why.
func writeProject(w http.ResponseWriter, v any, err error) {
	switch {
	case errors.Is(err, home.ErrInvalid):
		writeError(w, http.StatusBadRequest, "INVALID", strings.TrimPrefix(err.Error(), home.ErrInvalid.Error()+": "))
	case errors.Is(err, home.ErrExists):
		writeError(w, http.StatusConflict, "EXISTS", strings.TrimPrefix(err.Error(), home.ErrExists.Error()+": "))
	case errors.Is(err, home.ErrDeployed):
		writeError(w, http.StatusConflict, "DEPLOYED", strings.TrimPrefix(err.Error(), home.ErrDeployed.Error()+": "))
	default:
		writeFound(w, v, err, "project")
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
	var invalid *home.InvalidError
	switch {
	case errors.Is(err, home.ErrConflict):
		writeError(w, http.StatusConflict, "CONFLICT", file+" changed since you read it")
	case errors.As(err, &invalid):
		writeError(w, http.StatusBadRequest, "INVALID", invalid.Error())
	default:
		writeError(w, http.StatusInternalServerError, "INTERNAL", err.Error())
	}
}
