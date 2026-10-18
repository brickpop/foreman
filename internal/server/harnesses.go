package server

import (
	"net/http"

	"github.com/brickpop/foreman/internal/harness"
)

// Harnesses lists the supported harnesses and their facts (the keystroke for
// a new line); their credentials are profiles' (profiles.go).
type Harnesses interface {
	Harnesses() []harness.Status
}

func harnessRoutes(mux *http.ServeMux, h Harnesses) {
	mux.HandleFunc("GET /api/harnesses", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, h.Harnesses())
	})
}
