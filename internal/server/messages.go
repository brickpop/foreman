package server

import (
	"context"
	"net/http"

	"github.com/brickpop/foreman/internal/store"
)

// Messages is the team-message traffic (implemented by Messaging).
type Messages interface {
	// Traffic is a deployment's messages, oldest first.
	Traffic(ctx context.Context, deployment string) ([]store.TeamMessage, error)
}

func messageRoutes(mux *http.ServeMux, m Messages) {
	mux.HandleFunc("GET /api/messages", func(w http.ResponseWriter, r *http.Request) {
		deployment := r.URL.Query().Get("deployment")
		if deployment == "" {
			writeError(w, http.StatusBadRequest, "BAD_REQUEST", "expected ?deployment=<team@project>")
			return
		}
		list, err := m.Traffic(r.Context(), deployment)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, list)
	})
}
