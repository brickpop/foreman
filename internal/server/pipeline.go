package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/brickpop/foreman/internal/store"
	"github.com/brickpop/foreman/internal/workflow"
)

// Pipeline is a project's work on its way to main, and the CEO's decisions
// on merge requests (implemented by Workflow).
type Pipeline interface {
	Pipeline(ctx context.Context, project string) (workflow.Pipeline, error)
	MergeRequest(ctx context.Context, id string) (store.MergeRequest, error)
	Diff(ctx context.Context, id string) (string, error)
	// MergeToMain: workflow.ErrConflict, workflow.ErrLocalChanges or
	// workflow.ErrNotOpen when it can't.
	MergeToMain(ctx context.Context, id string) error
	Reject(ctx context.Context, id, reason string) error
}

func pipelineRoutes(mux *http.ServeMux, p Pipeline) {
	mux.HandleFunc("GET /api/pipeline", func(w http.ResponseWriter, r *http.Request) {
		project := r.URL.Query().Get("project")
		if project == "" {
			writeError(w, http.StatusBadRequest, "BAD_REQUEST", "expected ?project=<id>")
			return
		}
		list, err := p.Pipeline(r.Context(), project)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, list)
	})
	mux.HandleFunc("GET /api/merge-requests/{id}", func(w http.ResponseWriter, r *http.Request) {
		mr, err := p.MergeRequest(r.Context(), r.PathValue("id"))
		writeMerge(w, mr, err)
	})
	mux.HandleFunc("GET /api/merge-requests/{id}/diff", func(w http.ResponseWriter, r *http.Request) {
		diff, err := p.Diff(r.Context(), r.PathValue("id"))
		writeMerge(w, map[string]string{"diff": diff}, err)
	})
	mux.HandleFunc("POST /api/merge-requests/{id}/merge", func(w http.ResponseWriter, r *http.Request) {
		writeMerge(w, nil, p.MergeToMain(r.Context(), r.PathValue("id")))
	})
	mux.HandleFunc("POST /api/merge-requests/{id}/reject", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Reason string `json:"reason"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<14)).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "BAD_REQUEST", "expected {reason}")
			return
		}
		writeMerge(w, nil, p.Reject(r.Context(), r.PathValue("id"), req.Reason))
	})
}

// writeMerge answers a merge request read or decision; a decision answers 204.
func writeMerge(w http.ResponseWriter, v any, err error) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "NOT_FOUND", "No such merge request")
	case errors.Is(err, workflow.ErrNotOpen):
		writeError(w, http.StatusConflict, "NOT_OPEN", "It was merged or rejected already")
	case errors.Is(err, workflow.ErrConflict):
		writeError(w, http.StatusConflict, "CONFLICT", "It conflicts with main: the lead was asked to merge main in, then to ask again")
	case errors.Is(err, workflow.ErrLocalChanges):
		writeError(w, http.StatusConflict, "LOCAL_CHANGES", "Your checkout of the repo has uncommitted changes the merge would overwrite: commit or stash them, then merge again")
	case err != nil:
		writeError(w, http.StatusInternalServerError, "INTERNAL", err.Error())
	case v == nil:
		w.WriteHeader(http.StatusNoContent)
	default:
		writeJSON(w, http.StatusOK, v)
	}
}
