package server

import (
	"context"
	"net/http"

	"github.com/brickpop/foreman/internal/harness"
)

// Harnesses is Settings → Harnesses: the supported harnesses, and their login.
type Harnesses interface {
	Harnesses() []harness.Status
	// Connect starts the harness's login in a terminal and returns the
	// terminal's id, for /ws/pty/{id}.
	Connect(ctx context.Context, id string) (string, error)
	// EndLogin ends the harness's login, if one runs.
	EndLogin(id string) error
}

func harnessRoutes(mux *http.ServeMux, h Harnesses) {
	mux.HandleFunc("GET /api/harnesses", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, h.Harnesses())
	})
	mux.HandleFunc("POST /api/harnesses/{id}/connect", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if !known(h, id) {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "No such harness")
			return
		}
		term, err := h.Connect(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"terminal": term})
	})
	mux.HandleFunc("DELETE /api/harnesses/{id}/connect", func(w http.ResponseWriter, r *http.Request) {
		if !known(h, r.PathValue("id")) {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "No such harness")
			return
		}
		if err := h.EndLogin(r.PathValue("id")); err != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL", err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}

func known(h Harnesses, id string) bool {
	for _, s := range h.Harnesses() {
		if s.ID == id {
			return true
		}
	}
	return false
}
