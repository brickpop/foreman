package main

import (
	"context"
	"fmt"

	"github.com/brickpop/foreman/internal/runner"
)

// maintenance serves Settings → Maintenance from the engine and its images.
type maintenance struct {
	running   string                                // "podman" or "direct", chosen at startup
	version   func(context.Context) (string, error) // the engine's, nil for direct
	installed func(context.Context) error           // whether Podman could be chosen
	images    runner.Images
}

// maintenanceOf reads the engine's version when it has one (Podman does).
func maintenanceOf(running string, r runner.Runner, images runner.Images, installed func(context.Context) error) maintenance {
	m := maintenance{running: running, images: images, installed: installed}
	if v, ok := r.(interface {
		Version(context.Context) (string, error)
	}); ok {
		m.version = v.Version
	}
	return m
}

func (m maintenance) Status(ctx context.Context) runner.Maintenance {
	s := runner.Maintenance{Running: m.running, Engine: "direct (no isolation)", Images: m.images.Status(ctx)}
	if m.version != nil {
		if v, err := m.version(ctx); err == nil {
			s.Engine = fmt.Sprintf("Podman %s · running", v)
		} else {
			s.Engine = fmt.Sprintf("Podman not reachable: %v", err)
		}
	}
	if err := m.installed(ctx); err != nil {
		s.Podman = err.Error()
	}
	return s
}

func (m maintenance) Rebuild(ctx context.Context) error { return m.images.Refresh(ctx) }
