package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/brickpop/foreman/internal/harness"
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
	// Endpoint is the model server and model of a profile whose harness runs
	// on one (Pi), set in a form instead of a login (ErrConnectKind otherwise).
	Endpoint(name string) (harness.Endpoint, error)
	// SetEndpoint checks and saves it (harness.ErrInvalid).
	SetEndpoint(name string, c harness.EndpointChange) error
	// Models lists the models the server at url offers, with key, or the
	// profile's key when key is "".
	Models(ctx context.Context, name, url, key string) ([]string, error)
}

// ErrUnknownHarness means a profile was asked for a harness Foreman doesn't support.
var ErrUnknownHarness = errors.New("no such harness")

// ErrConnectKind means a profile was asked to connect the other way: a
// login for a harness set in a form, or the reverse.
var ErrConnectKind = errors.New("the profile connects another way")

func profileRoutes(mux *http.ServeMux, p Profiles) {
	endpointRoutes(mux, p)
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
		case errors.Is(err, ErrConnectKind):
			writeError(w, http.StatusBadRequest, "INVALID", err.Error())
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

// endpointRoutes are a Pi profile's form: its model server and model.
func endpointRoutes(mux *http.ServeMux, p Profiles) {
	refused := func(w http.ResponseWriter, err error) bool {
		switch {
		case err == nil:
			return false
		case errors.Is(err, profiles.ErrNotFound):
			writeError(w, http.StatusNotFound, "NOT_FOUND", "No such profile")
		case errors.Is(err, ErrConnectKind), errors.Is(err, harness.ErrInvalid):
			writeError(w, http.StatusBadRequest, "INVALID", err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "INTERNAL", err.Error())
		}
		return true
	}
	mux.HandleFunc("GET /api/profiles/{name}/endpoint", func(w http.ResponseWriter, r *http.Request) {
		e, err := p.Endpoint(r.PathValue("name"))
		if !refused(w, err) {
			writeJSON(w, http.StatusOK, e)
		}
	})
	mux.HandleFunc("PUT /api/profiles/{name}/endpoint", func(w http.ResponseWriter, r *http.Request) {
		var c harness.EndpointChange
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&c); err != nil {
			writeError(w, http.StatusBadRequest, "BAD_REQUEST", "expected {url, apiKey, model, contextWindow, maxTokens}")
			return
		}
		if !refused(w, p.SetEndpoint(r.PathValue("name"), c)) {
			w.WriteHeader(http.StatusNoContent)
		}
	})
	// A POST, not a GET: the key travels in the body, never in a URL.
	mux.HandleFunc("POST /api/profiles/{name}/endpoint/models", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			URL    string `json:"url"`
			APIKey string `json:"apiKey"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "BAD_REQUEST", "expected {url, apiKey}")
			return
		}
		if refused(w, harness.CheckURL(req.URL)) {
			return
		}
		ids, err := p.Models(r.Context(), r.PathValue("name"), req.URL, req.APIKey)
		switch {
		case errors.Is(err, profiles.ErrNotFound), errors.Is(err, ErrConnectKind), errors.Is(err, harness.ErrInvalid):
			refused(w, err)
		case err != nil:
			// The server's own failure, said as it is: the CEO fixes the URL or starts it.
			writeError(w, http.StatusBadGateway, "UNREACHABLE", err.Error())
		default:
			writeJSON(w, http.StatusOK, ids)
		}
	})
}
