package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/brickpop/foreman/internal/harness"
	"github.com/brickpop/foreman/internal/harness/fake"
	"github.com/brickpop/foreman/internal/instances"
	"github.com/brickpop/foreman/internal/runner/direct"
	"github.com/brickpop/foreman/internal/server"
	"github.com/brickpop/foreman/internal/sessionlog"
	"github.com/brickpop/foreman/internal/terminal"
)

func testLogins(t *testing.T) logins {
	t.Helper()
	program := filepath.Join(t.TempDir(), "login.sh")
	if err := os.WriteFile(program, []byte("#!/bin/sh\ncat\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	logs := sessionlog.Dir{Root: t.TempDir()}
	return logins{
		adapters: map[string]harness.Adapter{"fake": fake.Adapter{Program: program}},
		images:   direct.Images{},
		terms:    newSessions(direct.Runner{}, func(id string) (terminal.Log, error) { return logs.Open(id) }),
	}
}

func TestConnectStartsTheLoginTerminalOnce(t *testing.T) {
	l := testLogins(t)
	ctx := context.Background()
	if got := l.Harnesses(); len(got) != 1 || got[0] != (harness.Status{ID: "fake", Command: l.adapters["fake"].Command(), Newline: "\\\r", Connected: true}) {
		t.Errorf("harnesses %+v", got)
	}
	id, err := l.Connect(ctx, "fake")
	if err != nil || id != "connect:fake" {
		t.Fatalf("connect: %q %v", id, err)
	}
	a, err := l.terms.Attach(id)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	// A second Connect while it runs reuses it.
	if again, err := l.Connect(ctx, "fake"); err != nil || again != id {
		t.Errorf("second connect: %q %v", again, err)
	}
	if _, err := l.Connect(ctx, "nope"); !errors.Is(err, errUnknownHarness) {
		t.Errorf("unknown harness: %v", err)
	}
	_ = a.Input([]byte("\x04")) // end the login (cat ends on EOF)
}

// Closing the dialog ends a running login; ending none is harmless, and the
// next Connect starts a fresh one.
func TestEndLogin(t *testing.T) {
	l := testLogins(t)
	ctx := context.Background()
	if err := l.EndLogin("fake"); err != nil {
		t.Errorf("no login running: %v", err)
	}
	id, err := l.Connect(ctx, "fake")
	if err != nil {
		t.Fatal(err)
	}
	if err := l.EndLogin("fake"); err != nil {
		t.Fatal(err)
	}
	waitExit(t, l.terms, id)
	if err := l.EndLogin("fake"); err != nil {
		t.Errorf("already ended: %v", err)
	}
	if _, err := l.Connect(ctx, "fake"); err != nil {
		t.Errorf("connect after ending: %v", err)
	}
	_ = l.EndLogin("fake")
	if err := l.EndLogin("nope"); !errors.Is(err, errUnknownHarness) {
		t.Errorf("unknown harness: %v", err)
	}
}

// A terminal id is looked for among the instances, then the logins.
func TestTerminalsAreFoundWhereverTheyRun(t *testing.T) {
	l := testLogins(t)
	ctx := context.Background()
	if _, err := l.Connect(ctx, "fake"); err != nil {
		t.Fatal(err)
	}
	views := agents{Manager: instances.New(instances.Deps{}), logins: l.terms}
	a, err := views.Attach("connect:fake")
	if err != nil {
		t.Fatal(err)
	}
	_ = a.Input([]byte("\x04"))
	a.Close()
	if _, err := views.Attach("nobody@ledger"); !errors.Is(err, server.ErrNoTerminal) {
		t.Errorf("unknown terminal: %v", err)
	}
}
