package server

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/brickpop/foreman/internal/harness"
)

type fakeHarnesses struct{ connected, ended []string }

func (f *fakeHarnesses) EndLogin(id string) error {
	f.ended = append(f.ended, id)
	return nil
}

func (f *fakeHarnesses) Harnesses() []harness.Status {
	return []harness.Status{{ID: "claude-code", Connected: false}}
}

func (f *fakeHarnesses) Connect(_ context.Context, id string) (string, error) {
	f.connected = append(f.connected, id)
	return "connect:" + id, nil
}

func TestHarnessesAndConnect(t *testing.T) {
	f := &fakeHarnesses{}
	h := New(Options{Guard: openGuard{}, Workspace: fakeWorkspace("/ws"), Harnesses: f, UI: builtUI})
	var list []map[string]any
	decodeBody(t, get(t, h, "/api/harnesses"), &list)
	if len(list) != 1 || list[0]["id"] != "claude-code" || list[0]["connected"] != false {
		t.Errorf("harnesses %v", list)
	}
	w := send(h, "POST", "/api/harnesses/claude-code/connect", "")
	var body map[string]string
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if w.Code != http.StatusOK || body["terminal"] != "connect:claude-code" || len(f.connected) != 1 {
		t.Errorf("connect: %d %s", w.Code, w.Body)
	}
	if w := send(h, "POST", "/api/harnesses/pi/connect", ""); w.Code != http.StatusNotFound {
		t.Errorf("unknown harness: %d", w.Code)
	}
	if w := send(h, "DELETE", "/api/harnesses/claude-code/connect", ""); w.Code != http.StatusNoContent || len(f.ended) != 1 {
		t.Errorf("end the login: %d %v", w.Code, f.ended)
	}
	if w := send(h, "DELETE", "/api/harnesses/pi/connect", ""); w.Code != http.StatusNotFound || len(f.ended) != 1 {
		t.Errorf("end an unknown harness's login: %d", w.Code)
	}
}
