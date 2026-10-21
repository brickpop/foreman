package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/brickpop/foreman/internal/store"
	"github.com/brickpop/foreman/internal/workflow"
)

// Reviews are the CEO's reviews of the leads' branches: reading them,
// commenting, and deciding (implemented by Workflow;
// wiki/product/features/reviews.md).
type Reviews interface {
	ListReviews(ctx context.Context) ([]workflow.Review, error)
	Review(ctx context.Context, id string) (workflow.Review, error)
	ReviewDiff(ctx context.Context, id string, since bool) ([]workflow.RepoDiff, error)
	AddComment(ctx context.Context, review string, c store.ReviewComment) (store.ReviewComment, error)
	EditComment(ctx context.Context, review, id, body string) (store.ReviewComment, error)
	DeleteComment(ctx context.Context, review, id string) error
	RequestChanges(ctx context.Context, id, note string) error
	// MergeReview: workflow.ErrConflict, ErrLocalChanges or ErrBranchMoved
	// for each repo that couldn't be merged (the others are).
	MergeReview(ctx context.Context, id string, heads map[string]string) error
	// CloseReview: workflow.ErrInvalid when dropping the branches of a team
	// left running.
	CloseReview(ctx context.Context, id string, c workflow.Close) (workflow.Closed, error)
}

func reviewRoutes(mux *http.ServeMux, rv Reviews) {
	mux.HandleFunc("GET /api/reviews", func(w http.ResponseWriter, r *http.Request) {
		list, err := rv.ListReviews(r.Context())
		writeReview(w, list, err)
	})
	mux.HandleFunc("GET /api/reviews/{id}", func(w http.ResponseWriter, r *http.Request) {
		v, err := rv.Review(r.Context(), r.PathValue("id"))
		writeReview(w, v, err)
	})
	mux.HandleFunc("GET /api/reviews/{id}/diff", func(w http.ResponseWriter, r *http.Request) {
		diffs, err := rv.ReviewDiff(r.Context(), r.PathValue("id"), r.URL.Query().Get("since") == "1")
		writeReview(w, diffs, err)
	})
	mux.HandleFunc("POST /api/reviews/{id}/comments", func(w http.ResponseWriter, r *http.Request) {
		var c store.ReviewComment
		if !decode(w, r, &c, "expected {repo, path, side, line, commit, snippet, body}") {
			return
		}
		saved, err := rv.AddComment(r.Context(), r.PathValue("id"), c)
		writeReview(w, saved, err)
	})
	mux.HandleFunc("PUT /api/reviews/{id}/comments/{comment}", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Body string `json:"body"`
		}
		if !decode(w, r, &req, "expected {body}") {
			return
		}
		saved, err := rv.EditComment(r.Context(), r.PathValue("id"), r.PathValue("comment"), req.Body)
		writeReview(w, saved, err)
	})
	mux.HandleFunc("DELETE /api/reviews/{id}/comments/{comment}", func(w http.ResponseWriter, r *http.Request) {
		writeReview(w, nil, rv.DeleteComment(r.Context(), r.PathValue("id"), r.PathValue("comment")))
	})
	mux.HandleFunc("POST /api/reviews/{id}/changes", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Note string `json:"note"`
		}
		if !decode(w, r, &req, "expected {note}") {
			return
		}
		writeReview(w, nil, rv.RequestChanges(r.Context(), r.PathValue("id"), req.Note))
	})
	mux.HandleFunc("POST /api/reviews/{id}/merge", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Heads map[string]string `json:"heads"`
		}
		if !decode(w, r, &req, "expected {heads}: each repo to merge, with the commit you read") {
			return
		}
		writeReview(w, nil, rv.MergeReview(r.Context(), r.PathValue("id"), req.Heads))
	})
	mux.HandleFunc("POST /api/reviews/{id}/close", func(w http.ResponseWriter, r *http.Request) {
		var req workflow.Close
		if !decode(w, r, &req, "expected {note, dropBranches, stopTeam}") {
			return
		}
		closed, err := rv.CloseReview(r.Context(), r.PathValue("id"), req)
		writeReview(w, closed, err)
	})
}

// decode reads a small JSON body, answering 400 when it isn't one.
func decode(w http.ResponseWriter, r *http.Request, v any, expected string) bool {
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(v); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", expected)
		return false
	}
	return true
}

// writeReview answers a review read or decision; a decision answers 204.
func writeReview(w http.ResponseWriter, v any, err error) {
	switch {
	case err == nil && v == nil:
		w.WriteHeader(http.StatusNoContent)
	case err == nil:
		writeJSON(w, http.StatusOK, v)
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "NOT_FOUND", "No such review or comment")
	case errors.Is(err, workflow.ErrInvalid):
		writeError(w, http.StatusBadRequest, "INVALID", strings.TrimPrefix(err.Error(), workflow.ErrInvalid.Error()+": "))
	case errors.Is(err, workflow.ErrSent):
		writeError(w, http.StatusConflict, "SENT", "That comment was sent to the lead already: it stays as they got it")
	case errors.Is(err, workflow.ErrNotOpen):
		writeError(w, http.StatusConflict, "NOT_OPEN", "Nothing open to decide there: it was merged or closed already")
	case errors.Is(err, workflow.ErrConflict), errors.Is(err, workflow.ErrLocalChanges), errors.Is(err, workflow.ErrBranchMoved):
		writeError(w, http.StatusConflict, "NOT_MERGED", notMerged(err))
	default:
		writeError(w, http.StatusInternalServerError, "INTERNAL", err.Error())
	}
}

// notMerged says, repo by repo, why a merge didn't happen; the repos not
// named were merged.
func notMerged(err error) string {
	parts := []error{err}
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		parts = joined.Unwrap()
	}
	var lines []string
	for _, e := range parts {
		repo, _, _ := strings.Cut(e.Error(), ":")
		switch {
		case errors.Is(e, workflow.ErrConflict):
			lines = append(lines, repo+" conflicts with its main: the lead was asked to merge it in, then to ask again")
		case errors.Is(e, workflow.ErrLocalChanges):
			lines = append(lines, repo+": your checkout has uncommitted changes the merge would overwrite: commit or stash them, then merge again")
		case errors.Is(e, workflow.ErrBranchMoved):
			lines = append(lines, repo+": the branch has new commits since you loaded this page. Read them, then merge again")
		default:
			lines = append(lines, e.Error())
		}
	}
	return strings.Join(lines, ". ")
}
