package main

import (
	"context"
	"errors"
	"sort"

	"github.com/brickpop/foreman/internal/harness"
	"github.com/brickpop/foreman/internal/runner"
)

// logins runs the harnesses' login containers: Settings → Harnesses →
// Connect (wiki/product/architecture/session-runner.md#credentials).
type logins struct {
	adapters map[string]harness.Adapter
	images   runner.Images
	terms    *sessions // the login terminals, by id
}

var errUnknownHarness = errors.New("no such harness")

// Harnesses lists the harnesses Foreman supports, with their login state.
func (l logins) Harnesses() []harness.Status {
	list := []harness.Status{}
	for id, a := range l.adapters {
		list = append(list, harness.Status{ID: id, Connected: a.Connected()})
	}
	sort.Slice(list, func(i, j int) bool { return list[i].ID < list[j].ID })
	return list
}

// Connect starts the harness's login in a terminal, connect:<harness>, and
// returns its id; a login already running is reused.
func (l logins) Connect(ctx context.Context, id string) (string, error) {
	a := l.adapters[id]
	if a == nil {
		return "", errUnknownHarness
	}
	login, err := a.Login()
	if err != nil {
		return "", err
	}
	image, err := l.images.Ensure(ctx, "")
	if err != nil {
		return "", err
	}
	term := "connect:" + id
	err = l.terms.start(ctx, term, runner.Spec{
		Name: term, Image: image, Command: login.Command, Env: login.Env, Mounts: login.Mounts,
		Labels: map[string]string{"foreman.login": id},
	})
	if errors.Is(err, errRunning) {
		return term, nil
	}
	return term, err
}
