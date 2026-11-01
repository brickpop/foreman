package instances

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"sync"

	"github.com/brickpop/foreman/internal/store"
)

// Deploy deploys team on project and starts its lead; the other
// members sleep until woken. A project has one team at most: another team
// deployed on it is stopped first, as by Stop.
func (m *Manager) Deploy(ctx context.Context, teamID, projectID string) (store.Deployment, error) {
	if err := m.blocked(); err != nil {
		return store.Deployment{}, err
	}
	team, project, on, err := m.deployable(ctx, teamID, projectID)
	if err != nil {
		return store.Deployment{}, err
	}
	// One deploy at a time: two onto one project can't both replace its team.
	m.deploying.Lock()
	defer m.deploying.Unlock()
	if err := m.replace(ctx, teamID, projectID); err != nil {
		return store.Deployment{}, err
	}
	dep, err := m.Deployments.Deploy(ctx, teamID, projectID)
	if err != nil {
		return store.Deployment{}, err
	}
	m.publish("deployment.created", dep)
	for _, agent := range team.Config.Roster() {
		inst := store.Instance{
			ID: agent + "@" + projectID, DeploymentID: dep.ID, AgentID: agent, Status: Asleep,
			SandboxPath: project.Sandbox(agent), Harness: on.harness, Profile: on.profile,
		}
		if err := m.Instances.Put(ctx, inst); err != nil { // a new instance: nothing to change yet
			return dep, err
		}
		if agent == team.Config.Lead {
			m.enqueue(inst.ID)
		}
	}
	m.startQueued()
	return dep, nil
}

// replace stops every other team's active deployment on project.
func (m *Manager) replace(ctx context.Context, teamID, projectID string) error {
	deps, err := m.Deployments.List(ctx)
	if err != nil {
		return err
	}
	for _, d := range deps {
		if d.ProjectID == projectID && d.TeamID != teamID && d.Status == store.DeploymentActive {
			if err := m.stop(ctx, d.ID, ""); err != nil {
				return err
			}
		}
	}
	return nil
}

// Wake starts an asleep instance (the CEO's Start, or work arriving for it),
// or, on the CEO's Start again, a stopped one of an active deployment: both
// pick up their conversation. An instance that is up or on its way is left as
// it is.
func (m *Manager) Wake(ctx context.Context, id string) error {
	inst, err := m.Instances.Get(ctx, id)
	if err != nil {
		return err
	}
	if inst.Status != Asleep && inst.Status != Stopped {
		return nil
	}
	if dep, err := m.Deployments.Get(ctx, inst.DeploymentID); err != nil || dep.Status != store.DeploymentActive {
		return nil // a stopped deployment's instances start only with a redeploy
	}
	if err := m.blocked(); err != nil {
		return err
	}
	if err := m.resetRestarts(ctx, id); err != nil {
		return err
	}
	m.enqueue(id)
	m.startQueued()
	return nil
}

// Restart stops a running instance and starts it again at once, on its
// project's profile as it is now (Project settings → Profile), continuing
// its conversation unless the harness changed. One that isn't up is left as
// it is: it starts on that profile anyway.
func (m *Manager) Restart(ctx context.Context, id string) error {
	inst, err := m.Instances.Get(ctx, id)
	if err != nil {
		return err
	}
	switch inst.Status {
	case Starting, Running, Idle, Awaiting:
	default:
		return nil
	}
	if err := m.blocked(); err != nil {
		return err
	}
	if r := m.end(id); r != nil {
		if r.proc != nil {
			if err := r.proc.Stop(stopGrace); err != nil {
				return err
			}
		}
		// As a stop: the slot is free at once, and a run still starting ends
		// itself without touching the new one.
		m.release(id, r)
	}
	if err := m.resetRestarts(ctx, id); err != nil {
		return err
	}
	m.enqueue(id)
	m.startQueued()
	return nil
}

// blocked is why agents can't start now, wrapping ErrNotReady, or nil.
func (m *Manager) blocked() error {
	m.mu.Lock()
	closing := m.closing
	m.mu.Unlock()
	if closing {
		return fmt.Errorf("%w: foreman is stopping", ErrNotReady)
	}
	if m.Blocked == nil {
		return nil
	}
	if err := m.Blocked(); err != nil {
		return fmt.Errorf("%w: %v", ErrNotReady, err)
	}
	return nil
}

// Stop stops a deployment's instances and marks it stopped (the CEO's Stop).
// Their worktrees stay on disk.
func (m *Manager) Stop(ctx context.Context, deploymentID string) error {
	return m.stop(ctx, deploymentID, "")
}

// stop is Stop, with why when Foreman stops it on its own.
func (m *Manager) stop(ctx context.Context, deploymentID, reason string) error {
	dep, err := m.Deployments.Get(ctx, deploymentID)
	if err != nil {
		return err
	}
	insts, err := m.Instances.List(ctx, deploymentID)
	if err != nil {
		return err
	}
	for _, inst := range insts {
		m.mu.Lock()
		m.queue = slices.DeleteFunc(m.queue, func(q string) bool { return q == inst.ID })
		m.mu.Unlock()
		if r := m.end(inst.ID); r != nil {
			if r.proc != nil {
				if err := r.proc.Stop(stopGrace); err != nil {
					slog.Warn("stopping an instance", "instance", inst.ID, "err", err)
				}
			}
			// Its slot is free at once, also mid-start, so a redeploy right
			// after starts a new run; the old run's late end then changes nothing.
			m.release(inst.ID, r)
		}
		m.setStatus(ctx, inst.ID, Stopped)
	}
	if err := m.Deployments.Stop(ctx, deploymentID, reason); err != nil {
		return err
	}
	if dep.Status == store.DeploymentActive {
		dep.StopReason = reason
	}
	dep.Status = store.DeploymentStopped
	m.publish("deployment.stopped", dep)
	m.startQueued() // its slots go to whoever waits
	return nil
}

// Shutdown stops every instance's process as Foreman exits, all at once, each
// with the usual grace. Each goes to sleep, not to stopped: work or the CEO's
// Start wakes it on a later run, continuing its conversation. One whose stop
// fails (Podman not answering) keeps its status, so the next start re-attaches
// it as after a crash. Nothing starts after it
// (wiki/product/architecture/agent-runtime.md#recovery).
func (m *Manager) Shutdown(ctx context.Context) {
	m.mu.Lock()
	m.closing = true
	ids := m.queue
	m.queue = nil
	running := map[string]*run{}
	for id, r := range m.runs {
		// A start under way sees it and stops what it launched.
		r.ending = endAsked
		ids = append(ids, id)
		if r.proc != nil {
			running[id] = r
		}
	}
	m.mu.Unlock()
	var stops sync.WaitGroup
	var failedMu sync.Mutex
	failed := map[string]bool{}
	for id, r := range running {
		stops.Go(func() {
			if err := r.proc.Stop(stopGrace); err != nil {
				slog.Warn("stopping an instance; the next start re-attaches it", "instance", id, "err", err)
				failedMu.Lock()
				failed[id] = true
				failedMu.Unlock()
				return
			}
			// Its terminal has read the end (and closed its log), and the slot
			// is free, before Shutdown returns: its watcher would do both later.
			<-r.term.Done()
			m.release(id, r)
		})
	}
	stops.Wait()
	m.Wait()
	for _, id := range ids {
		if !failed[id] {
			m.setStatus(ctx, id, Asleep)
		}
	}
}
