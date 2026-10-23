package instances

import (
	"context"
	"slices"
	"time"

	"github.com/brickpop/foreman/internal/store"
)

// enqueue puts an instance in the queue for a slot, unless it has one or is
// in it already.
func (m *Manager) enqueue(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.runs[id] == nil && !slices.Contains(m.queue, id) {
		m.queue = append(m.queue, id)
	}
}

// startQueued starts queued instances while there are free slots
// (max_sessions; a slot is held by every run, starting or not). Those left
// waiting are marked queued: only they, so an instance that starts at once
// never shows queued (nor warns the CEO about slots). An idle agent may then
// make room for them (MakeRoom).
func (m *Manager) startQueued() {
	max := m.Config.Settings().Settings.MaxSessions
	for {
		m.mu.Lock()
		if m.closing || len(m.queue) == 0 {
			m.mu.Unlock()
			return
		}
		if len(m.runs) >= max {
			waiting := slices.Clone(m.queue)
			m.work.Add(1) // under the lock that checked closing: Shutdown waits for it
			m.mu.Unlock()
			for _, id := range waiting {
				m.markQueued(id)
			}
			go func() {
				defer m.work.Done()
				m.MakeRoom(context.Background(), time.Now())
			}()
			return
		}
		id := m.queue[0]
		m.queue = m.queue[1:]
		r := &run{}
		m.runs[id] = r
		m.work.Add(1)
		m.mu.Unlock()
		go func() {
			defer m.work.Done()
			m.start(id, r)
		}()
	}
}

// LimitChanged starts the queued instances that fit under a new session limit
// (settings.toml changed). A lower limit stops nothing: running ones finish.
func (m *Manager) LimitChanged() { m.startQueued() }

// markQueued marks an instance queued if it still waits: a start leaves the
// queue before it changes the status, so one that started meanwhile is
// never marked.
func (m *Manager) markQueued(id string) {
	_ = m.change(context.Background(), id, func(inst *store.Instance) bool {
		m.mu.Lock()
		waiting := slices.Contains(m.queue, id)
		m.mu.Unlock()
		if !waiting || inst.Status == Queued {
			return false
		}
		inst.Status = Queued
		return true
	})
}
