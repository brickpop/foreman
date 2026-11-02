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
}
