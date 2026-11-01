package server

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/brickpop/foreman/internal/git"
	"github.com/brickpop/foreman/internal/home"
	"github.com/brickpop/foreman/internal/repos"
)

// Repos reads a project's repos for its Repositories tab (implemented by the
// repos block), read-only.
type Repos interface {
	Repos(ctx context.Context, project string) ([]repos.Repo, error)
	Tree(ctx context.Context, project, repo, ref, path string) ([]git.TreeEntry, error)
	File(ctx context.Context, project, repo, ref, path string) (repos.File, error)
	Log(ctx context.Context, project, repo, ref string, all bool, skip int) (repos.LogPage, error)
}

func repoRoutes(mux *http.ServeMux, rs Repos) {
	mux.HandleFunc("GET /api/projects/{id}/repos", func(w http.ResponseWriter, r *http.Request) {
		list, err := rs.Repos(r.Context(), r.PathValue("id"))
		writeRead(w, list, err)
	})
	mux.HandleFunc("GET /api/projects/{id}/repos/{repo}/tree", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		list, err := rs.Tree(r.Context(), r.PathValue("id"), r.PathValue("repo"), q.Get("ref"), q.Get("path"))
		writeRead(w, list, err)
	})
	mux.HandleFunc("GET /api/projects/{id}/repos/{repo}/file", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		f, err := rs.File(r.Context(), r.PathValue("id"), r.PathValue("repo"), q.Get("ref"), q.Get("path"))
		writeRead(w, f, err)
	})
	mux.HandleFunc("GET /api/projects/{id}/repos/{repo}/log", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		skip := 0
		if s := q.Get("skip"); s != "" {
			n, err := strconv.Atoi(s)
			if err != nil || n < 0 {
				writeError(w, http.StatusBadRequest, "BAD_REQUEST", "expected ?skip=<a count>")
				return
			}
			skip = n
		}
		page, err := rs.Log(r.Context(), r.PathValue("id"), r.PathValue("repo"), q.Get("ref"), q.Get("all") == "1", skip)
		writeRead(w, page, err)
	})
}

// writeRead answers a read of a repo: 404 for a project, repo, ref or path
// that isn't there.
func writeRead(w http.ResponseWriter, v any, err error) {
	switch {
	case errors.Is(err, home.ErrNotFound):
		writeError(w, http.StatusNotFound, "NOT_FOUND", "No such project")
	case errors.Is(err, repos.ErrNoRepo):
		writeError(w, http.StatusNotFound, "NOT_FOUND", "No such repo in this project")
	case errors.Is(err, git.ErrNoRef):
		writeError(w, http.StatusNotFound, "NOT_FOUND", "No such branch or commit")
	case errors.Is(err, git.ErrNoPath):
		writeError(w, http.StatusNotFound, "NOT_FOUND", "No such file or folder")
	case err != nil:
		writeError(w, http.StatusInternalServerError, "INTERNAL", err.Error())
	default:
		writeJSON(w, http.StatusOK, v)
	}
}
