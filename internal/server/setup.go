package server

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/brickpop/foreman/internal/setup"
)

// Setup is the setup checklist on Home (wiki/product/features/first-run.md).
type Setup interface {
	Steps(ctx context.Context) []setup.Step
	// StartPodman starts the stopped Podman machine, in the background.
	StartPodman(ctx context.Context)
	// CreateMachine creates the Podman machine (macOS), memory in GiB, in the
	// background; it refuses a size out of bounds, or a machine that exists.
	CreateMachine(ctx context.Context, cpus, memory int) error
}

func setupRoutes(mux *http.ServeMux, s Setup) {
	mux.HandleFunc("GET /api/setup", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, s.Steps(r.Context()))
	})
	mux.HandleFunc("POST /api/setup/podman/start", func(w http.ResponseWriter, r *http.Request) {
		s.StartPodman(r.Context())
		w.WriteHeader(http.StatusAccepted)
	})
	mux.HandleFunc("POST /api/setup/podman/machine", func(w http.ResponseWriter, r *http.Request) {
		var req struct{ CPUs, Memory int }
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<10)).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "BAD_REQUEST", "expected {cpus, memory}")
			return
		}
		if err := s.CreateMachine(r.Context(), req.CPUs, req.Memory); err != nil {
			writeError(w, http.StatusConflict, "CANT_CREATE", err.Error())
			return
		}
		w.WriteHeader(http.StatusAccepted)
	})
}
