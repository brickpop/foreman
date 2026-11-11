package server

import (
	"context"
	"errors"
	"net/http"

	"github.com/brickpop/foreman/internal/workflow"
)

// Pipeline is a project's work on its way to its targets (implemented by Workflow);
// the CEO decides in Reviews.
type Pipeline interface {
	Pipeline(ctx context.Context, project string) (workflow.Pipeline, error)
	// CleanUp removes what agents no longer running left on a project, only
	// what is safe, and says what it kept (home.ErrNotFound: no such project).
	CleanUp(ctx context.Context, project string) (workflow.Cleanup, error)
	// CleanupBranches lists the branches of a code repo the CEO may delete
	// (the Repos page's branch cleanup); DeleteBranches deletes the pick
	// (home.ErrNotFound: no such project; workflow.ErrNoRepo: no such repo).
	CleanupBranches(ctx context.Context, project, repo string) ([]workflow.CleanupBranch, error)
	DeleteBranches(ctx context.Context, project, repo string, branches []string) (workflow.Cleanup, error)
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
		done, err := p.CleanUp(r.Context(), r.PathValue("id"))
		writeFound(w, done, err, "project")
	})
	mux.HandleFunc("GET /api/projects/{id}/repos/{repo}/branch-cleanup", func(w http.ResponseWriter, r *http.Request) {
		list, err := p.CleanupBranches(r.Context(), r.PathValue("id"), r.PathValue("repo"))
		writeBranches(w, list, err)
	})
	mux.HandleFunc("POST /api/projects/{id}/repos/{repo}/branch-cleanup", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Branches []string `json:"branches"`
		}
		if !decode(w, r, &req, "expected {branches}") {
			return
		}
		done, err := p.DeleteBranches(r.Context(), r.PathValue("id"), r.PathValue("repo"), req.Branches)
		writeBranches(w, done, err)
	})
}

// writeBranches answers a branch cleanup: 404 for a project or repo that
// isn't there.
func writeBranches(w http.ResponseWriter, v any, err error) {
	if errors.Is(err, workflow.ErrNoRepo) {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "No such repo in this project")
		return
	}
	writeFound(w, v, err, "project")
}
