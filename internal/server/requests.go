package server

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/brickpop/foreman/internal/store"
	"github.com/brickpop/foreman/internal/workflow"
)

// Requests is what waits for the CEO (implemented by Workflow).
type Requests interface {
	OpenRequests(ctx context.Context) ([]store.Request, error)
	// Answer is the CEO's answer to an agent's request (an escalation).
	Answer(ctx context.Context, id, answer string) (store.Request, error)
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
	mux.HandleFunc("POST /api/requests/{id}/resolve", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Answer string `json:"answer"`
		}
		if !decode(w, r, &req, "expected {answer}") {
			return
		}
		resolved, err := q.Answer(r.Context(), r.PathValue("id"), req.Answer)
		switch {
		case err == nil:
			writeJSON(w, http.StatusOK, resolved)
		case errors.Is(err, workflow.ErrAnswered):
			writeError(w, http.StatusConflict, "NOT_OPEN", "It isn't open any more: answered already, or its team was stopped")
		case errors.Is(err, workflow.ErrInvalid):
			writeError(w, http.StatusBadRequest, "INVALID", strings.TrimPrefix(err.Error(), workflow.ErrInvalid.Error()+": "))
		default:
			writeError(w, http.StatusInternalServerError, "INTERNAL", err.Error())
		}
	})
}
