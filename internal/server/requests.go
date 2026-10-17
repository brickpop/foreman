package server

import (
	"context"
	"net/http"

	"github.com/brickpop/foreman/internal/store"
)

// Requests is what waits for the CEO (implemented by Workflow).
type Requests interface {
	OpenRequests(ctx context.Context) ([]store.Request, error)
}

func requestRoutes(mux *http.ServeMux, q Requests) {
	mux.HandleFunc("GET /api/requests", func(w http.ResponseWriter, r *http.Request) {
		list, err := q.OpenRequests(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, list)
	})
}
