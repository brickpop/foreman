package main

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/brickpop/foreman/internal/harness"
	"github.com/brickpop/foreman/internal/home"
	"github.com/brickpop/foreman/internal/profiles"
	"github.com/brickpop/foreman/internal/runner"
	"github.com/brickpop/foreman/internal/server"
)

// profileLogins is the CEO's profiles as Foreman uses them
// (wiki/product/architecture/session-runner.md#credentials): each a harness
// and its credentials, with the harness's login run in a container
// (Settings → Agents → Profiles → Connect), and the profile each project's
// agents run on.
type profileLogins struct {
	store    profiles.Store
	adapters map[string]harness.Adapter
	images   runner.Images
	terms    *sessions // the login terminals, by id
	project  func(id string) (home.Project, error)
}

// Harnesses lists the harnesses Foreman supports.
func (p profileLogins) Harnesses() []harness.Status {
	list := []harness.Status{}
	for id, a := range p.adapters {
		connect := "login"
		if _, ok := a.(harness.EndpointAdapter); ok {
			connect = "endpoint"
		}
		list = append(list, harness.Status{ID: id, Command: a.Command(), Newline: a.Newline(), Connect: connect})
	}
	sort.Slice(list, func(i, j int) bool { return list[i].ID < list[j].ID })
	return list
}

// get is a profile with its state: whether its harness finds a login in it.
func (p profileLogins) get(name string) (profiles.Profile, error) {
	pr, err := p.store.Get(name)
	if err != nil {
		return pr, err
	}
	if a := p.adapters[pr.Harness]; a != nil {
		pr.Connected = a.Connected(p.store.Dir(name))
	}
	return pr, nil
}

// List returns the profiles with their state, the default first.
func (p profileLogins) List() ([]profiles.Profile, error) {
	list, err := p.store.List()
	if err != nil {
		return nil, err
	}
	for i := range list {
		if list[i], err = p.get(list[i].Name); err != nil {
			return nil, err
		}
	}
	return list, nil
}

// Create makes a profile for a supported harness.
func (p profileLogins) Create(name, h string) (profiles.Profile, error) {
	if p.adapters[h] == nil {
		return profiles.Profile{}, fmt.Errorf("%w: %q", server.ErrUnknownHarness, h)
	}
	return p.store.Create(name, h)
}

// Delete removes a profile, ending its login if one runs.
func (p profileLogins) Delete(name string) error {
	if err := p.store.Delete(name); err != nil {
		return err
	}
	_ = p.terms.stop("connect:" + name)
	return nil
}

// SetHarness switches a profile to a supported harness. A login still
// running is the old harness's: it ends.
func (p profileLogins) SetHarness(name, h string) (profiles.Profile, error) {
	if p.adapters[h] == nil {
		return profiles.Profile{}, fmt.Errorf("%w: %q", server.ErrUnknownHarness, h)
	}
	if _, err := p.store.SetHarness(name, h); err != nil {
		return profiles.Profile{}, err
	}
	_ = p.terms.stop("connect:" + name)
	return p.get(name)
}

// Connect starts the profile's harness login in a terminal, connect:<profile>,
// and returns its id; a login already running is reused.
func (p profileLogins) Connect(ctx context.Context, name string) (string, error) {
	pr, err := p.store.Get(name)
	if err != nil {
		return "", err
	}
	a := p.adapters[pr.Harness]
	if a == nil {
		return "", fmt.Errorf("%w: %q", server.ErrUnknownHarness, pr.Harness)
	}
	if _, ok := a.(harness.EndpointAdapter); ok {
		return "", fmt.Errorf("%w: %s is set in a form, not a login", server.ErrConnectKind, pr.Harness)
	}
	login, err := a.Login(p.store.Dir(name))
	if err != nil {
		return "", err
	}
	image, err := p.images.Ensure(ctx, "")
	if err != nil {
		return "", err
	}
	term := "connect:" + name
	err = p.terms.start(ctx, term, runner.Spec{
		Name: term, Image: image, Command: login.Command, Env: login.Env, Mounts: login.Mounts,
		Labels: map[string]string{"foreman.login": name},
	})
	if errors.Is(err, errRunning) {
		return term, nil
	}
	return term, err
}

// EndLogin ends the profile's login, if one runs: the CEO closed its dialog,
// logged in or not.
func (p profileLogins) EndLogin(name string) error {
	if _, err := p.store.Get(name); err != nil {
		return err
	}
	return p.terms.stop("connect:" + name)
}

// Of is the profile a project's agents run on, as its record says now, and
// its folder.
func (p profileLogins) Of(project string) (profiles.Profile, string, error) {
	proj, err := p.project(project)
	if err != nil {
		return profiles.Profile{}, "", err
	}
	pr, err := p.store.Get(proj.Profile)
	return pr, p.store.Dir(proj.Profile), err
}

// Used are the profiles the projects use, by name (the default while there
// is no project), with their state; one that doesn't exist has no harness.
func (p profileLogins) Used(projects []home.Project) []profiles.Profile {
	names := map[string]bool{}
	for _, pr := range projects {
		names[pr.Profile] = true
	}
	if len(projects) == 0 {
		names[profiles.Default] = true
	}
	var used []profiles.Profile
	for name := range names {
		pr, err := p.get(name)
		if err != nil {
			pr = profiles.Profile{Name: name}
		}
		used = append(used, pr)
	}
	sort.Slice(used, func(i, j int) bool { return used[i].Name < used[j].Name })
	return used
}

// endpointOf is the profile's adapter when its harness runs on a model
// server (Pi), and the profile's folder.
func (p profileLogins) endpointOf(name string) (harness.EndpointAdapter, string, error) {
	pr, err := p.store.Get(name)
	if err != nil {
		return nil, "", err
	}
	a, ok := p.adapters[pr.Harness].(harness.EndpointAdapter)
	if !ok {
		return nil, "", fmt.Errorf("%w: %s runs on a login, not a model server", server.ErrConnectKind, pr.Harness)
	}
	return a, p.store.Dir(name), nil
}

// Endpoint is the profile's model server and model.
func (p profileLogins) Endpoint(name string) (harness.Endpoint, error) {
	a, dir, err := p.endpointOf(name)
	if err != nil {
		return harness.Endpoint{}, err
	}
	return a.Endpoint(dir)
}

// SetEndpoint saves the profile's model server and model.
func (p profileLogins) SetEndpoint(name string, c harness.EndpointChange) error {
	a, dir, err := p.endpointOf(name)
	if err != nil {
		return err
	}
	return a.SetEndpoint(dir, c)
}

// Models lists the models a server offers, for the profile's form.
func (p profileLogins) Models(ctx context.Context, name, url, key string) ([]string, error) {
	a, dir, err := p.endpointOf(name)
	if err != nil {
		return nil, err
	}
	return a.Detect(ctx, dir, url, key)
}
