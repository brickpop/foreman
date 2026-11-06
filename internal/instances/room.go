package instances

import (
	"context"
	"log/slog"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/brickpop/foreman/internal/store"
)

// An idle agent holds a slot while it waits for nothing. When another
// instance waits for one, the idle agent quiet the longest gives its slot up:
// its process stops, it goes asleep, and a message or the CEO's Start wakes
// it with its conversation (the CEO, 2026-10-23;
// wiki/product/architecture/agent-runtime.md#idle-agents-make-room).

// idleBeforeSleep is how long an agent is idle and untouched before it may
// give its slot up: one just finished may be about to get its reply.
const idleBeforeSleep = time.Minute

// MakeRoom puts idle agents to sleep while instances wait for a slot: one per
// waiting instance, the longest quiet first, each only if it is quiet (sleep).
// Called when instances are left waiting, and once a minute, for agents
// that have gone idle since.
func (m *Manager) MakeRoom(ctx context.Context, now time.Time) {
	type candidate struct {
		id    string
		r     *run
		since time.Time
	}
	m.mu.Lock()
	if m.makingRoom || m.closing || len(m.queue) == 0 {
		m.mu.Unlock()
		return
	}
	m.makingRoom = true
	need := len(m.queue)
	var idle []candidate
	for id, r := range m.runs {
		if r.term == nil || r.ending != notEnding || r.idleSince.IsZero() {
			continue
		}
		since := r.idleSince
		if last := r.term.LastInput(); last.After(since) {
			since = last // the CEO's keys, or a delivery
		}
		if now.Sub(since) >= idleBeforeSleep {
			idle = append(idle, candidate{id, r, since})
		}
	}
	m.mu.Unlock()
	defer func() {
		m.mu.Lock()
		m.makingRoom = false
		m.mu.Unlock()
	}()
	slices.SortFunc(idle, func(a, b candidate) int { return a.since.Compare(b.since) })
	for _, c := range idle {
		if need == 0 {
			return
		}
		if m.sleep(ctx, c.id, c.r, c.since) {
			need--
		}
	}
}

// sleep stops an idle instance's process, quiet since since (its watch then
// puts it to sleep), unless the CEO has a draft in it, something besides
// foreman-agent runs in it (a dev server, a build), or it stirred meanwhile.
// Its terminal refuses input from the last check on, so a delivery either
// came before (and it stays awake) or waits for it to wake. It reports
// whether the process stopped.
func (m *Manager) sleep(ctx context.Context, id string, r *run, since time.Time) bool {
	if m.Drafting != nil && m.Drafting(id) {
		return false
	}
	procs, err := r.proc.Processes(ctx)
	if err != nil {
		slog.Warn("listing an idle instance's processes", "instance", id, "err", err)
		return false
	}
	for _, p := range procs {
		if !m.helper(p) {
			return false
		}
	}
	sealed := false
	_ = m.change(ctx, id, func(inst *store.Instance) bool {
		m.mu.Lock()
		defer m.mu.Unlock()
		sealed = inst.Status == Idle && m.current(id, r) && !r.idleSince.IsZero() &&
			!r.idleSince.After(since) && r.term.Seal(since)
		if sealed {
			r.ending = endSleep
		}
		return false // a check: the watch changes it once the process ended
	})
	if !sealed {
		return false
	}
	if err := r.proc.Stop(stopGrace); err != nil {
		slog.Warn("putting an instance to sleep", "instance", id, "err", err)
		m.mu.Lock()
		r.ending = notEnding
		m.mu.Unlock()
		r.term.Unseal()
		return false
	}
	return true
}

// helper reports whether a process (its command line) is one an idle harness
// keeps: foreman-agent, its MCP server for the team tools.
func (m *Manager) helper(command string) bool {
	program, _, _ := strings.Cut(command, " ")
	return m.Agent.Path != "" && filepath.Base(program) == filepath.Base(m.Agent.Path)
}
