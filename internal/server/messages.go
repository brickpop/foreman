package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

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

// Actions is the git tools agents called (implemented by the state store).
type Actions interface {
	List(ctx context.Context, deployment string) ([]store.TeamAction, error)
}

func actionRoutes(mux *http.ServeMux, a Actions) {
	mux.HandleFunc("GET /api/actions", func(w http.ResponseWriter, r *http.Request) {
		deployment := r.URL.Query().Get("deployment")
		if deployment == "" {
			writeError(w, http.StatusBadRequest, "BAD_REQUEST", "expected ?deployment=<team@project>")
			return
		}
		list, err := a.List(r.Context(), deployment)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, list)
	})
}

// Activity pages team traffic, messages and git actions together (implemented
// by the state store).
type Activity interface {
	Page(ctx context.Context, deployments []string, before string, limit int) (store.ActivityPage, error)
}

// activityPage is how many items a page of activity holds (the CEO, 2026-10-17).
const activityPage = 50

// activityRoutes serve a project's Activity tab: its active deployments'
// traffic merged, or one deployment's (an older session).
func activityRoutes(mux *http.ServeMux, act Activity, a Agents) {
	mux.HandleFunc("GET /api/activity", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		project, deployment := q.Get("project"), q.Get("deployment")
		var deployments []string
		switch {
		case (project == "") == (deployment == ""):
			writeError(w, http.StatusBadRequest, "BAD_REQUEST", "expected ?project=<id> or ?deployment=<team@project>")
			return
		case deployment != "":
			deployments = []string{deployment}
		default:
			active, err := a.DeploymentsIn(r.Context(), store.DeploymentActive)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "INTERNAL", err.Error())
				return
			}
			for _, d := range active {
				if d.ProjectID == project {
					deployments = append(deployments, d.ID)
				}
			}
		}
		page, err := act.Page(r.Context(), deployments, q.Get("before"), activityPage)
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
