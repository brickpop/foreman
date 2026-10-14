package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/brickpop/foreman/internal/instances"
	"github.com/brickpop/foreman/internal/store"
)

// Agents deploys teams and lists their instances (implemented by the
// instance manager).
type Agents interface {
	Deploy(ctx context.Context, team, project, goal string) (store.Deployment, error)
	Stop(ctx context.Context, deployment string) error
	List(ctx context.Context) ([]store.Instance, error)
}

// deployRequest is the body of POST /api/deployments.
type deployRequest struct {
	Team    string `json:"team"`
	Project string `json:"project"`
	Goal    string `json:"goal"`
}

func agentRoutes(mux *http.ServeMux, a Agents) {
	mux.HandleFunc("GET /api/instances", func(w http.ResponseWriter, r *http.Request) {
		list, err := a.List(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, list)
	})
	mux.HandleFunc("POST /api/deployments", func(w http.ResponseWriter, r *http.Request) {
		var req deployRequest
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "BAD_REQUEST", "expected {team, project, goal}")
			return
		}
		if req.Team == "" || req.Project == "" || strings.TrimSpace(req.Goal) == "" {
			writeError(w, http.StatusBadRequest, "BAD_REQUEST", "team, project and goal are required")
			return
		}
		dep, err := a.Deploy(r.Context(), req.Team, req.Project, req.Goal)
		switch {
		case errors.Is(err, store.ErrActive):
			writeError(w, http.StatusConflict, "ALREADY_DEPLOYED", "This team is already deployed on this project")
		case errors.Is(err, instances.ErrInvalid):
			writeError(w, http.StatusBadRequest, "CANT_DEPLOY", err.Error())
		case err != nil:
			writeError(w, http.StatusInternalServerError, "INTERNAL", err.Error())
		default:
			writeJSON(w, http.StatusCreated, dep)
		}
	})
	mux.HandleFunc("DELETE /api/deployments/{id}", func(w http.ResponseWriter, r *http.Request) {
		err := a.Stop(r.Context(), r.PathValue("id"))
		switch {
		case errors.Is(err, store.ErrNotFound):
			writeError(w, http.StatusNotFound, "NOT_FOUND", "No such deployment")
		case err != nil:
			writeError(w, http.StatusInternalServerError, "INTERNAL", err.Error())
		default:
			w.WriteHeader(http.StatusNoContent)
		}
	})
}
