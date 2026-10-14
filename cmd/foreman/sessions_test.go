package main

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/brickpop/foreman/internal/runner"
	"github.com/brickpop/foreman/internal/runner/direct"
	"github.com/brickpop/foreman/internal/server"
	"github.com/brickpop/foreman/internal/sessionlog"
	"github.com/brickpop/foreman/internal/terminal"
)

// newTestSessions runs real processes (the Direct engine) and logs them in a
// temporary folder; opened counts the logs opened.
func newTestSessions(t *testing.T, r runner.Runner) (s *sessions, opened *int) {
	t.Helper()
	logs := sessionlog.Dir{Root: t.TempDir()}
	opened = new(int)
	return newSessions(r, func(instance string) (terminal.Log, error) {
		*opened++
		return logs.Open(instance)
	}), opened
}

func sh(script string) runner.Spec { return runner.Spec{Command: []string{"/bin/sh", "-c", script}} }

func waitExit(t *testing.T, s *sessions, instance string) {
	t.Helper()
	s.mu.Lock()
	term := s.terms[instance]
	s.mu.Unlock()
	select {
	case <-term.Done():
	case <-time.After(5 * time.Second):
		t.Fatalf("%s never exited", instance)
	}
}

func TestSessionsStartRules(t *testing.T) {
	s, opened := newTestSessions(t, direct.Runner{})
	ctx := context.Background()
	if _, err := s.Attach("nobody"); !errors.Is(err, server.ErrNoTerminal) {
		t.Errorf("attach unknown: %v", err)
	}
	if err := s.start(ctx, "x", sh("read line")); err != nil {
		t.Fatal(err)
	}
	if err := s.start(ctx, "x", sh("true")); !errors.Is(err, errRunning) {
		t.Errorf("second start while running: %v", err)
	}
	a, err := s.Attach("x")
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	if err := a.Input([]byte("go\r")); err != nil {
		t.Fatal(err)
	}
	waitExit(t, s, "x")
	if code, exited := a.Exit(); !exited || code != 0 {
		t.Errorf("Exit = %d, %v", code, exited)
	}
	// After an exit the instance can run again, in a new terminal with a new log.
	if err := s.start(ctx, "x", sh("true")); err != nil {
		t.Fatalf("restart after exit: %v", err)
	}
	if *opened != 2 {
		t.Errorf("opened %d logs, want 2", *opened)
	}
}

func TestSessionsFailedStartLeavesNothingBehind(t *testing.T) {
	s, _ := newTestSessions(t, direct.Runner{})
	if err := s.start(context.Background(), "x", runner.Spec{}); err == nil {
		t.Fatal("start of an empty command succeeded")
	}
	if _, err := s.Attach("x"); !errors.Is(err, server.ErrNoTerminal) {
		t.Errorf("a failed start left an instance behind: %v", err)
	}
	noLog := newSessions(direct.Runner{}, func(string) (terminal.Log, error) { return nil, errors.New("disk full") })
	if err := noLog.start(context.Background(), "x", sh("true")); err == nil {
		t.Error("start succeeded without a log")
	}
}

func TestSessionsListIsSortedAndKeepsExited(t *testing.T) {
	s, _ := newTestSessions(t, direct.Runner{})
	if got := s.List(); len(got) != 0 {
		t.Errorf("empty List = %v", got)
	}
	for _, id := range []string{"tove@ledger", "mara@ledger"} {
		if err := s.start(context.Background(), id, sh("true")); err != nil {
			t.Fatal(err)
		}
		waitExit(t, s, id)
	}
	if got := strings.Join(s.List(), ","); got != "mara@ledger,tove@ledger" {
		t.Errorf("List = %s", got)
	}
}
