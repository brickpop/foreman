package server

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/brickpop/foreman/internal/events"
	"github.com/brickpop/foreman/internal/home"
)

type fakeConfig struct {
	projects []home.Project
	teams    []home.Team
	settings home.SettingsFile
}

func (f fakeConfig) Projects() []home.Project { return f.projects }
func (f fakeConfig) Project(id string) (home.Project, error) {
	for _, p := range f.projects {
		if p.ID == id {
			return p, nil
		}
	}
	return home.Project{}, home.ErrNotFound
}
func (f fakeConfig) Teams() []home.Team { return f.teams }
func (f fakeConfig) Team(id string) (home.Team, error) {
	for _, t := range f.teams {
		if t.ID == id {
			return t, nil
		}
	}
	return home.Team{}, home.ErrNotFound
}
func (f fakeConfig) Settings() home.SettingsFile { return f.settings }
func (f fakeConfig) SaveSettings(context.Context, home.Settings, string) (string, error) {
	return "", nil
}
func (f fakeConfig) SaveTeam(context.Context, home.TeamConfig, string) (home.Team, error) {
	return home.Team{}, nil
}

// The project routes answer by name: "bad" is invalid, "taken" exists, "busy"
// is deployed; the project they return says what was called, in Problems.
func (f fakeConfig) NewProject(_ context.Context, name string) (home.Project, error) {
	return f.changed("NewProject", "", name, "")
}
func (f fakeConfig) AddRepo(_ context.Context, project, name, path string) (home.Project, error) {
	return f.changed("AddRepo", project, name, path)
}
func (f fakeConfig) NewRepo(_ context.Context, project, name, path string) (home.Project, error) {
	return f.changed("NewRepo", project, name, path)
}
func (f fakeConfig) LocateRepo(_ context.Context, project, name, path string) (home.Project, error) {
	return f.changed("LocateRepo", project, name, path)
}
func (f fakeConfig) RemoveRepo(_ context.Context, project, name string) (home.Project, error) {
	return f.changed("RemoveRepo", project, name, "")
}
func (f fakeConfig) SetProfile(_ context.Context, project, profile string) (home.Project, error) {
	return f.changed("SetProfile", project, profile, "")
}

// A project repo from before the scaffold: its wiki.toml is missing.
func (f fakeConfig) Missing(project string) ([]home.Missing, error) {
	if project != "ledger" {
		return nil, home.ErrNotFound
	}
	return []home.Missing{{File: "wiki/wiki.toml", Required: true}}, nil
}
func (f fakeConfig) Scaffold(_ context.Context, project string) error {
	_, err := f.Missing(project)
	return err
}
func (f fakeConfig) changed(call, project, name, path string) (home.Project, error) {
	if project != "" && project != "ledger" {
		return home.Project{}, home.ErrNotFound
	}
	switch name {
	case "bad":
		return home.Project{}, fmt.Errorf("%w: a repo name is lowercase", home.ErrInvalid)
	case "taken":
		return home.Project{}, fmt.Errorf("%w: ledger has a repo taken already", home.ErrExists)
	case "busy":
		return home.Project{}, fmt.Errorf("%w: stop the team on ledger before removing busy", home.ErrDeployed)
	}
	return home.Project{ID: project, Problems: []string{call, name, path}}, nil
}

var config = fakeConfig{
	projects: []home.Project{{ID: "ledger", Repos: []home.Repo{{Name: "ledger-api", Path: "/home/ceo/dev/ledger-api"}}, Version: "v1", Problems: []string{"checks: x"}}},
	teams:    []home.Team{{ID: "atlas", Config: home.TeamConfig{Name: "Atlas"}}},
	settings: home.SettingsFile{Settings: home.DefaultSettings(), Version: "v2"},
}

func withConfig(bus Events) http.Handler {
	return New(Options{Guard: openGuard{}, Home: fakeHome("/home/ceo/.foreman"), Config: config, Events: bus, UI: builtUI})
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
	bus.Publish(events.Event{Type: "config.changed", Data: home.Change{Kind: "team", ID: "atlas", Version: "v3"}})
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
	c.teams = []home.Team{{ID: "atlas", Config: home.TeamConfig{
		Name: "Atlas", Lead: "mara", Agents: map[string]home.Soul{"mara": {Name: "Mara", Role: "team_lead"}},
	}}}
	h := New(Options{Guard: openGuard{}, Home: fakeHome("/home/ceo/.foreman"), Config: c, UI: builtUI})
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
	saved  *home.Settings
	team   *home.TeamConfig
	refuse error
}

// SaveTeam records what it was given, as the home would save it.
func (s savingConfig) SaveTeam(_ context.Context, c home.TeamConfig, version string) (home.Team, error) {
	if s.refuse != nil {
		return home.Team{}, s.refuse
	}
	*s.team = c
	return home.Team{ID: c.Team, Config: c, Version: "v4"}, nil
}

func (s savingConfig) SaveSettings(_ context.Context, v home.Settings, version string) (string, error) {
	if s.refuse != nil {
		return "", s.refuse
	}
	*s.saved = v
	return "v3", nil
}

func TestSaveSettings(t *testing.T) {
	var saved home.Settings
	h := New(Options{Guard: openGuard{}, Home: fakeHome("/home/ceo/.foreman"), Config: savingConfig{fakeConfig: config, saved: &saved}, UI: builtUI})
	w := send(h, "PUT", "/api/settings", `{"settings":{"maxSessions":4,"port":8888},"version":"v2"}`)
	if w.Code != http.StatusOK || saved.MaxSessions != 4 {
		t.Errorf("save: %d %s %+v", w.Code, w.Body, saved)
	}
	for _, tc := range []struct {
		refuse error
		status int
	}{
		{home.ErrConflict, http.StatusConflict},
		{&home.InvalidError{Problems: []string{"port: 0 is not a port"}}, http.StatusBadRequest},
	} {
		h := New(Options{Guard: openGuard{}, Home: fakeHome("/home/ceo/.foreman"), Config: savingConfig{fakeConfig: config, saved: &saved, refuse: tc.refuse}, UI: builtUI})
		if w := send(h, "PUT", "/api/settings", `{"settings":{},"version":"v2"}`); w.Code != tc.status {
			t.Errorf("%v: %d %s", tc.refuse, w.Code, w.Body)
		}
	}
	if w := send(h, "PUT", "/api/settings", `nope`); w.Code != http.StatusBadRequest {
		t.Errorf("bad body: %d", w.Code)
	}
}

func TestSaveSoul(t *testing.T) {
	agents := map[string]home.Soul{"mara": {Name: "Mara"}, "tove": {Name: "Tove"}}
	c := fakeConfig{teams: []home.Team{{ID: "atlas", Config: home.TeamConfig{Team: "atlas", Agents: agents, Order: []string{"tove", "mara"}}}}}
	var team home.TeamConfig
	h := New(Options{Guard: openGuard{}, Home: fakeHome("/home/ceo/.foreman"), Config: savingConfig{fakeConfig: c, team: &team}, UI: builtUI})

	w := send(h, "PUT", "/api/teams/atlas/agents/tove", `{"soul":{"name":"Tove","personality":"Careful."},"version":"v3"}`)
	if w.Code != http.StatusOK || team.Agents["tove"].Personality != "Careful." || team.Agents["mara"].Name != "Mara" {
		t.Fatalf("save: %d %s %+v", w.Code, w.Body, team)
	}
	if strings.Join(team.Order, ",") != "tove,mara" {
		t.Errorf("the roster order was lost: %v", team.Order)
	}
	// The home's own team is never changed in place.
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
		{home.ErrConflict, http.StatusConflict},
		{&home.InvalidError{Problems: []string{"agents.tove.name: required"}}, http.StatusBadRequest},
	} {
		h := New(Options{Guard: openGuard{}, Home: fakeHome("/home/ceo/.foreman"), Config: savingConfig{fakeConfig: c, team: &team, refuse: tc.refuse}, UI: builtUI})
		if w := send(h, "PUT", "/api/teams/atlas/agents/tove", `{"soul":{},"version":"v3"}`); w.Code != tc.status {
			t.Errorf("%v: %d %s", tc.refuse, w.Code, w.Body)
		}
	}
}

func TestProjectRoutes(t *testing.T) {
	h := withConfig(events.New())
	send := func(method, path, body string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(method, path, strings.NewReader(body)))
		return w
	}
	for _, tc := range []struct {
		method, path, body string
		call               []string
	}{
		{http.MethodPost, "/api/projects", `{"name":"aragon"}`, []string{"NewProject", "aragon", ""}},
		{http.MethodPost, "/api/projects/ledger/repos", `{"name":"web","path":"/home/ceo/dev/web"}`, []string{"AddRepo", "web", "/home/ceo/dev/web"}},
		{http.MethodPost, "/api/projects/ledger/repos", `{"name":"web","path":"/home/ceo/dev/web","new":true}`, []string{"NewRepo", "web", "/home/ceo/dev/web"}},
		{http.MethodPut, "/api/projects/ledger/repos/web", `{"path":"/home/ceo/src/web"}`, []string{"LocateRepo", "web", "/home/ceo/src/web"}},
		{http.MethodDelete, "/api/projects/ledger/repos/web", ``, []string{"RemoveRepo", "web", ""}},
		{http.MethodPut, "/api/projects/ledger/profile", `{"profile":"work"}`, []string{"SetProfile", "work", ""}},
	} {
		var p home.Project
		decodeBody(t, send(tc.method, tc.path, tc.body), &p)
		if strings.Join(p.Problems, "|") != strings.Join(tc.call, "|") {
			t.Errorf("%s %s: called %v, want %v", tc.method, tc.path, p.Problems, tc.call)
		}
	}
	// What a project repo lacks, and creating it.
	var missing []home.Missing
	decodeBody(t, get(t, h, "/api/projects/ledger/scaffold"), &missing)
	if len(missing) != 1 || missing[0].File != "wiki/wiki.toml" || !missing[0].Required {
		t.Errorf("missing %v", missing)
	}
	if w := send(http.MethodPost, "/api/projects/ledger/scaffold", ""); w.Code != http.StatusOK {
		t.Errorf("scaffold: %d %s", w.Code, w.Body)
	}
	for _, w := range []*httptest.ResponseRecorder{get(t, h, "/api/projects/nope/scaffold"), send(http.MethodPost, "/api/projects/nope/scaffold", "")} {
		if w.Code != http.StatusNotFound {
			t.Errorf("an unknown project's scaffold: %d %s", w.Code, w.Body)
		}
	}
	for _, tc := range []struct {
		method, path, body string
		status             int
		want               string
	}{
		{http.MethodPost, "/api/projects", `{"name":"bad"}`, http.StatusBadRequest, `{"code":"INVALID","error":"a repo name is lowercase"}`},
		{http.MethodPost, "/api/projects/ledger/repos", `{"name":"taken","path":"/x"}`, http.StatusConflict, `{"code":"EXISTS","error":"ledger has a repo taken already"}`},
		{http.MethodDelete, "/api/projects/ledger/repos/busy", ``, http.StatusConflict, `{"code":"DEPLOYED","error":"stop the team on ledger before removing busy"}`},
		{http.MethodPost, "/api/projects/nope/repos", `{"name":"x","path":"/x"}`, http.StatusNotFound, `"NOT_FOUND"`},
		{http.MethodPut, "/api/projects/nope/profile", `{"profile":"work"}`, http.StatusNotFound, `"NOT_FOUND"`},
		{http.MethodPost, "/api/projects", `not json`, http.StatusBadRequest, `"BAD_REQUEST"`},
		{http.MethodPut, "/api/projects/ledger/repos/web", `{"path":`, http.StatusBadRequest, `"BAD_REQUEST"`},
		{http.MethodGet, "/api/candidates", ``, http.StatusNotFound, `"NOT_FOUND"`},
	} {
		if w := send(tc.method, tc.path, tc.body); w.Code != tc.status || !strings.Contains(w.Body.String(), tc.want) {
			t.Errorf("%s %s %s: %d %s", tc.method, tc.path, tc.body, w.Code, w.Body)
		}
	}
}
