package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/brickpop/foreman/internal/store"
	"github.com/brickpop/foreman/internal/workflow"
)

// fakeReviews knows rev-1; it records what it was asked.
type fakeReviews struct {
	since   bool
	added   store.ReviewComment
	did     []string
	merges  map[string]string
	landing string
	err     error // what a decision answers
}

func (*fakeReviews) ListReviews(context.Context) ([]workflow.Review, error) {
	return []workflow.Review{{Review: store.Review{ID: "rev-1"}, Status: workflow.ReviewNeedsYou}}, nil
}

func (*fakeReviews) Review(_ context.Context, id string) (workflow.Review, error) {
	if id != "rev-1" {
		return workflow.Review{}, store.ErrNotFound
	}
	return workflow.Review{Review: store.Review{ID: id}, Comments: []workflow.Comment{{ReviewComment: store.ReviewComment{ID: "com-1"}, Now: 7, Outdated: true}}}, nil
}

func (f *fakeReviews) ReviewDiff(_ context.Context, _ string, since bool) ([]workflow.RepoDiff, error) {
	f.since = since
	return []workflow.RepoDiff{{Repo: "api", Diff: "+x"}}, nil
}

func (f *fakeReviews) AddComment(_ context.Context, review string, c store.ReviewComment) (store.ReviewComment, error) {
	if c.Body == "" {
		return c, fmt.Errorf("%w: write the comment", workflow.ErrInvalid)
	}
	f.added = c
	c.ID, c.ReviewID = "com-2", review
	return c, nil
}

func (f *fakeReviews) EditComment(_ context.Context, review, id, body string) (store.ReviewComment, error) {
	if id == "com-1" {
		return store.ReviewComment{}, workflow.ErrSent
	}
	f.did = append(f.did, "edit "+review+" "+id+" "+body)
	return store.ReviewComment{ID: id, Body: body}, nil
}

func (f *fakeReviews) DeleteComment(_ context.Context, review, id string) error {
	f.did = append(f.did, "delete "+review+" "+id)
	return nil
}

func (f *fakeReviews) RequestChanges(_ context.Context, id, note string) error {
	f.did = append(f.did, "changes "+id+" "+note)
	return f.err
}

func (f *fakeReviews) MergeReview(_ context.Context, id string, heads map[string]string, landing string) error {
	f.merges, f.landing = heads, landing
	return f.err
}

func (f *fakeReviews) DismissReview(_ context.Context, id string, c workflow.Dismiss) (workflow.Dismissed, error) {
	f.did = append(f.did, fmt.Sprintf("dismiss %s %s drop=%v stop=%v", id, c.Note, c.DropBranches, c.StopTeam))
	return workflow.Dismissed{Kept: []workflow.KeptBranch{{Repo: "api", Branch: "lead/atlas/x", Why: "changes in the way"}}}, f.err
}

// errorCode is an error answer as "CODE: message".
func errorCode(body string) string {
	var got map[string]string
	_ = json.Unmarshal([]byte(body), &got)
	return got["code"] + ": " + got["error"]
}

func TestReviewRoutes(t *testing.T) {
	f := &fakeReviews{}
	h := New(Options{Guard: openGuard{}, Home: fakeHome("/home/ceo/.foreman"), Reviews: f, UI: builtUI})
	var list []workflow.Review
	decodeBody(t, get(t, h, "/api/reviews"), &list)
	if len(list) != 1 || list[0].ID != "rev-1" || list[0].Status != "needs-review" {
		t.Errorf("list %+v", list)
	}
	var one map[string]any
	decodeBody(t, get(t, h, "/api/reviews/rev-1"), &one)
	comments, _ := one["comments"].([]any)
	if one["id"] != "rev-1" || len(comments) != 1 || comments[0].(map[string]any)["now"] != 7.0 || comments[0].(map[string]any)["outdated"] != true {
		t.Errorf("review %v", one)
	}
	if w := send(h, "GET", "/api/reviews/rev-9", ""); w.Code != http.StatusNotFound {
		t.Errorf("unknown review: %d", w.Code)
	}
	var diffs []workflow.RepoDiff
	decodeBody(t, get(t, h, "/api/reviews/rev-1/diff?since=1"), &diffs)
	if len(diffs) != 1 || !f.since {
		t.Errorf("diff %+v since %v", diffs, f.since)
	}
	decodeBody(t, get(t, h, "/api/reviews/rev-1/diff"), &diffs)
	if f.since {
		t.Error("against main by default")
	}

	w := send(h, "POST", "/api/reviews/rev-1/comments", `{"repo":"api","path":"a.go","side":"new","line":3,"commit":"c0ffee","snippet":"x","body":"why?"}`)
	if w.Code != http.StatusOK || f.added.Path != "a.go" || f.added.Line != 3 || f.added.Commit != "c0ffee" {
		t.Errorf("add: %d %s %+v", w.Code, w.Body, f.added)
	}
	if w := send(h, "POST", "/api/reviews/rev-1/comments", `{"body":""}`); w.Code != http.StatusBadRequest || errorCode(w.Body.String()) != "INVALID: write the comment" {
		t.Errorf("an empty comment: %d %s", w.Code, w.Body)
	}
	if w := send(h, "POST", "/api/reviews/rev-1/comments", `not json`); w.Code != http.StatusBadRequest {
		t.Errorf("not json: %d", w.Code)
	}
	if w := send(h, "PUT", "/api/reviews/rev-1/comments/com-2", `{"body":"better"}`); w.Code != http.StatusOK {
		t.Errorf("edit: %d %s", w.Code, w.Body)
	}
	if w := send(h, "PUT", "/api/reviews/rev-1/comments/com-1", `{"body":"x"}`); w.Code != http.StatusConflict {
		t.Errorf("edit a sent one: %d", w.Code)
	}
	if w := send(h, "DELETE", "/api/reviews/rev-1/comments/com-2", ""); w.Code != http.StatusNoContent {
		t.Errorf("delete: %d", w.Code)
	}
	if w := send(h, "POST", "/api/reviews/rev-1/changes", `{"note":"see below"}`); w.Code != http.StatusNoContent {
		t.Errorf("changes: %d", w.Code)
	}
	if w := send(h, "POST", "/api/reviews/rev-1/merge", `{"heads":{"api":"a1b2","web":"c3d4"}}`); w.Code != http.StatusNoContent || len(f.merges) != 2 || f.merges["api"] != "a1b2" || f.merges["web"] != "c3d4" {
		t.Errorf("merge: %d %v", w.Code, f.merges)
	}
	// Landed under the CEO's name.
	if w := send(h, "POST", "/api/reviews/rev-1/merge", `{"heads":{"api":"a1b2"},"landing":" f/csv "}`); w.Code != http.StatusNoContent || f.landing != "f/csv" {
		t.Errorf("merge onto a landing: %d %q", w.Code, f.landing)
	}
	// The dismissal answers with the branches it kept, for the CEO.
	if w := send(h, "POST", "/api/reviews/rev-1/dismiss", `{"note":"not now","dropBranches":true,"stopTeam":true}`); w.Code != http.StatusOK ||
		strings.TrimSpace(w.Body.String()) != `{"kept":[{"repo":"api","branch":"lead/atlas/x","why":"changes in the way"}]}` {
		t.Errorf("dismiss: %d %s", w.Code, w.Body)
	}
	if got, want := strings.Join(f.did, "; "), "edit rev-1 com-2 better; delete rev-1 com-2; changes rev-1 see below; dismiss rev-1 not now drop=true stop=true"; got != want {
		t.Errorf("did %q, want %q", got, want)
	}

	// A merge that stops in some repos says which, and why; the others merged.
	f.err = errors.Join(fmt.Errorf("api: %w", workflow.ErrConflict), fmt.Errorf("web: %w", workflow.ErrLocalChanges), fmt.Errorf("docs: %w", workflow.ErrBranchMoved))
	w = send(h, "POST", "/api/reviews/rev-1/merge", `{"heads":{"api":"a1b2","web":"c3d4","docs":"e5f6"}}`)
	if msg := errorCode(w.Body.String()); w.Code != http.StatusConflict || !strings.HasPrefix(msg, "NOT_MERGED: api conflicts with its target") || !strings.Contains(msg, ". web: your checkout has uncommitted changes") ||
		!strings.HasSuffix(msg, ". docs: the branch has new commits since you loaded this page. Read them, then merge again") {
		t.Errorf("not merged: %d %s", w.Code, msg)
	}
	f.err = workflow.ErrNotOpen
	if w := send(h, "POST", "/api/reviews/rev-1/changes", `{"note":"x"}`); w.Code != http.StatusConflict {
		t.Errorf("not open: %d", w.Code)
	}
}
