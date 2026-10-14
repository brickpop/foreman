package main

import (
	"context"
	"errors"

	"github.com/brickpop/foreman/internal/instances"
	"github.com/brickpop/foreman/internal/server"
	"github.com/brickpop/foreman/internal/store"
)

// agents serves the instance manager to the server, and the terminals that
// aren't instances: the harnesses' login terminals, and the --demo-session
// one (scaffolding until the UI can deploy a team: wiki/technical-debt.md).
type agents struct {
	*instances.Manager
	demo   *sessions
	logins *sessions
}

// Attach implements server.Terminals: an instance's terminal, else a login's
// or the demo's.
func (a agents) Attach(id string) (server.Terminal, error) {
	term, err := a.Terminal(id)
	switch {
	case errors.Is(err, instances.ErrNotRunning):
		if t, err := a.logins.Attach(id); !errors.Is(err, server.ErrNoTerminal) {
			return t, err
		}
		return a.demo.Attach(id)
	case err != nil:
		return nil, err
	}
	v, err := term.Attach()
	if err != nil {
		return nil, err
	}
	return attachment{View: v, term: term}, nil
}

// List implements server.Agents: the instances, and the demo session if any.
func (a agents) List(ctx context.Context) ([]store.Instance, error) {
	list, err := a.Manager.List(ctx)
	if err != nil {
		return nil, err
	}
	for _, id := range a.demo.List() {
		list = append(list, store.Instance{ID: id, Status: instances.Running})
	}
	return list, nil
}
