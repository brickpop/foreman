package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/brickpop/foreman/internal/profiles"
)

// Profiles is Settings → Global → Profiles: each a harness and its
// credentials, shared by every workspace (wiki/product/architecture/session-runner.md#credentials).
type Profiles interface {
	// List returns the profiles with their state, the default first.
	List() ([]profiles.Profile, error)
	// Create makes a profile for a supported harness, with no credentials yet.
	Create(name, harness string) (profiles.Profile, error)
	// Delete removes a profile and its credentials; never the default.
	Delete(name string) error
	// Connect starts the profile's harness login in a terminal and returns
	// the terminal's id, for /ws/pty/{id}; one already running is reused.
	Connect(ctx context.Context, name string) (string, error)
	// EndLogin ends the profile's login, if one runs.
	EndLogin(name string) error
}

// ErrUnknownHarness means a profile was asked for a harness Foreman doesn't support.
var ErrUnknownHarness = errors.New("no such harness")

func profileRoutes(mux *http.ServeMux, p Profiles) {
	mux.HandleFunc("GET /api/profiles", func(w http.ResponseWriter, r *http.Request) {
		list, err := p.List()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, list)
	})
	mux.HandleFunc("POST /api/profiles", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Name    string `json:"name"`
			Harness string `json:"harness"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<10)).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "BAD_REQUEST", "expected {name, harness}")
			return
		}
		created, err := p.Create(req.Name, req.Harness)
		switch {
		case errors.Is(err, profiles.ErrExists):
			writeError(w, http.StatusConflict, "EXISTS", "A profile named "+req.Name+" exists")
		case errors.Is(err, profiles.ErrInvalid), errors.Is(err, ErrUnknownHarness):
			writeError(w, http.StatusBadRequest, "INVALID", err.Error())
		case err != nil:
			writeError(w, http.StatusInternalServerError, "INTERNAL", err.Error())
		default:
			writeJSON(w, http.StatusCreated, created)
		}
	})
	mux.HandleFunc("DELETE /api/profiles/{name}", func(w http.ResponseWriter, r *http.Request) {
		err := p.Delete(r.PathValue("name"))
		switch {
		case errors.Is(err, profiles.ErrNotFound):
			writeError(w, http.StatusNotFound, "NOT_FOUND", "No such profile")
		case errors.Is(err, profiles.ErrInvalid):
			writeError(w, http.StatusBadRequest, "INVALID", err.Error())
		case err != nil:
			writeError(w, http.StatusInternalServerError, "INTERNAL", err.Error())
		default:
			w.WriteHeader(http.StatusNoContent)
		}
	})
	mux.HandleFunc("POST /api/profiles/{name}/connect", func(w http.ResponseWriter, r *http.Request) {
		term, err := p.Connect(r.Context(), r.PathValue("name"))
		switch {
		case errors.Is(err, profiles.ErrNotFound):
			writeError(w, http.StatusNotFound, "NOT_FOUND", "No such profile")
		case err != nil:
			writeError(w, http.StatusInternalServerError, "INTERNAL", err.Error())
		default:
			writeJSON(w, http.StatusOK, map[string]string{"terminal": term})
		}
	})
	mux.HandleFunc("DELETE /api/profiles/{name}/connect", func(w http.ResponseWriter, r *http.Request) {
		err := p.EndLogin(r.PathValue("name"))
		switch {
		case errors.Is(err, profiles.ErrNotFound):
			writeError(w, http.StatusNotFound, "NOT_FOUND", "No such profile")
		case err != nil:
			writeError(w, http.StatusInternalServerError, "INTERNAL", err.Error())
		default:
			w.WriteHeader(http.StatusNoContent)
		}
	})
}
