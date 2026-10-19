package server

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/brickpop/foreman/internal/userdata"
)

// Preferences are the CEO's, the same in every workspace (Settings → Global →
// Appearance), kept outside any workspace.
type Preferences interface {
	Get() userdata.Preferences
	Set(userdata.Preferences) error
}

func preferenceRoutes(mux *http.ServeMux, p Preferences) {
	mux.HandleFunc("GET /api/preferences", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, p.Get())
	})
	mux.HandleFunc("PUT /api/preferences", func(w http.ResponseWriter, r *http.Request) {
		// The fields named change; the rest stay as they are.
		req := p.Get()
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<10)).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "BAD_REQUEST", "expected the preferences")
			return
		}
		err := p.Set(req)
		switch {
		case errors.Is(err, userdata.ErrInvalid):
			writeError(w, http.StatusBadRequest, "INVALID", err.Error())
		case err != nil:
			writeError(w, http.StatusInternalServerError, "INTERNAL", err.Error())
		default:
			writeJSON(w, http.StatusOK, p.Get())
		}
	})
}
