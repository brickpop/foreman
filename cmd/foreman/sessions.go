package main

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/brickpop/foreman/internal/runner"
	"github.com/brickpop/foreman/internal/server"
	"github.com/brickpop/foreman/internal/terminal"
)

// sessions starts terminals by id and serves them to the server: the
// harnesses' login terminals (profiles.go) and the Chief of Staff's (cos.go).
// Instances' terminals are the Instance manager's.
type sessions struct {
	runner  runner.Runner
	openLog func(instance string) (terminal.Log, error)

	mu    sync.Mutex
	terms map[string]*terminal.Terminal
	procs map[string]runner.Process // each terminal's process, to stop it
}

var errRunning = errors.New("instance already running")

func newSessions(r runner.Runner, openLog func(string) (terminal.Log, error)) *sessions {
	return &sessions{runner: r, openLog: openLog, terms: map[string]*terminal.Terminal{}, procs: map[string]runner.Process{}}
}

// start runs spec as the terminal id; one whose process exited can start
// again, with a new log.
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
	s.procs[instance] = process
	return nil
}

// running reports whether the terminal's process runs.
func (s *sessions) running(id string) bool {
	s.mu.Lock()
	t, ok := s.terms[id]
	s.mu.Unlock()
	if !ok {
		return false
	}
	_, exited := t.ExitCode()
	return !exited
}

// input types p into the terminal, as if typed.
func (s *sessions) input(id string, p []byte) error {
	s.mu.Lock()
	t, ok := s.terms[id]
	s.mu.Unlock()
	if !ok {
		return server.ErrNoTerminal
	}
	return t.Input(p)
}

// stop ends the terminal's process, if it runs; its terminal stays, exited.
func (s *sessions) stop(id string) error {
	s.mu.Lock()
	t, p := s.terms[id], s.procs[id]
	s.mu.Unlock()
	if t == nil {
		return nil
	}
	if _, exited := t.ExitCode(); exited {
		return nil
	}
	return p.Stop(2 * time.Second)
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

// attachment is what one browser connection needs: its view, plus the
// terminal's input and exit.
type attachment struct {
	*terminal.View
	term *terminal.Terminal
}

func (a attachment) Input(p []byte) error          { return a.term.Input(p) }
func (a attachment) Resize(size runner.Size) error { return a.term.Resize(size) }
func (a attachment) Exit() (code int, exited bool) { return a.term.ExitCode() }

// stopAll ends every terminal's process that still runs (Foreman exiting).
func (s *sessions) stopAll() {
	s.mu.Lock()
	ids := make([]string, 0, len(s.terms))
	for id := range s.terms {
		ids = append(ids, id)
	}
	s.mu.Unlock()
	for _, id := range ids {
		_ = s.stop(id)
	}
}
