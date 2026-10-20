package server

import (
	"context"
	"net/http"

	"github.com/brickpop/foreman/internal/runner"
)

// Maintenance is Settings → System → Maintenance: the engine and its images.
type Maintenance interface {
	Status(ctx context.Context) runner.Maintenance
	// Rebuild rebuilds the images without cache (new harness versions).
	Rebuild(ctx context.Context) error
}

func maintenanceRoutes(mux *http.ServeMux, m Maintenance) {
	mux.HandleFunc("GET /api/maintenance", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, m.Status(r.Context()))
	})
	mux.HandleFunc("POST /api/maintenance/rebuild", func(w http.ResponseWriter, r *http.Request) {
		// A rebuild takes minutes: it runs on, and the status shows its progress.
		go func() { _ = m.Rebuild(context.WithoutCancel(r.Context())) }()
		w.WriteHeader(http.StatusAccepted)
	})
}
