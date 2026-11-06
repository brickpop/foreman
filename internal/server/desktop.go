package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"path/filepath"
	"slices"

	"github.com/brickpop/foreman/internal/desktop"
	"github.com/brickpop/foreman/internal/home"
	"github.com/brickpop/foreman/internal/store"
)

// Desktop opens folders on the desktop of Foreman's machine (implemented by
// desktop.Opener).
type Desktop interface {
	CanOpen() bool
	// Open opens a folder: desktop.ErrUnavailable without a desktop,
	// desktop.ErrNotAFolder for anything but a real folder.
	Open(dir string) error
}

// openRequest is the body of POST /api/desktop/open: an agent's folder
// (instance), or one of a project's file folders (project and folder). The
// server works out the path: the browser never names one.
type openRequest struct {
	Instance string `json:"instance"`
	Project  string `json:"project"`
	Folder   string `json:"folder"` // home.FileFolders
}

func desktopRoutes(mux *http.ServeMux, d Desktop, agents Agents, config Config) {
	mux.HandleFunc("GET /api/desktop", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]bool{"canOpen": d.CanOpen()})
	})
	mux.HandleFunc("POST /api/desktop/open", func(w http.ResponseWriter, r *http.Request) {
		var req openRequest
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<12)).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "INVALID", "Invalid JSON body")
			return
		}
		var dir string
		switch {
		case req.Instance != "" && req.Project == "" && req.Folder == "":
			list, err := agents.List(r.Context())
			if err != nil {
				writeError(w, http.StatusInternalServerError, "INTERNAL", err.Error())
				return
			}
			i := slices.IndexFunc(list, func(i store.Instance) bool { return i.ID == req.Instance })
			if i < 0 {
				writeError(w, http.StatusNotFound, "NOT_FOUND", "No such instance")
				return
			}
			dir = list[i].SandboxPath
		case req.Instance == "" && req.Project != "":
			if !slices.Contains(home.FileFolders, req.Folder) {
				writeError(w, http.StatusBadRequest, "INVALID", "folder: one of inbox, shared")
				return
			}
			p, err := config.Project(req.Project)
			if errors.Is(err, home.ErrNotFound) {
				writeError(w, http.StatusNotFound, "NOT_FOUND", "No such project")
				return
			}
			if err != nil {
				writeError(w, http.StatusInternalServerError, "INTERNAL", err.Error())
				return
			}
			if p.Files == "" {
				writeError(w, http.StatusConflict, "NO_FILES", req.Project+" has no files folder: move it into a workspace first")
				return
			}
			// Made on first use too: before any agent has started.
			if err := p.MakeFiles(); err != nil {
				writeError(w, http.StatusInternalServerError, "INTERNAL", err.Error())
				return
			}
			dir = filepath.Join(p.Files, req.Folder)
		default:
			writeError(w, http.StatusBadRequest, "INVALID", "Name an instance, or a project and a folder")
			return
		}
		switch err := d.Open(dir); {
		case errors.Is(err, desktop.ErrUnavailable):
			writeError(w, http.StatusConflict, "NO_DESKTOP", "This machine has no desktop to open folders on")
		case errors.Is(err, desktop.ErrNotAFolder):
			writeError(w, http.StatusConflict, "NOT_A_FOLDER", err.Error())
		case err != nil:
			writeError(w, http.StatusInternalServerError, "INTERNAL", err.Error())
		default:
			w.WriteHeader(http.StatusNoContent)
		}
	})
}
