package instances

import (
	"context"
	"log/slog"
	"time"

	"github.com/brickpop/foreman/internal/store"
)

// ProjectsChanged looks for active deployments whose project is gone
// (home.Project.Gone: its HQ deleted, or every code repo missing)
// and gives each such project Recheck to come back (a move or a git
// operation in progress); still gone then, its deployments stop, with the
// reason (wiki/product/models/deployment.md#lifecycle). Called when the
// projects change, and once the instances are restored.
func (m *Manager) ProjectsChanged(ctx context.Context) {
	deps, err := m.DeploymentsIn(ctx, store.DeploymentActive)
	if err != nil {
		slog.Warn("looking for gone projects", "err", err)
		return
	}
	recheck := m.Recheck
	if recheck == 0 {
		recheck = time.Minute
	}
	for _, d := range deps {
		p, err := m.Config.Project(d.ProjectID)
		if err != nil || p.Gone() == "" {
			continue
		}
		m.mu.Lock()
		pending := m.gone[p.ID]
		m.gone[p.ID] = true
		m.mu.Unlock()
		if !pending {
			time.AfterFunc(recheck, func() { m.stopIfGone(p.ID) })
		}
	}
}

// stopIfGone stops the project's active deployments if it is still gone.
func (m *Manager) stopIfGone(project string) {
	ctx := context.Background()
	m.mu.Lock()
	delete(m.gone, project)
	closing := m.closing
	m.mu.Unlock()
	if closing {
		return
	}
	p, err := m.Config.Checked(project)
	if err != nil {
		return
	}
	reason := p.Gone()
	if reason == "" {
		return
	}
	deps, err := m.DeploymentsIn(ctx, store.DeploymentActive)
	if err != nil {
		slog.Warn("stopping a gone project's deployments", "project", project, "err", err)
		return
	}
	for _, d := range deps {
		if d.ProjectID != project {
			continue
		}
		if err := m.stop(ctx, d.ID, project+" is gone: "+reason); err != nil {
			slog.Warn("stopping a gone project's deployment", "deployment", d.ID, "err", err)
		}
	}
}
