package main

import (
	"context"
	"fmt"

	"github.com/brickpop/foreman/internal/runner"
)

// maintenance serves Settings → Maintenance from the engine and its images.
type maintenance struct {
	version func(context.Context) (string, error) // the engine's, nil for direct
	images  runner.Images
}

// maintenanceOf reads the engine's version when it has one (Podman does).
func maintenanceOf(r runner.Runner, images runner.Images) maintenance {
	m := maintenance{images: images}
	if v, ok := r.(interface {
		Version(context.Context) (string, error)
	}); ok {
		m.version = v.Version
	}
	return m
}

func (m maintenance) Status(ctx context.Context) runner.Maintenance {
	engine := "direct (no isolation)"
	if m.version != nil {
		if v, err := m.version(ctx); err == nil {
			engine = fmt.Sprintf("Podman %s · running", v)
		} else {
			engine = fmt.Sprintf("Podman not reachable: %v", err)
		}
	}
	return runner.Maintenance{Engine: engine, Images: m.images.Status(ctx)}
}

func (m maintenance) Rebuild(ctx context.Context) error { return m.images.Refresh(ctx) }
