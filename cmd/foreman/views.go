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
	// typed hears what the CEO types into an agent's terminal (Messaging
	// holds deliveries while it is an unsent draft).
	typed func(instance string, p []byte)
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
	return agentAttachment{attachment: attachment{View: v, term: term}, id: id, typed: a.typed}, nil
}

// agentAttachment is an agent's terminal as the CEO types into it.
type agentAttachment struct {
	attachment
	id    string
	typed func(instance string, p []byte)
}

func (a agentAttachment) Input(p []byte) error {
	if err := a.attachment.Input(p); err != nil {
		return err
	}
	if a.typed != nil {
		a.typed(a.id, p)
	}
	return nil
}
