package server

import (
	"errors"
	"net/http"

	"github.com/brickpop/foreman/internal/wiki"
)

// Wiki reads the projects' wikis (implemented by wiki.Wikis).
type Wiki interface {
	Tree(project string) ([]wiki.Node, error)
	Entry(project, path string) (wiki.Entry, error)
	// Image is an image an entry links to: its file and content type.
	Image(project, path string) (file, contentType string, err error)
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
	mux.HandleFunc("GET /api/wiki/{project}/image/{path...}", func(w http.ResponseWriter, r *http.Request) {
		file, contentType, err := wk.Image(r.PathValue("project"), r.PathValue("path"))
		if err != nil {
			writeWiki(w, nil, err)
			return
		}
		// Declared, never sniffed: the browser takes it for what the extension says.
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Content-Type", contentType)
		if contentType == "image/svg+xml" {
			w.Header().Set("Content-Security-Policy", svgPolicy)
		}
		http.ServeFile(w, r, file)
	})
}

func writeWiki(w http.ResponseWriter, v any, err error) {
	switch {
	case errors.Is(err, wiki.ErrNotFound):
		writeError(w, http.StatusNotFound, "NOT_FOUND", err.Error())
	case err != nil:
		writeError(w, http.StatusInternalServerError, "INTERNAL", err.Error())
	default:
		writeJSON(w, http.StatusOK, v)
	}
}
