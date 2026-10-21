package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/brickpop/foreman/internal/harness"
	"github.com/brickpop/foreman/internal/harness/fake"
	"github.com/brickpop/foreman/internal/harness/pi"
	"github.com/brickpop/foreman/internal/home"
	"github.com/brickpop/foreman/internal/instances"
	"github.com/brickpop/foreman/internal/profiles"
	"github.com/brickpop/foreman/internal/runner/direct"
	"github.com/brickpop/foreman/internal/server"
	"github.com/brickpop/foreman/internal/sessionlog"
	"github.com/brickpop/foreman/internal/terminal"
)

// loginFile is the fake harness, connected when its folder holds a file
// named login.
type loginFile struct{ fake.Adapter }

func (loginFile) Connected(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, "login"))
	return err == nil
}

// testProfiles has the default profile, on the fake harness ("fake"), whose
// login is cat, and the projects given.
func testProfiles(t *testing.T, projects ...home.Project) profileLogins {
	t.Helper()
	program := filepath.Join(t.TempDir(), "login.sh")
	if err := fake.WriteProgram(program, "cat"); err != nil {
		t.Fatal(err)
	}
	logs := sessionlog.Dir{Root: t.TempDir()}
	p := profileLogins{
		store:    profiles.Store{Root: t.TempDir()},
		adapters: map[string]harness.Adapter{"fake": loginFile{fake.Adapter{Program: program}}},
		images:   direct.Images{},
		terms:    newSessions(direct.Runner{}, func(id string) (terminal.Log, error) { return logs.Open(id) }),
		project: func(id string) (home.Project, error) {
			for _, p := range projects {
				if p.ID == id {
					return p, nil
				}
			}
			return home.Project{}, home.ErrNotFound
		},
	}
	if err := p.store.Ensure("fake"); err != nil {
		t.Fatal(err)
	}
	return p
}

func login(t *testing.T, p profileLogins, name string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(p.store.Dir(name), "login"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
}

// The list says which profiles have a login; only supported harnesses get one.
func TestProfilesAndTheirState(t *testing.T) {
	p := testProfiles(t)
	if _, err := p.Create("work", "fake"); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Create("pi", "pi"); !errors.Is(err, server.ErrUnknownHarness) {
		t.Errorf("an unsupported harness: %v", err)
	}
	login(t, p, "work")
	list, err := p.List()
	if err != nil {
		t.Fatal(err)
	}
	if want := []profiles.Profile{{Name: "default", Harness: "fake"}, {Name: "work", Harness: "fake", Connected: true}}; !slices.Equal(list, want) {
		t.Errorf("list %+v", list)
	}
	if got := p.Harnesses(); len(got) != 1 || got[0].ID != "fake" || got[0].Newline != "\\\r" {
		t.Errorf("harnesses %+v", got)
	}
}

// A project runs on its profile (the default unless set); the checklist asks
// for the profiles the projects use, one that doesn't exist included.
func TestTheProfileAProjectRunsOn(t *testing.T) {
	projects := []home.Project{{ID: "ledger", Profile: "work"}, {ID: "relay", Profile: "gone"}, {ID: "web", Profile: "default"}}
	p := testProfiles(t, projects...)
	if _, err := p.Create("work", "fake"); err != nil {
		t.Fatal(err)
	}
	if pr, dir, err := p.Of("ledger"); err != nil || pr.Name != "work" || dir != p.store.Dir("work") {
		t.Errorf("ledger: %+v %s %v", pr, dir, err)
	}
	if pr, _, err := p.Of("web"); err != nil || pr.Name != "default" {
		t.Errorf("web: %+v %v", pr, err)
	}
	if _, _, err := p.Of("relay"); !errors.Is(err, profiles.ErrNotFound) {
		t.Errorf("relay: %v", err)
	}
	if _, _, err := p.Of("nowhere"); !errors.Is(err, home.ErrNotFound) {
		t.Errorf("no such project: %v", err)
	}
	used := p.Used(projects)
	if want := []profiles.Profile{{Name: "default", Harness: "fake"}, {Name: "gone"}, {Name: "work", Harness: "fake"}}; !slices.Equal(used, want) {
		t.Errorf("used %+v", used)
	}
	if used := p.Used(nil); len(used) != 1 || used[0].Name != "default" {
		t.Errorf("no project: %+v", used)
	}
}

func TestConnectStartsTheLoginTerminalOnce(t *testing.T) {
	p := testProfiles(t)
	ctx := context.Background()
	id, err := p.Connect(ctx, "default")
	if err != nil || id != "connect:default" {
		t.Fatalf("connect: %q %v", id, err)
	}
	a, err := p.terms.Attach(id)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	// A second Connect while it runs reuses it.
	if again, err := p.Connect(ctx, "default"); err != nil || again != id {
		t.Errorf("second connect: %q %v", again, err)
	}
	if _, err := p.Connect(ctx, "nope"); !errors.Is(err, profiles.ErrNotFound) {
		t.Errorf("unknown profile: %v", err)
	}
	_ = a.Input([]byte("\x04")) // end the login (cat ends on EOF)
}

// Closing the dialog ends a running login; ending none is harmless, and the
// next Connect starts a fresh one. Deleting a profile ends its login too.
func TestEndLogin(t *testing.T) {
	p := testProfiles(t)
	ctx := context.Background()
	if err := p.EndLogin("default"); err != nil {
		t.Errorf("no login running: %v", err)
	}
	id, err := p.Connect(ctx, "default")
	if err != nil {
		t.Fatal(err)
	}
	if err := p.EndLogin("default"); err != nil {
		t.Fatal(err)
	}
	waitExit(t, p.terms, id)
	if err := p.EndLogin("default"); err != nil {
		t.Errorf("already ended: %v", err)
	}
	if err := p.EndLogin("nope"); !errors.Is(err, profiles.ErrNotFound) {
		t.Errorf("unknown profile: %v", err)
	}
	if _, err := p.Create("work", "fake"); err != nil {
		t.Fatal(err)
	}
	id, err = p.Connect(ctx, "work")
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Delete("work"); err != nil {
		t.Fatal(err)
	}
	waitExit(t, p.terms, id)
}

// A terminal id is looked for among the instances, then the logins.
func TestTerminalsAreFoundWhereverTheyRun(t *testing.T) {
	p := testProfiles(t)
	ctx := context.Background()
	if _, err := p.Connect(ctx, "default"); err != nil {
		t.Fatal(err)
	}
	var typed []string
	views := agents{Manager: instances.New(instances.Deps{}), logins: p.terms,
		typed: func(id string, b []byte) { typed = append(typed, id+": "+string(b)) }}
	a, err := views.Attach("connect:default")
	if err != nil {
		t.Fatal(err)
	}
	// What the CEO types into an agent's terminal is heard (drafts); a login's isn't.
	agent := agentAttachment{attachment: a.(attachment), id: "mara@ledger", typed: views.typed}
	if err := agent.Input([]byte("hi")); err != nil {
		t.Fatal(err)
	}
	_ = a.Input([]byte("\x04"))
	if len(typed) != 1 || typed[0] != "mara@ledger: hi" {
		t.Errorf("typed %q", typed)
	}
	a.Close()
	if _, err := views.Attach("nobody@ledger"); !errors.Is(err, server.ErrNoTerminal) {
		t.Errorf("unknown terminal: %v", err)
	}
}

// A Pi profile is a model server set in a form: no login terminal, connected
// once its server and model are saved; a login harness has no form.
func TestAPiProfile(t *testing.T) {
	p := testProfiles(t)
	p.adapters["pi"] = pi.Adapter{}
	if _, err := p.Create("local", "pi"); err != nil {
		t.Fatal(err)
	}
	kinds := map[string]string{}
	for _, h := range p.Harnesses() {
		kinds[h.ID] = h.Connect
	}
	if kinds["fake"] != "login" || kinds["pi"] != "endpoint" {
		t.Errorf("connect kinds %v", kinds)
	}
	if _, err := p.Connect(context.Background(), "local"); !errors.Is(err, server.ErrConnectKind) {
		t.Errorf("a Pi login terminal: %v", err)
	}
	for _, err := range []error{
		p.SetEndpoint("default", harness.EndpointChange{URL: "http://h/v1", Model: "m", ContextWindow: 2, MaxTokens: 1}),
		func() error { _, err := p.Endpoint("default"); return err }(),
		func() error { _, err := p.Models(context.Background(), "default", "http://h/v1", ""); return err }(),
	} {
		if !errors.Is(err, server.ErrConnectKind) {
			t.Errorf("a form for a login harness: %v", err)
		}
	}
	if _, err := p.Endpoint("nope"); !errors.Is(err, profiles.ErrNotFound) {
		t.Errorf("unknown profile: %v", err)
	}
	if pr, _ := p.get("local"); pr.Connected {
		t.Error("connected before its form is saved")
	}
	if err := p.SetEndpoint("local", harness.EndpointChange{URL: "http://localhost:11434/v1", Model: "qwen3", ContextWindow: 262144, MaxTokens: 131072}); err != nil {
		t.Fatal(err)
	}
	if e, err := p.Endpoint("local"); err != nil || e.Model != "qwen3" {
		t.Errorf("endpoint %+v %v", e, err)
	}
	if pr, _ := p.get("local"); !pr.Connected {
		t.Error("not connected once its form is saved")
	}
	if _, err := os.Stat(filepath.Join(p.store.Dir("local"), "models.json")); err != nil {
		t.Errorf("the profile folder holds no models.json: %v", err)
	}
}
