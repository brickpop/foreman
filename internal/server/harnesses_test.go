package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/brickpop/foreman/internal/harness"
	"github.com/brickpop/foreman/internal/profiles"
)

type fakeHarnesses struct{}

func (fakeHarnesses) Harnesses() []harness.Status {
	return []harness.Status{{ID: "claude-code", Command: "claude", Newline: "\\\r"}}
}

func TestHarnesses(t *testing.T) {
	h := New(Options{Guard: openGuard{}, Workspace: fakeWorkspace("/ws"), Harnesses: fakeHarnesses{}, UI: builtUI})
	var list []map[string]any
	decodeBody(t, get(t, h, "/api/harnesses"), &list)
	if len(list) != 1 || list[0]["id"] != "claude-code" || list[0]["newline"] != "\\\r" {
		t.Errorf("harnesses %v", list)
	}
}

// fakeProfiles answers as the store does: default exists, work is created.
type fakeProfiles struct {
	list             []profiles.Profile
	connected, ended []string
	endpoint         harness.Endpoint
	detect           []string // the server's models; nil: unreachable
	detectKey        string
}

func (f *fakeProfiles) List() ([]profiles.Profile, error) { return f.list, nil }

func (f *fakeProfiles) harness(name string) string {
	for _, p := range f.list {
		if p.Name == name {
			return p.Harness
		}
	}
	return ""
}

func (f *fakeProfiles) find(name string) error {
	for _, p := range f.list {
		if p.Name == name {
			return nil
		}
	}
	return profiles.ErrNotFound
}

func (f *fakeProfiles) Create(name, h string) (profiles.Profile, error) {
	switch {
	case h != "claude-code":
		return profiles.Profile{}, ErrUnknownHarness
	case !profiles.Valid(name):
		return profiles.Profile{}, profiles.ErrInvalid
	case f.find(name) == nil:
		return profiles.Profile{}, profiles.ErrExists
	}
	p := profiles.Profile{Name: name, Harness: h}
	f.list = append(f.list, p)
	return p, nil
}

func (f *fakeProfiles) Delete(name string) error {
	if name == profiles.Default {
		return profiles.ErrInvalid
	}
	return f.find(name)
}

func (f *fakeProfiles) Connect(_ context.Context, name string) (string, error) {
	if err := f.find(name); err != nil {
		return "", err
	}
	if f.harness(name) == "pi" {
		return "", ErrConnectKind
	}
	f.connected = append(f.connected, name)
	return "connect:" + name, nil
}

func (f *fakeProfiles) EndLogin(name string) error {
	if err := f.find(name); err != nil {
		return err
	}
	f.ended = append(f.ended, name)
	return nil
}

func TestProfiles(t *testing.T) {
	f := &fakeProfiles{list: []profiles.Profile{{Name: "default", Harness: "claude-code", Connected: true}}}
	h := New(Options{Guard: openGuard{}, Workspace: fakeWorkspace("/ws"), Profiles: f, UI: builtUI})
	var list []profiles.Profile
	if decodeBody(t, get(t, h, "/api/profiles"), &list); len(list) != 1 || !list[0].Connected {
		t.Errorf("profiles %+v", list)
	}
	for _, c := range []struct {
		body string
		want int
	}{
		{`{"name":"work","harness":"claude-code"}`, http.StatusCreated},
		{`{"name":"work","harness":"claude-code"}`, http.StatusConflict},
		{`{"name":"Work","harness":"claude-code"}`, http.StatusBadRequest},
		{`{"name":"pi","harness":"pi"}`, http.StatusBadRequest},
		{`not json`, http.StatusBadRequest},
	} {
		if w := send(h, "POST", "/api/profiles", c.body); w.Code != c.want {
			t.Errorf("create %s: %d %s, want %d", c.body, w.Code, w.Body, c.want)
		}
	}
	for path, want := range map[string]int{"/api/profiles/work": http.StatusNoContent, "/api/profiles/default": http.StatusBadRequest, "/api/profiles/nope": http.StatusNotFound} {
		if w := send(h, "DELETE", path, ""); w.Code != want {
			t.Errorf("delete %s: %d, want %d", path, w.Code, want)
		}
	}
	w := send(h, "POST", "/api/profiles/default/connect", "")
	var body map[string]string
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if w.Code != http.StatusOK || body["terminal"] != "connect:default" {
		t.Errorf("connect: %d %s", w.Code, w.Body)
	}
	if w := send(h, "POST", "/api/profiles/nope/connect", ""); w.Code != http.StatusNotFound {
		t.Errorf("connect an unknown profile: %d", w.Code)
	}
	if w := send(h, "DELETE", "/api/profiles/default/connect", ""); w.Code != http.StatusNoContent || len(f.ended) != 1 {
		t.Errorf("end the login: %d %v", w.Code, f.ended)
	}
	if w := send(h, "DELETE", "/api/profiles/nope/connect", ""); w.Code != http.StatusNotFound {
		t.Errorf("end an unknown profile's login: %d", w.Code)
	}
}

// pi answers for an endpoint harness, as cmd/foreman does.
func (f *fakeProfiles) pi(name string) error {
	if err := f.find(name); err != nil {
		return err
	}
	if f.harness(name) != "pi" {
		return ErrConnectKind
	}
	return nil
}

func (f *fakeProfiles) Endpoint(name string) (harness.Endpoint, error) {
	return f.endpoint, f.pi(name)
}

func (f *fakeProfiles) SetEndpoint(name string, c harness.EndpointChange) error {
	if err := f.pi(name); err != nil {
		return err
	}
	if err := c.Check(); err != nil {
		return err
	}
	f.endpoint = harness.Endpoint{URL: c.URL, Model: c.Model, ContextWindow: c.ContextWindow, MaxTokens: c.MaxTokens, APIKeySet: c.APIKey != ""}
	return nil
}

func (f *fakeProfiles) Models(_ context.Context, name, _, key string) ([]string, error) {
	if err := f.pi(name); err != nil {
		return nil, err
	}
	f.detectKey = key
	if f.detect == nil {
		return nil, errors.New("dial tcp 127.0.0.1:11434: connection refused")
	}
	return f.detect, nil
}

// A Pi profile's form: its server and model, never a login terminal; the
// key goes in, never out.
func TestProfileEndpoint(t *testing.T) {
	f := &fakeProfiles{list: []profiles.Profile{{Name: "default", Harness: "claude-code"}, {Name: "local", Harness: "pi"}}}
	h := New(Options{Guard: openGuard{}, Workspace: fakeWorkspace("/ws"), Profiles: f, UI: builtUI})
	good := `{"url":"http://localhost:11434/v1","apiKey":"sk-1","model":"qwen3:30b","contextWindow":262144,"maxTokens":131072}`
	if w := send(h, "PUT", "/api/profiles/local/endpoint", good); w.Code != http.StatusNoContent {
		t.Fatalf("set: %d %s", w.Code, w.Body)
	}
	w := get(t, h, "/api/profiles/local/endpoint")
	var e map[string]any
	decodeBody(t, w, &e)
	if e["model"] != "qwen3:30b" || e["apiKeySet"] != true || strings.Contains(w.Body.String(), "sk-1") {
		t.Errorf("endpoint %s", w.Body)
	}
	for _, c := range []struct {
		method, path, body string
		want               int
	}{
		{"PUT", "/api/profiles/local/endpoint", `{"url":"ftp://x","model":"m","contextWindow":2,"maxTokens":1}`, http.StatusBadRequest},
		{"PUT", "/api/profiles/local/endpoint", `nope`, http.StatusBadRequest},
		{"PUT", "/api/profiles/default/endpoint", good, http.StatusBadRequest},
		{"PUT", "/api/profiles/nope/endpoint", good, http.StatusNotFound},
		{"GET", "/api/profiles/default/endpoint", "", http.StatusBadRequest},
		{"POST", "/api/profiles/local/connect", "", http.StatusBadRequest},
		{"POST", "/api/profiles/local/endpoint/models", `{"url":"http://localhost:11434/v1"}`, http.StatusBadGateway},
		{"POST", "/api/profiles/local/endpoint/models", `{"url":"localhost:11434"}`, http.StatusBadRequest},
		{"POST", "/api/profiles/default/endpoint/models", `{"url":"http://localhost:11434/v1"}`, http.StatusBadRequest},
	} {
		if w := send(h, c.method, c.path, c.body); w.Code != c.want {
			t.Errorf("%s %s %s: %d %s, want %d", c.method, c.path, c.body, w.Code, w.Body, c.want)
		}
	}
	f.detect = []string{"qwen3:30b", "llama"}
	var ids []string
	decodeBody(t, send(h, "POST", "/api/profiles/local/endpoint/models", `{"url":"http://localhost:11434/v1","apiKey":"sk-2"}`), &ids)
	if len(ids) != 2 || f.detectKey != "sk-2" {
		t.Errorf("models %v, key %q", ids, f.detectKey)
	}
}
