package instances

import (
	"context"
	"log/slog"

	"github.com/brickpop/foreman/internal/runner"
	"github.com/brickpop/foreman/internal/store"
)

// Restore picks up after a Foreman restart: this home's running containers
// are re-attached, live instances whose process is gone restart (continuing
// their conversation), and this home's processes that belong to nothing are
// stopped. Stuck cards from before are resolved: the watch starts over, and
// raises them again if the agent is still quiet. A card for an agent still
// waiting on the CEO in its re-attached session stays, the others resolve. Another home's (a test's, just dev's) are left alone.
func (m *Manager) Restore(ctx context.Context) error {
	m.resolveOldStuck(ctx)
	procs, err := m.Runner.List(ctx)
	if err != nil {
		return err
	}
	byID := map[string]runner.Process{} // this home's
	for _, p := range procs {
		if p.Labels()[HomeLabel] == m.Home {
			byID[p.Labels()["foreman.instance"]] = p
		}
	}
	deps, err := m.Deployments.List(ctx)
	if err != nil {
		return err
	}
	for _, dep := range deps {
		if dep.Status != store.DeploymentActive {
			continue
		}
		insts, err := m.Instances.List(ctx, dep.ID)
		if err != nil {
			return err
		}
		for _, inst := range insts {
			if inst.Status == Stopped || inst.Status == Asleep {
				continue
			}
			if p, ok := byID[inst.ID]; ok {
				delete(byID, inst.ID)
				if err := m.adopt(ctx, inst.ID, p); err != nil {
					slog.Warn("re-attaching", "instance", inst.ID, "err", err)
				}
				continue
			}
			m.enqueue(inst.ID)
		}
	}
	for id, p := range byID {
		slog.Warn("stopping an orphan process: no live instance", "instance", id)
		go func() { _ = p.Stop(stopGrace) }()
	}
	m.settleOldWaiting(ctx)
	m.startQueued()
	return nil
}
