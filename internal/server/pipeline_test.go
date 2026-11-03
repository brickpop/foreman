package server

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/brickpop/foreman/internal/home"
	"github.com/brickpop/foreman/internal/store"
	"github.com/brickpop/foreman/internal/workflow"
)

type fakePipeline struct{}

func (fakePipeline) Pipeline(_ context.Context, project string) (workflow.Pipeline, error) {
	return workflow.Pipeline{WorkItems: []store.WorkItem{{ID: "wi-1", Branch: "wt/tove/" + project}}, MergeRequests: []store.MergeRequest{}}, nil
}

func (fakePipeline) CleanUp(_ context.Context, project string) (workflow.Cleanup, error) {
	if project != "ledger" {
		return workflow.Cleanup{}, home.ErrNotFound
	}
	return workflow.Cleanup{Removed: []workflow.Cleaned{{Agent: "tove", Repo: "api", What: "worktree", Branch: "wt/tove/main"}}}, nil
}

func TestPipeline(t *testing.T) {
	h := New(Options{Guard: openGuard{}, Home: fakeHome("/home/ceo/.foreman"), Pipeline: fakePipeline{}, UI: builtUI})
	var p struct {
		WorkItems     []store.WorkItem     `json:"workItems"`
		MergeRequests []store.MergeRequest `json:"mergeRequests"`
	}
	decodeBody(t, get(t, h, "/api/pipeline?project=ledger"), &p)
	if len(p.WorkItems) != 1 || p.WorkItems[0].Branch != "wt/tove/ledger" || p.MergeRequests == nil {
		t.Errorf("pipeline %+v", p)
	}
	if w := send(h, "GET", "/api/pipeline", ""); w.Code != http.StatusBadRequest {
		t.Errorf("without a project: %d", w.Code)
	}
	var done workflow.Cleanup
	decodeBody(t, send(h, "POST", "/api/projects/ledger/cleanup", ""), &done)
	if len(done.Removed) != 1 || done.Removed[0].Agent != "tove" {
		t.Errorf("cleanup %+v", done)
	}
	if w := send(h, "POST", "/api/projects/nope/cleanup", ""); w.Code != http.StatusNotFound {
		t.Errorf("cleanup of no project: %d", w.Code)
	}
	// The CEO decides in a review: the merge request's own decisions are gone.
	for _, path := range []string{"/api/merge-requests/mr-1/merge", "/api/merge-requests/mr-1/reject"} {
		if w := send(h, "POST", path, "{}"); w.Code != http.StatusNotFound {
			t.Errorf("%s: %d", path, w.Code)
		}
	}
}

type fakeRequests struct{}

func (fakeRequests) OpenRequests(context.Context) ([]store.Request, error) {
	return []store.Request{{ID: "req-1", Kind: "review", ReviewID: "rev-1"}}, nil
}

// Answer: req-2 is open, req-3 answered already; "?" is no answer.
func (fakeRequests) Answer(_ context.Context, id, answer string) (store.Request, error) {
	switch {
	case answer == "?":
		return store.Request{}, fmt.Errorf("%w: write an answer", workflow.ErrInvalid)
	case id != "req-2":
		return store.Request{}, workflow.ErrAnswered
	}
	return store.Request{ID: id, Status: store.RequestResolved, Answer: answer}, nil
}

func TestRequests(t *testing.T) {
	h := New(Options{Guard: openGuard{}, Home: fakeHome("/home/ceo/.foreman"), Requests: fakeRequests{}, UI: builtUI})
	var list []store.Request
	decodeBody(t, get(t, h, "/api/requests"), &list)
	if len(list) != 1 || list[0].ReviewID != "rev-1" {
		t.Errorf("requests %+v", list)
	}
	var answered store.Request
	decodeBody(t, send(h, "POST", "/api/requests/req-2/resolve", `{"answer":"Approve"}`), &answered)
	if answered.Answer != "Approve" || answered.Status != store.RequestResolved {
		t.Errorf("answered %+v", answered)
	}
	for body, code := range map[string]int{`{"answer":"?"}`: http.StatusBadRequest, `{"answer":`: http.StatusBadRequest} {
		if w := send(h, "POST", "/api/requests/req-2/resolve", body); w.Code != code {
			t.Errorf("%s: %d", body, w.Code)
		}
	}
	if w := send(h, "POST", "/api/requests/req-3/resolve", `{"answer":"Approve"}`); w.Code != http.StatusConflict {
		t.Errorf("answered already: %d", w.Code)
	}
}
