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
	CleanUp(ctx context.Context, project string) (workflow.Cleanup, error)
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
}
