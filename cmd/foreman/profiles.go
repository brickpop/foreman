package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sort"

	"github.com/brickpop/foreman/internal/harness"
	"github.com/brickpop/foreman/internal/profiles"
	"github.com/brickpop/foreman/internal/runner"
	"github.com/brickpop/foreman/internal/server"
	"github.com/brickpop/foreman/internal/workspace"
)

// profileLogins is the CEO's profiles as Foreman uses them
// (wiki/product/architecture/session-runner.md#credentials): each a harness
// and its credentials, with the harness's login run in a container
// (Settings → Global → Profiles → Connect), and the profile each project's
// agents run on.
type profileLogins struct {
	store    profiles.Store
	adapters map[string]harness.Adapter
	images   runner.Images
	terms    *sessions // the login terminals, by id
	settings func() workspace.Settings
}

// Harnesses lists the harnesses Foreman supports.
func (p profileLogins) Harnesses() []harness.Status {
	list := []harness.Status{}
	for id, a := range p.adapters {
		list = append(list, harness.Status{ID: id, Command: a.Command(), Newline: a.Newline()})
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

// Of is the profile a project's agents run on, as the settings say now, and
// its folder.
func (p profileLogins) Of(project string) (profiles.Profile, string, error) {
	name := p.settings().ProfileOf(project)
	pr, err := p.store.Get(name)
	return pr, p.store.Dir(name), err
}

// Used are the profiles the projects use, by name (the default while there
// is no project), with their state; one that doesn't exist has no harness.
func (p profileLogins) Used(projects []workspace.Project) []profiles.Profile {
	names := map[string]bool{}
	for _, pr := range projects {
		names[p.settings().ProfileOf(pr.ID)] = true
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

// adoptLogins turns the workspace's own logins, from before profiles, into
// profiles, once: then nothing is left to move. Foreman starts whatever
// happens; a login that couldn't move is connected again.
func adoptLogins(store profiles.Store, root string, connected map[string]func(string) bool, stdout io.Writer) {
	adopted, err := store.Adopt(root, connected)
	if err != nil {
		slog.Warn("moving the workspace's logins into profiles", "err", err)
	}
	for _, pr := range adopted {
		fmt.Fprintf(stdout, "This workspace's %s login is now the profile %s, shared by your workspaces (%s)\n", pr.Harness, pr.Name, store.Dir(pr.Name))
	}
}
