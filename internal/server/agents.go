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
	// DeploymentsIn lists the deployments with status (active or stopped).
	DeploymentsIn(ctx context.Context, status string) ([]store.Deployment, error)
	// Prompt is an instance's session prompt, exactly as composed for its harness.
	Prompt(ctx context.Context, instance string) (string, error)
	// Wake starts an asleep instance; any other is left as it is.
	Wake(ctx context.Context, instance string) error
	// Restart stops a running instance and starts it again, on its project's
	// profile as it is now; one that isn't up is left as it is.
	Restart(ctx context.Context, instance string) error
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
	mux.HandleFunc("GET /api/instances/{id}/prompt", func(w http.ResponseWriter, r *http.Request) {
		text, err := a.Prompt(r.Context(), r.PathValue("id"))
		switch {
		case errors.Is(err, store.ErrNotFound):
			writeError(w, http.StatusNotFound, "NOT_FOUND", "No such instance")
		case err != nil:
			writeError(w, http.StatusInternalServerError, "INTERNAL", err.Error())
		default:
			writeJSON(w, http.StatusOK, map[string]string{"prompt": text})
		}
	})
	for _, action := range []struct {
		path string
		do   func(context.Context, string) error
	}{
		// Closures, not method values: a method value of a nil Agents panics here.
		{"wake", func(ctx context.Context, id string) error { return a.Wake(ctx, id) }},
		{"restart", func(ctx context.Context, id string) error { return a.Restart(ctx, id) }},
	} {
		mux.HandleFunc("POST /api/instances/{id}/"+action.path, func(w http.ResponseWriter, r *http.Request) {
			err := action.do(r.Context(), r.PathValue("id"))
			switch {
			case errors.Is(err, store.ErrNotFound):
				writeError(w, http.StatusNotFound, "NOT_FOUND", "No such instance")
			case errors.Is(err, instances.ErrNotReady):
				writeError(w, http.StatusConflict, "NOT_READY", err.Error())
			case err != nil:
				writeError(w, http.StatusInternalServerError, "INTERNAL", err.Error())
			default:
				w.WriteHeader(http.StatusNoContent)
			}
		})
	}
	mux.HandleFunc("GET /api/deployments", func(w http.ResponseWriter, r *http.Request) {
		status := r.URL.Query().Get("status")
		switch status {
		case "":
			status = store.DeploymentActive
		case store.DeploymentActive, store.DeploymentStopped:
		default:
			writeError(w, http.StatusBadRequest, "BAD_REQUEST", "expected ?status=active or stopped")
			return
		}
		list, err := a.DeploymentsIn(r.Context(), status)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, list)
	})
	mux.HandleFunc("POST /api/deployments", func(w http.ResponseWriter, r *http.Request) {
		var req deployRequest
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "BAD_REQUEST", "expected {team, project, goal?}")
			return
		}
		if req.Team == "" || req.Project == "" {
			writeError(w, http.StatusBadRequest, "BAD_REQUEST", "team and project are required")
			return
		}
		// The goal is optional: the CEO may give it in the lead's session.
		dep, err := a.Deploy(r.Context(), req.Team, req.Project, strings.TrimSpace(req.Goal))
		switch {
		case errors.Is(err, store.ErrActive):
			writeError(w, http.StatusConflict, "ALREADY_DEPLOYED", "This team is already deployed on this project")
		case errors.Is(err, instances.ErrNotReady):
			writeError(w, http.StatusConflict, "NOT_READY", err.Error())
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
