package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/brickpop/foreman/internal/instances"
	"github.com/brickpop/foreman/internal/store"
)

type fakeAgents struct {
	list      []store.Instance
	deployed  []deployRequest
	stopped   []string
	woken     []string
	restarted []string
}

func (f *fakeAgents) Deploy(_ context.Context, team, project string) (store.Deployment, error) {
	switch project {
	case "busy":
		return store.Deployment{}, store.ErrActive
	case "broken":
		return store.Deployment{}, fmt.Errorf("%w: project broken has problems", instances.ErrInvalid)
	case "early":
		return store.Deployment{}, fmt.Errorf("%w: podman isn't running", instances.ErrNotReady)
	}
	f.deployed = append(f.deployed, deployRequest{team, project})
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
	return New(Options{Guard: openGuard{}, Home: fakeHome("/home/ceo/.foreman"), Agents: a, UI: builtUI})
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
	w := send(h, "POST", "/api/deployments", `{"team":"atlas","project":"ledger"}`)
	var dep map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &dep)
	if w.Code != http.StatusCreated || dep["id"] != "atlas@ledger" || len(a.deployed) != 1 || a.deployed[0] != (deployRequest{"atlas", "ledger"}) {
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
		{"POST", "/api/deployments", `{"project":"ledger"}`, 400, "BAD_REQUEST"},
		{"POST", "/api/deployments", `{"team":"atlas","project":"busy"}`, 409, "ALREADY_DEPLOYED"},
		{"POST", "/api/deployments", `{"team":"atlas","project":"broken"}`, 400, "CANT_DEPLOY"},
		{"POST", "/api/deployments", `{"team":"atlas","project":"early"}`, 409, "NOT_READY"},
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

// DeploymentsIn: atlas active on ledger, orion on billing, build on relay,
// atlas stopped on relay.
func (f *fakeAgents) DeploymentsIn(_ context.Context, status string) ([]store.Deployment, error) {
	if status == store.DeploymentStopped {
		return []store.Deployment{{ID: "atlas@relay", ProjectID: "relay", Status: status}}, nil
	}
	return []store.Deployment{
		{ID: "atlas@ledger", ProjectID: "ledger", Status: status},
		{ID: "orion@billing", ProjectID: "billing", Status: status},
		{ID: "build@relay", ProjectID: "relay", Status: status},
	}, nil
}

func TestDeploymentsAreListed(t *testing.T) {
	h := withAgents(&fakeAgents{})
	var list []map[string]any
	decodeBody(t, get(t, h, "/api/deployments"), &list)
	if len(list) != 3 || list[0]["id"] != "atlas@ledger" || list[0]["status"] != "active" {
		t.Errorf("deployments %v", list)
	}
	decodeBody(t, get(t, h, "/api/deployments?status=stopped"), &list)
	if len(list) != 1 || list[0]["id"] != "atlas@relay" {
		t.Errorf("stopped %v", list)
	}
	if w := send(h, "GET", "/api/deployments?status=gone", ""); w.Code != http.StatusBadRequest {
		t.Errorf("a bad status: %d", w.Code)
	}
}

// pages records what each Page was asked.
type pages struct{ asked [][]string }

func (p *pages) Latest(context.Context) (map[string]time.Time, error) {
	return map[string]time.Time{"ledger": time.Date(2026, 11, 1, 9, 0, 0, 0, time.UTC)}, nil
}

func (p *pages) Page(_ context.Context, deployment, before string, limit int) (store.ActivityPage, error) {
	if before == "bad" {
		return store.ActivityPage{}, store.ErrBadCursor
	}
	p.asked = append(p.asked, []string{deployment, before, strconv.Itoa(limit)})
	return store.ActivityPage{Items: []store.ActivityItem{}, Next: "c2"}, nil
}

// A deployment's activity, 50 at a time: the team on the project, or an older
// session.
func TestActivity(t *testing.T) {
	p := &pages{}
	h := New(Options{Guard: openGuard{}, Home: fakeHome("/home/ceo/.foreman"), Agents: &fakeAgents{}, Activity: p, UI: builtUI})
	var page map[string]any
	decodeBody(t, get(t, h, "/api/activity?deployment=atlas@ledger"), &page)
	if page["next"] != "c2" {
		t.Errorf("page %v", page)
	}
	decodeBody(t, get(t, h, "/api/activity?deployment=atlas@relay&before=c2"), &page)
	want := [][]string{{"atlas@ledger", "", "50"}, {"atlas@relay", "c2", "50"}}
	if !slices.EqualFunc(p.asked, want, slices.Equal) {
		t.Errorf("asked %v, want %v", p.asked, want)
	}
	var latest map[string]string
	decodeBody(t, get(t, h, "/api/activity/latest"), &latest)
	if latest["ledger"] != "2026-11-01T09:00:00Z" || len(latest) != 1 {
		t.Errorf("latest %v", latest)
	}
	// A project no longer names it: its one team's deployment does.
	for _, path := range []string{"/api/activity", "/api/activity?project=ledger", "/api/activity?deployment=atlas@ledger&before=bad"} {
		if w := send(h, "GET", path, ""); w.Code != http.StatusBadRequest {
			t.Errorf("%s: %d %s", path, w.Code, w.Body)
		}
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

func (f *fakeAgents) Restart(_ context.Context, id string) error {
	if id != "mara@ledger" {
		return store.ErrNotFound
	}
	f.restarted = append(f.restarted, id)
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

func TestRestart(t *testing.T) {
	a := &fakeAgents{}
	h := withAgents(a)
	if w := send(h, "POST", "/api/instances/mara@ledger/restart", ""); w.Code != http.StatusNoContent || len(a.restarted) != 1 || len(a.woken) != 0 {
		t.Errorf("restart: %d %+v %+v", w.Code, a.restarted, a.woken)
	}
	if w := send(h, "POST", "/api/instances/nobody@ledger/restart", ""); w.Code != http.StatusNotFound {
		t.Errorf("unknown instance: %d", w.Code)
	}
}
