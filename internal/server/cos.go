package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/brickpop/foreman/internal/cos"
	"github.com/brickpop/foreman/internal/harness"
	"github.com/brickpop/foreman/internal/home"
	"github.com/brickpop/foreman/internal/instances"
)

// ChiefOfStaff is the Chief of Staff's page (wiki/product/features/chief-of-staff.md):
// its session, started and stopped with the page, and the team it proposes.
type ChiefOfStaff interface {
	// Start starts its session unless it runs, and types opener into it
	// ("" for none); it returns the session's terminal id, for /ws/pty/{id}.
	Start(ctx context.Context, opener string) (string, error)
	// Stop ends its session, if it runs; the proposal on screen goes with it.
	Stop() error
	// Proposal is the team on screen, if any.
	Proposal() (cos.Proposal, bool)
	// Create writes the proposal on screen.
	Create(ctx context.Context, id int) (home.Team, error)
}

// cosSession is the body of POST /api/cos.
type cosSession struct {
	Terminal string        `json:"terminal"`
	Proposal *cos.Proposal `json:"proposal"`
}

func cosRoutes(mux *http.ServeMux, c ChiefOfStaff) {
	mux.HandleFunc("POST /api/cos", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Opener string `json:"opener"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "BAD_REQUEST", "expected {opener}")
			return
		}
		term, err := c.Start(r.Context(), req.Opener)
		switch {
		case errors.Is(err, instances.ErrNotReady):
			writeError(w, http.StatusConflict, "NOT_READY", err.Error())
		case errors.Is(err, harness.ErrNotConnected):
			writeError(w, http.StatusConflict, "NOT_CONNECTED", "Connect the default profile in Settings → Agents → Profiles first")
		case err != nil:
			writeError(w, http.StatusInternalServerError, "INTERNAL", err.Error())
		default:
			body := cosSession{Terminal: term}
			if p, ok := c.Proposal(); ok {
				body.Proposal = &p
			}
			writeJSON(w, http.StatusOK, body)
		}
	})
	mux.HandleFunc("DELETE /api/cos", func(w http.ResponseWriter, r *http.Request) {
		if err := c.Stop(); err != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL", err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /api/cos/proposal", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID int `json:"id"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "BAD_REQUEST", "expected {id}")
			return
		}
		team, err := c.Create(r.Context(), req.ID)
		switch {
		case errors.Is(err, cos.ErrStale):
			writeError(w, http.StatusConflict, "STALE", "A newer proposal replaced this one")
		case errors.Is(err, cos.ErrChanged):
			writeError(w, http.StatusConflict, "CONFLICT", "The team changed since it was proposed: the Chief of Staff will propose it again")
		case err != nil:
			writeSaveError(w, err, "The team file")
		default:
			writeJSON(w, http.StatusOK, team)
		}
	})
}
