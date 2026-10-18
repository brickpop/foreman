package server

import (
	"context"
	"encoding/json"
	"net/http"
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
}

func (f *fakeProfiles) List() ([]profiles.Profile, error) { return f.list, nil }

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
