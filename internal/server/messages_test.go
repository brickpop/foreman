package server

import (
	"context"
	"net/http"
	"testing"

	"github.com/brickpop/foreman/internal/store"
)

type fakeMessages struct{ asked string }

func (f *fakeMessages) Traffic(_ context.Context, deployment string) ([]store.TeamMessage, error) {
	f.asked = deployment
	return []store.TeamMessage{{Ref: "msg-1", DeploymentID: deployment, Kind: "ask", Body: "build cal"}}, nil
}

func TestMessagesOfADeployment(t *testing.T) {
	f := &fakeMessages{}
	h := New(Options{Guard: openGuard{}, Workspace: fakeWorkspace("/ws"), Messages: f, UI: builtUI})
	var list []map[string]any
	decodeBody(t, get(t, h, "/api/messages?deployment=atlas%40ledger"), &list)
	if f.asked != "atlas@ledger" || len(list) != 1 || list[0]["ref"] != "msg-1" || list[0]["body"] != "build cal" {
		t.Errorf("asked %q, got %v", f.asked, list)
	}
	if w := get(t, h, "/api/messages"); w.Code != http.StatusBadRequest {
		t.Errorf("without a deployment: %d", w.Code)
	}
}
