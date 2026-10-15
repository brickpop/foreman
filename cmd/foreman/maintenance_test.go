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

func installed(context.Context) error { return nil }

func TestMaintenanceEngine(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		name    string
		running string
		runner  runner.Runner
		want    string
	}{
		{"direct has no version", "direct", direct.Runner{}, "direct (no isolation)"},
		{"podman running", "podman", versioned{}, "Podman 5.7.1 · running"},
		{"podman down", "podman", versioned{err: errors.New("no socket")}, "Podman not reachable: no socket"},
	} {
		s := maintenanceOf(tc.running, tc.runner, direct.Images{}, installed).Status(ctx)
		if s.Engine != tc.want || s.Running != tc.running || s.Podman != "" {
			t.Errorf("%s: %+v", tc.name, s)
		}
	}
}

// Podman's absence is told even while running directly, so Settings can say
// why Containers can't be chosen.
func TestMaintenanceTellsWhyPodmanCantBeChosen(t *testing.T) {
	missing := func(context.Context) error { return errors.New("podman is not installed") }
	s := maintenanceOf("direct", direct.Runner{}, direct.Images{}, missing).Status(context.Background())
	if s.Podman != "podman is not installed" {
		t.Errorf("podman %q", s.Podman)
	}
}
