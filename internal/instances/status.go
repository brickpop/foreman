package instances

import (
	"context"
	"log/slog"
	"time"

	"github.com/brickpop/foreman/internal/store"
)

// StatusChange is the instance.status event's data.
type StatusChange struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

// change checks and changes an instance in the store as one step: edit
// changes it, or returns false to leave it as it is (a transition that no
// longer applies, or only a check). It is the only way an existing instance
// is written, and changes go one at a time, so each edit sees what the last
// one left. A new status starts or stops its run's idle clock; it is
// published (a new stop reason too) before the next change.
func (m *Manager) change(ctx context.Context, id string, edit func(*store.Instance) bool) error {
	m.changing.Lock()
	defer m.changing.Unlock()
	inst, err := m.Instances.Get(ctx, id)
	if err != nil {
		return err
	}
	was, wasReason := inst.Status, inst.StopReason
	if !edit(&inst) {
		return nil
	}
	if err := m.Instances.Put(ctx, inst); err != nil {
		slog.Warn("saving an instance", "instance", id, "err", err)
		return err
	}
	if inst.Status == was && inst.StopReason == wasReason {
		return nil
	}
	m.mu.Lock()
	if r := m.runs[id]; r != nil {
		r.idleSince = time.Time{}
		if inst.Status == Idle {
			r.idleSince = time.Now()
		}
	}
	m.mu.Unlock()
	m.publish("instance.status", StatusChange{ID: id, Status: inst.Status})
	return nil
}

// setStatus sets an instance's status, clearing a stop's reason.
func (m *Manager) setStatus(ctx context.Context, id, status string) {
	_ = m.change(ctx, id, func(inst *store.Instance) bool {
		if inst.Status == status {
			return false
		}
		inst.Status, inst.StopReason = status, ""
		return true
	})
}

// stopped stops an instance on Foreman's own decision, saying why: its pane
// shows it until the next start.
func (m *Manager) stopped(ctx context.Context, id, reason string) {
	_ = m.change(ctx, id, func(inst *store.Instance) bool {
		inst.Status, inst.StopReason = Stopped, reason
		return true
	})
}

// resetRestarts gives the instance its one restart after a crash again (the
// CEO started it).
func (m *Manager) resetRestarts(ctx context.Context, id string) error {
	return m.change(ctx, id, func(inst *store.Instance) bool {
		if inst.Restarts == 0 {
			return false
		}
		inst.Restarts = 0
		return true
	})
}
