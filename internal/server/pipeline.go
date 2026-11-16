package server

import (
	"context"
	"net/http"

	"github.com/brickpop/foreman/internal/workflow"
)

// Pipeline is a project's work on its way to its targets (implemented by Workflow);
// the CEO decides in Reviews.
type Pipeline interface {
	Pipeline(ctx context.Context, project string) (workflow.Pipeline, error)
	// CleanUp removes what agents no longer running left on a project, only
	// what is safe, and says what it kept (home.ErrNotFound: no such project).
	CleanUp(ctx context.Context, project string, uncommitted bool) (workflow.Cleanup, error)
	// CleanupBranches lists the branches of the project's code repos the CEO
	// may delete (the Repos section's branch cleanup); DeleteBranches deletes
	// the pick (home.ErrNotFound: no such project).
	CleanupBranches(ctx context.Context, project string) ([]workflow.CleanupBranch, error)
	DeleteBranches(ctx context.Context, project string, branches []workflow.BranchPick) (workflow.Cleanup, error)
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
	mux.HandleFunc("POST /api/projects/{id}/cleanup", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Uncommitted bool `json:"uncommitted"`
		}
		if !decode(w, r, &req, "expected {uncommitted}") {
			return
		}
		done, err := p.CleanUp(r.Context(), r.PathValue("id"), req.Uncommitted)
		writeFound(w, done, err, "project")
	})
	mux.HandleFunc("GET /api/projects/{id}/branch-cleanup", func(w http.ResponseWriter, r *http.Request) {
		list, err := p.CleanupBranches(r.Context(), r.PathValue("id"))
		writeFound(w, list, err, "project")
	})
	mux.HandleFunc("POST /api/projects/{id}/branch-cleanup", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Branches []workflow.BranchPick `json:"branches"`
		}
		if !decode(w, r, &req, "expected {branches: [{repo, branch}]}") {
			return
		}
		done, err := p.DeleteBranches(r.Context(), r.PathValue("id"), req.Branches)
		writeFound(w, done, err, "project")
	})
}
