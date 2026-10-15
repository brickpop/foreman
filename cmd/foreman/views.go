package main

import (
	"errors"

	"github.com/brickpop/foreman/internal/instances"
	"github.com/brickpop/foreman/internal/server"
)

// agents serves the instance manager to the server, and the terminals that
// aren't instances: the harnesses' login terminals.
type agents struct {
	*instances.Manager
	logins *sessions
}

// Attach implements server.Terminals: an instance's terminal, else a login's.
func (a agents) Attach(id string) (server.Terminal, error) {
	term, err := a.Terminal(id)
	switch {
	case errors.Is(err, instances.ErrNotRunning):
		return a.logins.Attach(id)
	case err != nil:
		return nil, err
	}
	v, err := term.Attach()
	if err != nil {
		return nil, err
	}
	return attachment{View: v, term: term}, nil
}
