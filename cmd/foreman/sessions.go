package main

import (
	"context"
	"errors"
	"sort"
	"sync"

	"github.com/brickpop/foreman/internal/runner"
	"github.com/brickpop/foreman/internal/server"
	"github.com/brickpop/foreman/internal/terminal"
)

// sessions starts terminals by instance id and serves them to the server.
// Scaffolding for --demo-session until the Instance manager exists
// (wiki/technical-debt.md).
type sessions struct {
	runner  runner.Runner
	openLog func(instance string) (terminal.Log, error)

	mu    sync.Mutex
	terms map[string]*terminal.Terminal
}

var errRunning = errors.New("instance already running")

func newSessions(r runner.Runner, openLog func(string) (terminal.Log, error)) *sessions {
	return &sessions{runner: r, openLog: openLog, terms: map[string]*terminal.Terminal{}}
}

// start runs spec as instance's terminal; an instance whose process exited
// can start again, with a new log.
func (s *sessions) start(ctx context.Context, instance string, spec runner.Spec) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if t, ok := s.terms[instance]; ok {
		if _, exited := t.ExitCode(); !exited {
			return errRunning
		}
	}
	log, err := s.openLog(instance)
	if err != nil {
		return err
	}
	process, err := s.runner.Start(ctx, spec)
	if err != nil {
		_ = log.Close()
		return err
	}
	s.terms[instance] = terminal.New(instance, process, log)
	return nil
}

// Attach implements server.Terminals.
func (s *sessions) Attach(instance string) (server.Terminal, error) {
	s.mu.Lock()
	t, ok := s.terms[instance]
	s.mu.Unlock()
	if !ok {
		return nil, server.ErrNoTerminal
	}
	v, err := t.Attach()
	if err != nil {
		return nil, err
	}
	return attachment{View: v, term: t}, nil
}

// List implements server.Terminals: every instance with a terminal, running
// or exited, sorted.
func (s *sessions) List() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	ids := make([]string, 0, len(s.terms))
	for id := range s.terms {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// attachment is what one browser connection needs: its view, plus the
// terminal's input and exit.
type attachment struct {
	*terminal.View
	term *terminal.Terminal
}

func (a attachment) Input(p []byte) error          { return a.term.Input(p) }
func (a attachment) Resize(size runner.Size) error { return a.term.Resize(size) }
func (a attachment) Exit() (code int, exited bool) { return a.term.ExitCode() }
