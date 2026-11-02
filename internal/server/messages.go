package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/brickpop/foreman/internal/instances"
	"github.com/brickpop/foreman/internal/store"
)

// Messages is the team-message traffic (implemented by Messaging).
type Messages interface {
	// Traffic is a deployment's messages, oldest first.
	Traffic(ctx context.Context, deployment string) ([]store.TeamMessage, error)
	// Line types the CEO's line into an agent's session and sends it.
	Line(instance, text string) error
	// PauseCountdown pauses or resumes the countdown of messages waiting on
	// the CEO's draft in an agent's terminal.
	PauseCountdown(instance string, paused bool)
}

// Activity pages team traffic, messages and git actions together (implemented
// by the state store).
type Activity interface {
	Page(ctx context.Context, deployment, before string, limit int) (store.ActivityPage, error)
	Latest(ctx context.Context) (map[string]time.Time, error)
}

// activityPage is how many items a page of activity holds (the CEO, 2026-10-17).
const activityPage = 50

// activityRoutes serve the Activity view of a project's Team section: one deployment's
// traffic, the team on the project or an older session.
func activityRoutes(mux *http.ServeMux, act Activity) {
	// When something last happened on each project: the UI orders them by it.
	mux.HandleFunc("GET /api/activity/latest", func(w http.ResponseWriter, r *http.Request) {
		latest, err := act.Latest(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, latest)
	})
	mux.HandleFunc("GET /api/activity", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		deployment := q.Get("deployment")
		if deployment == "" {
			writeError(w, http.StatusBadRequest, "BAD_REQUEST", "expected ?deployment=<team@project>")
			return
		}
		page, err := act.Page(r.Context(), deployment, q.Get("before"), activityPage)
		switch {
		case errors.Is(err, store.ErrBadCursor):
			writeError(w, http.StatusBadRequest, "BAD_REQUEST", "a bad ?before= cursor")
		case err != nil:
			writeError(w, http.StatusInternalServerError, "INTERNAL", err.Error())
		default:
			writeJSON(w, http.StatusOK, page)
		}
	})
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
	mux.HandleFunc("POST /api/instances/{id}/line", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Text string `json:"text"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "BAD_REQUEST", "expected {text}")
			return
		}
		switch err := m.Line(r.PathValue("id"), req.Text); {
		case errors.Is(err, instances.ErrNotRunning):
			writeError(w, http.StatusConflict, "NOT_RUNNING", "the agent isn't running")
		case err != nil:
			writeError(w, http.StatusInternalServerError, "INTERNAL", err.Error())
		default:
			w.WriteHeader(http.StatusNoContent)
		}
	})
	mux.HandleFunc("PUT /api/instances/{id}/countdown", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Paused bool `json:"paused"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<10)).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "BAD_REQUEST", "expected {paused}")
			return
		}
		m.PauseCountdown(r.PathValue("id"), req.Paused)
		w.WriteHeader(http.StatusNoContent)
	})
}
