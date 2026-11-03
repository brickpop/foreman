package instances

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/brickpop/foreman/internal/runner"
	"github.com/brickpop/foreman/internal/store"
	"github.com/brickpop/foreman/internal/terminal"
)

// run is one run of an instance, from its start (or its re-attaching after a
// Foreman restart) to its process's end. It holds the instance's session
// slot. What is known about the instance only while it runs lives here, and
// goes with it. Its fields are guarded by Manager.mu.
type run struct {
	term   *terminal.Terminal // nil while starting
	proc   runner.Process
	ending ending

	doing     *store.Doing   // what its harness last said it is doing (Hook)
	heard     time.Time      // its last activity: a hook, or its start (CheckStuck)
	stuck     *store.Request // its open stuck card
	waiting   *store.Request // its open card for the harness waiting on the user (waiting.go)
	idleSince time.Time      // when it went idle; zero unless idle (MakeRoom)
}

// ending is why a run's process ends, when it was asked to.
type ending int

const (
	notEnding ending = iota
	// endAsked: a stop, a restart, Foreman exiting. Whoever asked sets what
	// follows; its end is not a crash.
	endAsked
	// endSleep: MakeRoom put it to sleep. Its watch makes it asleep.
	endSleep
)

// current reports whether r is still the instance's run, and not ending.
// Called with m.mu held.
func (m *Manager) current(id string, r *run) bool {
	return m.runs[id] == r && r.ending == notEnding
}

// end marks the instance's run, if any, as ending at someone's request, and
// returns it.
func (m *Manager) end(id string) *run {
	m.mu.Lock()
	defer m.mu.Unlock()
	r := m.runs[id]
	if r != nil {
		r.ending = endAsked
	}
	return r
}

// release frees the slot of one run of the instance: only its own, so the late
// end of a stopped run never frees the run that replaced it (a redeploy). It
// says whether the slot was still that run's.
func (m *Manager) release(id string, r *run) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.runs[id] != r {
		return false
	}
	delete(m.runs, id)
	return true
}

// start prepares the instance's sandbox, launches its harness and keeps its
// terminal. A failure stops the instance (with its reason in the log).
func (m *Manager) start(id string, r *run) {
	ctx := context.Background()
	m.setStatusIfCurrent(ctx, id, r, Starting)
	proc, err := m.launch(ctx, id)
	var log terminal.Log
	if err == nil {
		if log, err = m.OpenLog(id); err != nil {
			_ = proc.Stop(stopGrace)
			err = fmt.Errorf("session log: %w", err)
		}
	}
	if err == nil {
		// Checked and handed over in one step: a stop either sees the process
		// (and stops it) or is seen here.
		m.mu.Lock()
		asked := r.ending != notEnding
		if !asked {
			r.proc, r.term, r.heard = proc, terminal.New(id, proc, log), time.Now()
		}
		m.mu.Unlock()
		if asked {
			_ = proc.Stop(stopGrace)
			_ = log.Close()
			err = errors.New("stopped while starting")
		}
	}
	if err != nil {
		slog.Warn("starting an instance", "instance", id, "err", err)
		// A stop already freed the slot and set the status; a run may have
		// replaced this one since.
		if m.release(id, r) {
			m.stopped(ctx, id, "couldn't start: "+err.Error())
		}
		m.startQueued()
		return
	}
	_ = m.change(ctx, id, func(inst *store.Instance) bool {
		m.mu.Lock()
		defer m.mu.Unlock()
		if !m.current(id, r) {
			return false // stopped meanwhile: the stop set its status
		}
		inst.Status, inst.StartedAt, inst.StopReason, inst.Fresh = Running, time.Now(), "", false
		return true
	})
	go m.watch(id, r)
}

// setStatusIfCurrent sets the status while r is the instance's live run.
func (m *Manager) setStatusIfCurrent(ctx context.Context, id string, r *run, status string) {
	_ = m.change(ctx, id, func(inst *store.Instance) bool {
		m.mu.Lock()
		defer m.mu.Unlock()
		if !m.current(id, r) || inst.Status == status {
			return false
		}
		inst.Status, inst.StopReason = status, ""
		return true
	})
}

// adopt takes over a process found running.
func (m *Manager) adopt(ctx context.Context, id string, p runner.Process) error {
	log, err := m.OpenLog(id)
	if err != nil {
		_ = p.Stop(stopGrace)
		return err
	}
	inst, err := m.Instances.Get(ctx, id)
	if err != nil {
		_ = p.Stop(stopGrace)
		_ = log.Close()
		return err
	}
	r := &run{proc: p, term: terminal.New(id, p, log), heard: time.Now()}
	if inst.Status == Idle {
		r.idleSince = time.Now() // its idle clock starts over
	}
	m.mu.Lock()
	if m.closing {
		// Exiting: left running, for the next start to re-attach.
		m.mu.Unlock()
		_ = log.Close()
		return nil
	}
	m.runs[id] = r
	m.mu.Unlock()
	// Its process lived on: what its hooks last said still holds (idle stays
	// idle, so a message queued for it is delivered).
	if inst.Status != Idle && inst.Status != Awaiting {
		m.setStatusIfCurrent(ctx, id, r, Running)
	}
	go m.watch(id, r)
	return nil
}

// watch waits for the run's process to end. One asked to end is left to
// whoever asked; one put to sleep goes asleep; otherwise a clean exit sleeps,
// and a crash restarts once (continuing its conversation) and stops at the
// second.
func (m *Manager) watch(id string, r *run) {
	<-r.term.Done()
	ctx := context.Background()
	m.mu.Lock()
	ending, stuck, waiting := r.ending, r.stuck, r.waiting
	r.stuck, r.waiting = nil, nil
	m.mu.Unlock()
	m.resolveCard(ctx, stuck, "its run ended")
	m.resolveCard(ctx, waiting, "its run ended")
	released := m.release(id, r)
	switch {
	case ending == endSleep && released:
		_ = m.change(ctx, id, func(inst *store.Instance) bool {
			if inst.Status != Idle {
				return false // a stop or a restart took it meanwhile
			}
			inst.Status = Asleep
			return true
		})
	case ending != notEnding:
	default:
		m.exited(ctx, id, r)
	}
	m.startQueued()
}

// exited follows a run that ended on its own.
func (m *Manager) exited(ctx context.Context, id string, r *run) {
	code, _ := r.term.ExitCode()
	restart := false
	_ = m.change(ctx, id, func(inst *store.Instance) bool {
		switch {
		case code == 0:
			// A clean exit (the CEO's /exit, say) is done for now, not a
			// crash: it sleeps, holding no slot, until work or the CEO's Start
			// wakes it, and then picks up its conversation.
			inst.Status, inst.Restarts = Asleep, 0
		case inst.Restarts == 0:
			slog.Warn("instance exited; restarting it once", "instance", id, "code", code)
			inst.Restarts++
			restart = true
		default:
			slog.Warn("instance exited again; stopped", "instance", id, "code", code)
			inst.Status, inst.StopReason = Stopped, fmt.Sprintf("it exited twice in a row (code %d)", code)
		}
		return true
	})
	if restart {
		m.enqueue(id)
	}
}
