package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/brickpop/foreman/internal/instances"
	"github.com/brickpop/foreman/internal/store"
)

type fakeAgents struct {
	list     []store.Instance
	deployed []deployRequest
	stopped  []string
	woken    []string
}

func (f *fakeAgents) Deploy(_ context.Context, team, project, goal string) (store.Deployment, error) {
	switch project {
	case "busy":
		return store.Deployment{}, store.ErrActive
	case "broken":
		return store.Deployment{}, fmt.Errorf("%w: project broken has problems", instances.ErrInvalid)
	case "early":
		return store.Deployment{}, fmt.Errorf("%w: podman isn't running", instances.ErrNotReady)
	}
	f.deployed = append(f.deployed, deployRequest{team, project, goal})
	return store.Deployment{ID: team + "@" + project, Status: store.DeploymentActive}, nil
}

func (f *fakeAgents) Stop(_ context.Context, id string) error {
	if id == "nope@x" {
		return store.ErrNotFound
	}
	f.stopped = append(f.stopped, id)
	return nil
}

func (f *fakeAgents) List(context.Context) ([]store.Instance, error) { return f.list, nil }

func withAgents(a Agents) http.Handler {
	return New(Options{Guard: openGuard{}, Workspace: fakeWorkspace("/ws"), Agents: a, UI: builtUI})
}

func send(h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(method, path, strings.NewReader(body)))
	return w
}

func TestInstancesAreListedWithTheirStatus(t *testing.T) {
	h := withAgents(&fakeAgents{list: []store.Instance{{ID: "mara@ledger", Status: "running", Harness: "claude-code"}}})
	var got []map[string]any
	decodeBody(t, get(t, h, "/api/instances"), &got)
	if len(got) != 1 || got[0]["id"] != "mara@ledger" || got[0]["status"] != "running" {
		t.Errorf("instances %v", got)
	}
	if _, internal := got[0]["sandboxPath"]; !internal {
		t.Error("expected the instance's fields")
	}
	empty := withAgents(&fakeAgents{list: []store.Instance{}})
	if w := get(t, empty, "/api/instances"); strings.TrimSpace(w.Body.String()) != "[]" {
		t.Errorf("no instances must be an empty list: %q", w.Body.String())
	}
}

func TestDeployAndStop(t *testing.T) {
	a := &fakeAgents{}
	h := withAgents(a)
	w := send(h, "POST", "/api/deployments", `{"team":"atlas","project":"ledger","goal":"Ship it"}`)
	var dep map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &dep)
	if w.Code != http.StatusCreated || dep["id"] != "atlas@ledger" || len(a.deployed) != 1 || a.deployed[0].Goal != "Ship it" {
		t.Errorf("deploy: %d %s %+v", w.Code, w.Body, a.deployed)
	}
	if w := send(h, "DELETE", "/api/deployments/atlas@ledger", ""); w.Code != http.StatusNoContent || len(a.stopped) != 1 {
		t.Errorf("stop: %d %+v", w.Code, a.stopped)
	}
}

func TestDeployErrors(t *testing.T) {
	h := withAgents(&fakeAgents{})
	for _, tc := range []struct {
		method, path, body string
		status             int
		code               string
	}{
		{"POST", "/api/deployments", `not json`, 400, "BAD_REQUEST"},
		{"POST", "/api/deployments", `{"team":"atlas","project":"ledger","goal":"  "}`, 400, "BAD_REQUEST"},
		{"POST", "/api/deployments", `{"team":"atlas","project":"busy","goal":"g"}`, 409, "ALREADY_DEPLOYED"},
		{"POST", "/api/deployments", `{"team":"atlas","project":"broken","goal":"g"}`, 400, "CANT_DEPLOY"},
		{"POST", "/api/deployments", `{"team":"atlas","project":"early","goal":"g"}`, 409, "NOT_READY"},
		{"POST", "/api/instances/early@ledger/wake", ``, 409, "NOT_READY"},
		{"DELETE", "/api/deployments/nope@x", ``, 404, "NOT_FOUND"},
	} {
		w := send(h, tc.method, tc.path, tc.body)
		var body map[string]string
		_ = json.Unmarshal(w.Body.Bytes(), &body)
		if w.Code != tc.status || body["code"] != tc.code {
			t.Errorf("%s %s %s: %d %s", tc.method, tc.path, tc.body, w.Code, w.Body)
		}
	}
}

func (f *fakeAgents) ActiveDeployments(context.Context) ([]store.Deployment, error) {
	return []store.Deployment{{ID: "atlas@ledger", Goal: "Ship it", Status: "active"}}, nil
}

func TestDeploymentsAreListed(t *testing.T) {
	var list []map[string]any
	decodeBody(t, get(t, withAgents(&fakeAgents{}), "/api/deployments"), &list)
	if len(list) != 1 || list[0]["goal"] != "Ship it" {
		t.Errorf("deployments %v", list)
	}
}

func (f *fakeAgents) Prompt(_ context.Context, id string) (string, error) {
	if id != "mara@ledger" {
		return "", store.ErrNotFound
	}
	return "# You are Mara", nil
}

func TestInstancePrompt(t *testing.T) {
	h := withAgents(&fakeAgents{})
	var body map[string]string
	decodeBody(t, get(t, h, "/api/instances/mara@ledger/prompt"), &body)
	if body["prompt"] != "# You are Mara" {
		t.Errorf("prompt %v", body)
	}
	if w := get(t, h, "/api/instances/nobody@ledger/prompt"); w.Code != http.StatusNotFound {
		t.Errorf("unknown instance: %d", w.Code)
	}
}

func (f *fakeAgents) Wake(_ context.Context, id string) error {
	if id == "early@ledger" {
		return fmt.Errorf("%w: podman isn't running", instances.ErrNotReady)
	}
	if id != "tove@ledger" {
		return store.ErrNotFound
	}
	f.woken = append(f.woken, id)
	return nil
}

func TestWake(t *testing.T) {
	a := &fakeAgents{}
	h := withAgents(a)
	if w := send(h, "POST", "/api/instances/tove@ledger/wake", ""); w.Code != http.StatusNoContent || len(a.woken) != 1 {
		t.Errorf("wake: %d %+v", w.Code, a.woken)
	}
	if w := send(h, "POST", "/api/instances/nobody@ledger/wake", ""); w.Code != http.StatusNotFound {
		t.Errorf("unknown instance: %d", w.Code)
	}
}
