package main

import (
	"context"
	"errors"
	"testing"

	"github.com/brickpop/foreman/internal/runner"
	"github.com/brickpop/foreman/internal/runner/direct"
)

type versioned struct {
	direct.Runner
	err error
}

func (v versioned) Version(context.Context) (string, error) { return "5.7.1", v.err }

func TestMaintenanceEngine(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		name   string
		runner runner.Runner
		want   string
	}{
		{"direct has no version", direct.Runner{}, "direct (no isolation)"},
		{"podman running", versioned{}, "Podman 5.7.1 · running"},
		{"podman down", versioned{err: errors.New("no socket")}, "Podman not reachable: no socket"},
	} {
		if got := maintenanceOf(tc.runner, direct.Images{}).Status(ctx).Engine; got != tc.want {
			t.Errorf("%s: %q, want %q", tc.name, got, tc.want)
		}
	}
}
