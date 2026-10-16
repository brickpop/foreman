package server

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/brickpop/foreman/internal/instances"
	"github.com/brickpop/foreman/internal/store"
)

type fakeMessages struct {
	asked string
	line  string // instance: text
	pause string // instance: paused
}

func (f *fakeMessages) Line(instance, text string) error {
	if instance == "asleep@ledger" {
		return instances.ErrNotRunning
	}
	f.line = instance + ": " + text
	return nil
}

func (f *fakeMessages) PauseCountdown(instance string, paused bool) {
	f.pause = fmt.Sprintf("%s: %v", instance, paused)
}

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

func TestTheCEOsLineAndTheCountdown(t *testing.T) {
	f := &fakeMessages{}
	h := New(Options{Guard: openGuard{}, Workspace: fakeWorkspace("/ws"), Messages: f, UI: builtUI})
	do := func(method, path, body string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(method, path, strings.NewReader(body)))
		return w
	}
	if w := do(http.MethodPost, "/api/instances/mara%40ledger/line", `{"text":"hi\nthere"}`); w.Code != http.StatusNoContent || f.line != "mara@ledger: hi\nthere" {
		t.Errorf("%d %q", w.Code, f.line)
	}
	if w := do(http.MethodPost, "/api/instances/asleep%40ledger/line", `{"text":"hi"}`); w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), "NOT_RUNNING") {
		t.Errorf("not running: %d %s", w.Code, w.Body)
	}
	if w := do(http.MethodPut, "/api/instances/mara%40ledger/countdown", `{"paused":true}`); w.Code != http.StatusNoContent || f.pause != "mara@ledger: true" {
		t.Errorf("%d %q", w.Code, f.pause)
	}
	if w := do(http.MethodPost, "/api/instances/mara%40ledger/line", `nope`); w.Code != http.StatusBadRequest {
		t.Errorf("bad body: %d", w.Code)
	}
}
