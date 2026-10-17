package server

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/brickpop/foreman/internal/store"
	"github.com/brickpop/foreman/internal/workflow"
)

type fakePipeline struct{ rejected string }

func (*fakePipeline) Pipeline(_ context.Context, project string) (workflow.Pipeline, error) {
	return workflow.Pipeline{WorkItems: []store.WorkItem{{ID: "wi-1", Branch: "wt/tove/" + project}}, MergeRequests: []store.MergeRequest{}}, nil
}

func (*fakePipeline) MergeRequest(_ context.Context, id string) (store.MergeRequest, error) {
	if id != "mr-1" {
		return store.MergeRequest{}, store.ErrNotFound
	}
	return store.MergeRequest{ID: id, Branch: "lead/atlas/x"}, nil
}

func (*fakePipeline) Diff(_ context.Context, id string) (string, error) {
	return "+++ b/x", nil
}

func (*fakePipeline) MergeToMain(_ context.Context, id string) error {
	return map[string]error{"mr-1": nil, "mr-2": workflow.ErrConflict, "mr-3": workflow.ErrLocalChanges, "mr-4": workflow.ErrNotOpen, "mr-5": store.ErrNotFound}[id]
}

func (f *fakePipeline) Reject(_ context.Context, id, reason string) error {
	f.rejected = id + ": " + reason
	return nil
}

func TestPipeline(t *testing.T) {
	f := &fakePipeline{}
	h := New(Options{Guard: openGuard{}, Workspace: fakeWorkspace("/ws"), Pipeline: f, UI: builtUI})
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
	var mr store.MergeRequest
	decodeBody(t, get(t, h, "/api/merge-requests/mr-1"), &mr)
	if mr.Branch != "lead/atlas/x" {
		t.Errorf("merge request %+v", mr)
	}
	var diff map[string]string
	decodeBody(t, get(t, h, "/api/merge-requests/mr-1/diff"), &diff)
	if diff["diff"] != "+++ b/x" {
		t.Errorf("diff %v", diff)
	}
	for id, code := range map[string]string{"mr-2": "CONFLICT", "mr-3": "LOCAL_CHANGES", "mr-4": "NOT_OPEN", "mr-5": "NOT_FOUND"} {
		w := send(h, "POST", "/api/merge-requests/"+id+"/merge", "")
		var got map[string]string
		_ = json.Unmarshal(w.Body.Bytes(), &got)
		if got["code"] != code {
			t.Errorf("%s: %d %s, want %s", id, w.Code, w.Body, code)
		}
	}
	if w := send(h, "POST", "/api/merge-requests/mr-1/merge", ""); w.Code != http.StatusNoContent {
		t.Errorf("merge: %d %s", w.Code, w.Body)
	}
	if w := send(h, "POST", "/api/merge-requests/mr-1/reject", `{"reason":"split it"}`); w.Code != http.StatusNoContent || f.rejected != "mr-1: split it" {
		t.Errorf("reject: %d %q", w.Code, f.rejected)
	}
	if w := send(h, "GET", "/api/merge-requests/nope", ""); w.Code != http.StatusNotFound {
		t.Errorf("unknown: %d", w.Code)
	}
}

type fakeRequests struct{}

func (fakeRequests) OpenRequests(context.Context) ([]store.Request, error) {
	return []store.Request{{ID: "req-1", Kind: "review", MergeRequestID: "mr-1"}}, nil
}

func TestRequests(t *testing.T) {
	h := New(Options{Guard: openGuard{}, Workspace: fakeWorkspace("/ws"), Requests: fakeRequests{}, UI: builtUI})
	var list []store.Request
	decodeBody(t, get(t, h, "/api/requests"), &list)
	if len(list) != 1 || list[0].MergeRequestID != "mr-1" {
		t.Errorf("requests %+v", list)
	}
}
