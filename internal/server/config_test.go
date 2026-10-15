package server

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/brickpop/foreman/internal/events"
	"github.com/brickpop/foreman/internal/workspace"
)

type fakeConfig struct {
	projects []workspace.Project
	teams    []workspace.Team
	settings workspace.SettingsFile
}

func (f fakeConfig) Projects() []workspace.Project { return f.projects }
func (f fakeConfig) Project(id string) (workspace.Project, error) {
	for _, p := range f.projects {
		if p.ID == id {
			return p, nil
		}
	}
	return workspace.Project{}, workspace.ErrNotFound
}
func (f fakeConfig) Teams() []workspace.Team { return f.teams }
func (f fakeConfig) Team(id string) (workspace.Team, error) {
	for _, t := range f.teams {
		if t.ID == id {
			return t, nil
		}
	}
	return workspace.Team{}, workspace.ErrNotFound
}
func (f fakeConfig) Settings() workspace.SettingsFile { return f.settings }
func (f fakeConfig) SaveSettings(context.Context, workspace.Settings, string) (string, error) {
	return "", nil
}
func (f fakeConfig) SaveTeam(context.Context, workspace.TeamConfig, string) (workspace.Team, error) {
	return workspace.Team{}, nil
}

var config = fakeConfig{
	projects: []workspace.Project{{ID: "ledger", Repos: []string{"ledger-api"}, Version: "v1", Problems: []string{"checks: x"}}},
	teams:    []workspace.Team{{ID: "atlas", Config: workspace.TeamConfig{Name: "Atlas"}}},
	settings: workspace.SettingsFile{Settings: workspace.DefaultSettings(), Version: "v2"},
}

func withConfig(bus Events) http.Handler {
	return New(Options{Guard: openGuard{}, Workspace: fakeWorkspace("/ws"), Config: config, Events: bus, UI: builtUI})
}

func decodeBody(t *testing.T, w *httptest.ResponseRecorder, v any) {
	t.Helper()
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body)
	}
	if err := json.Unmarshal(w.Body.Bytes(), v); err != nil {
		t.Fatalf("%v: %s", err, w.Body)
	}
}

func TestConfigEndpoints(t *testing.T) {
	h := withConfig(events.New())
	var projects []map[string]any
	decodeBody(t, get(t, h, "/api/projects"), &projects)
	if len(projects) != 1 || projects[0]["id"] != "ledger" || projects[0]["version"] != "v1" || projects[0]["problems"] == nil {
		t.Errorf("projects %v", projects)
	}
	var team map[string]any
	decodeBody(t, get(t, h, "/api/teams/atlas"), &team)
	if team["config"].(map[string]any)["name"] != "Atlas" {
		t.Errorf("team %v", team)
	}
	var settings map[string]any
	decodeBody(t, get(t, h, "/api/settings"), &settings)
	if settings["version"] != "v2" || settings["settings"].(map[string]any)["port"] != float64(8888) {
		t.Errorf("settings %v", settings)
	}
	var teams []any
	decodeBody(t, get(t, h, "/api/teams"), &teams)
	if len(teams) != 1 {
		t.Errorf("teams %v", teams)
	}
}

func TestUnknownIdsAre404InTheErrorFormat(t *testing.T) {
	h := withConfig(events.New())
	for _, path := range []string{"/api/projects/nope", "/api/teams/nope"} {
		w := get(t, h, path)
		var body map[string]string
		_ = json.Unmarshal(w.Body.Bytes(), &body)
		if w.Code != http.StatusNotFound || body["code"] != "NOT_FOUND" || body["error"] == "" {
			t.Errorf("%s: %d %s", path, w.Code, w.Body)
		}
	}
}

// readEvent reads the data of one server-sent event, skipping comments.
func readEvent(t *testing.T, r *bufio.Reader) string {
	t.Helper()
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			t.Fatalf("stream ended: %v", err)
		}
		if data, ok := strings.CutPrefix(strings.TrimRight(line, "\n"), "data: "); ok {
			return data
		}
	}
}

func TestEventsStreamWhatIsPublished(t *testing.T) {
	bus := events.New()
	srv := httptest.NewServer(withConfig(bus))
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/api/events")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("Content-Type %q", ct)
	}
	r := bufio.NewReader(resp.Body)
	if line, _ := r.ReadString('\n'); !strings.HasPrefix(line, ":") {
		t.Fatalf("first line %q: the stream must open at once", line)
	}
	bus.Publish(events.Event{Type: "config.changed", Data: workspace.Change{Kind: "team", ID: "atlas", Version: "v3"}})
	if data := readEvent(t, r); data != `{"type":"config.changed","data":{"object":"team","id":"atlas","version":"v3"}}` {
		t.Errorf("got %s", data)
	}
}

// endedSubscription is a bus that has already dropped its subscriber, as it
// does with one that fell behind.
type endedSubscription struct{}

func (endedSubscription) Subscribe() (<-chan events.Event, func()) {
	ch := make(chan events.Event)
	close(ch)
	return ch, func() {}
}

// When the bus drops the subscription, the stream ends, so the browser
// reconnects and refetches.
func TestEventsStreamEndsWithItsSubscription(t *testing.T) {
	srv := httptest.NewServer(withConfig(endedSubscription{}))
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/api/events")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	done := make(chan struct{})
	go func() { _, _ = io.Copy(io.Discard, resp.Body); close(done) }()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("the stream outlived its subscription")
	}
}

func TestPromptTemplateOfAnAgent(t *testing.T) {
	c := config
	c.teams = []workspace.Team{{ID: "atlas", Config: workspace.TeamConfig{
		Name: "Atlas", Lead: "mara", Agents: map[string]workspace.Soul{"mara": {Name: "Mara", Role: "team_lead"}},
	}}}
	h := New(Options{Guard: openGuard{}, Workspace: fakeWorkspace("/ws"), Config: c, UI: builtUI})
	var body map[string]string
	decodeBody(t, get(t, h, "/api/teams/atlas/agents/mara/prompt"), &body)
	if !strings.Contains(body["prompt"], "Goal: {goal}") {
		t.Errorf("template %q", body["prompt"])
	}
	for _, path := range []string{"/api/teams/atlas/agents/ghost/prompt", "/api/teams/nope/agents/mara/prompt"} {
		if w := get(t, h, path); w.Code != http.StatusNotFound {
			t.Errorf("%s: %d", path, w.Code)
		}
	}
}

// savingConfig is a config whose settings can be saved.
type savingConfig struct {
	fakeConfig
	saved  *workspace.Settings
	team   *workspace.TeamConfig
	refuse error
}

// SaveTeam records what it was given, as the workspace would save it.
func (s savingConfig) SaveTeam(_ context.Context, c workspace.TeamConfig, version string) (workspace.Team, error) {
	if s.refuse != nil {
		return workspace.Team{}, s.refuse
	}
	*s.team = c
	return workspace.Team{ID: c.Team, Config: c, Version: "v4"}, nil
}

func (s savingConfig) SaveSettings(_ context.Context, v workspace.Settings, version string) (string, error) {
	if s.refuse != nil {
		return "", s.refuse
	}
	*s.saved = v
	return "v3", nil
}

func TestSaveSettings(t *testing.T) {
	var saved workspace.Settings
	h := New(Options{Guard: openGuard{}, Workspace: fakeWorkspace("/ws"), Config: savingConfig{fakeConfig: config, saved: &saved}, UI: builtUI})
	w := send(h, "PUT", "/api/settings", `{"settings":{"maxSessions":4,"port":8888},"version":"v2"}`)
	if w.Code != http.StatusOK || saved.MaxSessions != 4 {
		t.Errorf("save: %d %s %+v", w.Code, w.Body, saved)
	}
	for _, tc := range []struct {
		refuse error
		status int
	}{
		{workspace.ErrConflict, http.StatusConflict},
		{&workspace.InvalidError{Problems: []string{"port: 0 is not a port"}}, http.StatusBadRequest},
	} {
		h := New(Options{Guard: openGuard{}, Workspace: fakeWorkspace("/ws"), Config: savingConfig{fakeConfig: config, saved: &saved, refuse: tc.refuse}, UI: builtUI})
		if w := send(h, "PUT", "/api/settings", `{"settings":{},"version":"v2"}`); w.Code != tc.status {
			t.Errorf("%v: %d %s", tc.refuse, w.Code, w.Body)
		}
	}
	if w := send(h, "PUT", "/api/settings", `nope`); w.Code != http.StatusBadRequest {
		t.Errorf("bad body: %d", w.Code)
	}
}

func TestSaveSoul(t *testing.T) {
	agents := map[string]workspace.Soul{"mara": {Name: "Mara"}, "tove": {Name: "Tove"}}
	c := fakeConfig{teams: []workspace.Team{{ID: "atlas", Config: workspace.TeamConfig{Team: "atlas", Agents: agents, Order: []string{"tove", "mara"}}}}}
	var team workspace.TeamConfig
	h := New(Options{Guard: openGuard{}, Workspace: fakeWorkspace("/ws"), Config: savingConfig{fakeConfig: c, team: &team}, UI: builtUI})

	w := send(h, "PUT", "/api/teams/atlas/agents/tove", `{"soul":{"name":"Tove","personality":"Careful."},"version":"v3"}`)
	if w.Code != http.StatusOK || team.Agents["tove"].Personality != "Careful." || team.Agents["mara"].Name != "Mara" {
		t.Fatalf("save: %d %s %+v", w.Code, w.Body, team)
	}
	if strings.Join(team.Order, ",") != "tove,mara" {
		t.Errorf("the roster order was lost: %v", team.Order)
	}
	// The workspace's own team is never changed in place.
	if agents["tove"].Personality != "" {
		t.Error("the loaded team was changed before the save")
	}

	for path, status := range map[string]int{
		"/api/teams/atlas/agents/nobody": http.StatusNotFound,
		"/api/teams/orion/agents/tove":   http.StatusNotFound,
	} {
		if w := send(h, "PUT", path, `{"soul":{},"version":"v3"}`); w.Code != status {
			t.Errorf("%s: %d", path, w.Code)
		}
	}
	if w := send(h, "PUT", "/api/teams/atlas/agents/tove", `nope`); w.Code != http.StatusBadRequest {
		t.Errorf("bad body: %d", w.Code)
	}
	for _, tc := range []struct {
		refuse error
		status int
	}{
		{workspace.ErrConflict, http.StatusConflict},
		{&workspace.InvalidError{Problems: []string{"agents.tove.name: required"}}, http.StatusBadRequest},
	} {
		h := New(Options{Guard: openGuard{}, Workspace: fakeWorkspace("/ws"), Config: savingConfig{fakeConfig: c, team: &team, refuse: tc.refuse}, UI: builtUI})
		if w := send(h, "PUT", "/api/teams/atlas/agents/tove", `{"soul":{},"version":"v3"}`); w.Code != tc.status {
			t.Errorf("%v: %d %s", tc.refuse, w.Code, w.Body)
		}
	}
}
