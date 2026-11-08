package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/brickpop/foreman/internal/wiki"
)

// Wiki reads the projects' wikis and makes Foreman's writes to them
// (implemented by wiki.Wikis).
type Wiki interface {
	Tree(project string) ([]wiki.Node, error)
	Entry(project, path string) (wiki.Entry, error)
	// Image is an image an entry links to: its file and content type.
	Image(project, path string) (file, contentType string, err error)
	Boards(project string) ([]wiki.Board, error)
	// SetCheckbox and MoveCard write an entry if it is still at version, and
	// commit it: wiki.ErrConflict when it changed, wiki.ErrInvalid when the
	// write can't apply, wiki.ErrRefused when the wiki library refuses it.
	SetCheckbox(ctx context.Context, project, path string, line int, done bool, version string) error
	MoveCard(ctx context.Context, project, board, path, column, lane, version string) error
}

// svgPolicy lets an SVG draw and nothing else, even opened on its own.
const svgPolicy = "default-src 'none'; style-src 'unsafe-inline'; sandbox"

func wikiRoutes(mux *http.ServeMux, wk Wiki) {
	mux.HandleFunc("GET /api/wiki/{project}/tree", func(w http.ResponseWriter, r *http.Request) {
		t, err := wk.Tree(r.PathValue("project"))
		writeWiki(w, t, err)
	})
	mux.HandleFunc("GET /api/wiki/{project}/entry/{path...}", func(w http.ResponseWriter, r *http.Request) {
		e, err := wk.Entry(r.PathValue("project"), r.PathValue("path"))
		writeWiki(w, e, err)
	})
	mux.HandleFunc("GET /api/wiki/{project}/boards", func(w http.ResponseWriter, r *http.Request) {
		b, err := wk.Boards(r.PathValue("project"))
		writeWiki(w, b, err)
	})
	mux.HandleFunc("PUT /api/wiki/{project}/checkbox/{path...}", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Line    int    `json:"line"`
			Done    bool   `json:"done"`
			Version string `json:"version"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<10)).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "BAD_REQUEST", "expected {line, done, version}")
			return
		}
		err := wk.SetCheckbox(r.Context(), r.PathValue("project"), r.PathValue("path"), req.Line, req.Done, req.Version)
		writeWritten(w, err)
	})
	mux.HandleFunc("PUT /api/wiki/{project}/card/{board}/{path...}", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Value   string `json:"value"`
			Lane    string `json:"lane"`
			Version string `json:"version"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<10)).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "BAD_REQUEST", "expected {value, lane, version}")
			return
		}
		err := wk.MoveCard(r.Context(), r.PathValue("project"), r.PathValue("board"), r.PathValue("path"), req.Value, req.Lane, req.Version)
		writeWritten(w, err)
	})
	mux.HandleFunc("GET /api/wiki/{project}/image/{path...}", func(w http.ResponseWriter, r *http.Request) {
		file, contentType, err := wk.Image(r.PathValue("project"), r.PathValue("path"))
		if err != nil {
			writeWiki(w, nil, err)
			return
		}
		imageHeaders(w, contentType)
		http.ServeFile(w, r, file)
	})
}

// imageHeaders are an image's, wherever it comes from (the wiki, a repo):
// declared, never sniffed, so the browser takes it for what the extension
// says; an SVG may draw and nothing else.
func imageHeaders(w http.ResponseWriter, contentType string) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Type", contentType)
	if contentType == "image/svg+xml" {
		w.Header().Set("Content-Security-Policy", svgPolicy)
	}
}

func writeWiki(w http.ResponseWriter, v any, err error) {
	switch {
	case errors.Is(err, wiki.ErrConflict):
		writeError(w, http.StatusConflict, "CONFLICT", strings.TrimPrefix(err.Error(), wiki.ErrConflict.Error()+": ")+" changed since you read it")
	case errors.Is(err, wiki.ErrInvalid):
		writeError(w, http.StatusBadRequest, "INVALID", strings.TrimPrefix(err.Error(), wiki.ErrInvalid.Error()+": "))
	case errors.Is(err, wiki.ErrRefused):
		writeError(w, http.StatusUnprocessableEntity, "REFUSED", strings.TrimPrefix(err.Error(), wiki.ErrRefused.Error()+": "))
	case errors.Is(err, wiki.ErrNotFound):
		writeError(w, http.StatusNotFound, "NOT_FOUND", err.Error())
	case err != nil:
		writeError(w, http.StatusInternalServerError, "INTERNAL", err.Error())
	default:
		writeJSON(w, http.StatusOK, v)
	}
}

// writeWritten answers a wiki write: 204, or why it was refused.
func writeWritten(w http.ResponseWriter, err error) {
	if err != nil {
		writeWiki(w, nil, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
