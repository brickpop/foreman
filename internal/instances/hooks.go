package instances

import (
	"context"
	"strings"
	"time"
	"unicode"

	"github.com/brickpop/foreman/internal/harness"
	"github.com/brickpop/foreman/internal/store"
)

// activityStatus is the status each harness activity means.
var activityStatus = map[harness.Activity]string{harness.Busy: Running, harness.Idle: Idle, harness.Waiting: Awaiting}

// Hook takes one of an instance's harness hook events: what it says the
// harness is doing becomes the instance's status (running, idle, awaiting)
// and its activity line. Before the instance runs, or once its run is
// ending, the event changes nothing.
func (m *Manager) Hook(id, event string, payload []byte) {
	ctx := context.Background()
	inst, err := m.Instances.Get(ctx, id)
	if err != nil {
		return
	}
	adapter := m.Adapters[inst.Harness]
	m.mu.Lock()
	r := m.runs[id]
	live := r != nil && m.current(id, r)
	m.mu.Unlock()
	if adapter == nil || !live || !runsNow(inst.Status) {
		return
	}
	m.heardFrom(ctx, id, r)
	activity := adapter.Interpret(event, payload)
	switch activity {
	case harness.Waiting:
		m.raiseWaiting(ctx, id, r, adapter.Asks(event, payload))
	case harness.Busy, harness.Idle:
		m.answeredInSession(ctx, r)
	}
	if status, ok := activityStatus[activity]; ok {
		_ = m.change(ctx, id, func(inst *store.Instance) bool {
			m.mu.Lock()
			defer m.mu.Unlock()
			if !m.current(id, r) || !runsNow(inst.Status) || inst.Status == status {
				return false
			}
			inst.Status = status
			return true
		})
	}
	if line := oneLine(adapter.Describe(event, payload)); line != "" {
		a := store.Doing{Line: line, At: time.Now().UTC()}
		m.mu.Lock()
		r.doing = &a
		m.mu.Unlock()
		m.publish("instance.doing", DoingChange{ID: id, Doing: a})
	}
}

// runsNow reports whether a status is one of a run under way: its harness's
// hooks move it among them.
func runsNow(status string) bool {
	return status == Running || status == Idle || status == Awaiting
}

// DoingChange is the instance.doing event's data.
type DoingChange struct {
	ID    string      `json:"id"`
	Doing store.Doing `json:"doing"`
}

// doingMax is the most of a line kept, in characters: a row shows one line.
const doingMax = 200

// oneLine is a description as a row shows it: its first line, without
// control characters (a payload's text is the harness's, so anything), cut
// at doingMax.
func oneLine(s string) string {
	s, _, _ = strings.Cut(strings.TrimSpace(s), "\n")
	s = strings.TrimSpace(strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, s))
	if r := []rune(s); len(r) > doingMax {
		s = string(r[:doingMax-1]) + "…"
	}
	return s
}
