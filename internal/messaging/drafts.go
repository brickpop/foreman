package messaging

import (
	"time"
	"unicode/utf8"

	"github.com/brickpop/foreman/internal/events"
)

// The CEO's draft: text typed straight into an agent's terminal and not sent
// yet (wiki/product/architecture/team-tools.md#delivery). A delivery would
// land on it, so messages wait while it is there, on a countdown that runs
// only while the CEO isn't typing. Sending or clearing the line lets them
// through; the CEO can pause the countdown; at zero they go out after the
// draft, marked as unfinished.

// draftNote follows the separator when a delivery can't wait any longer.
const draftNote = "The CEO's text above is an unfinished draft: don't act on it yet."

type draft struct {
	length  int // characters the CEO typed since the last Enter (or clear)
	lastKey time.Time
	// The countdown, while messages wait on the draft.
	holding bool
	left    time.Duration
	paused  bool
	expired bool // the countdown ran out: the next delivery goes after the draft
	waiting int  // messages waiting
}

// Typed hears what the CEO types into an agent's terminal: printable text
// makes a draft; Enter, Ctrl+C and Ctrl+U end it; Backspace shrinks it.
// Escape sequences (arrows, function keys) change nothing.
func (m *Messenger) Typed(id string, p []byte) {
	m.mu.Lock()
	d := m.draft(id)
	ended, submitted := false, false
	for i := 0; i < len(p); {
		switch b := p[i]; {
		case b == '\r' || b == '\n':
			ended, submitted = d.length > 0 || ended, true
			d.length = 0
			i++
		case b == 0x03 || b == 0x15: // Ctrl+C, Ctrl+U
			ended = d.length > 0 || ended
			d.length = 0
			i++
		case b == 0x7f || b == 0x08:
			if d.length > 0 {
				d.length--
				ended = d.length == 0 || ended
			}
			i++
		case b == 0x1b:
			i += escapeLength(p[i:])
		case b < 0x20:
			i++
		default:
			_, size := utf8.DecodeRune(p[i:])
			d.length++
			i += size
		}
	}
	d.lastKey = m.now()
	holding := d.holding
	if ended && d.length == 0 {
		m.release(id, d)
	}
	m.mu.Unlock()
	switch {
	case !holding || !ended:
	case submitted:
		// The CEO's prompt goes first: its turn starts (and the messages follow
		// at the next idle), or, if the Enter started nothing, they go now.
		time.AfterFunc(m.Quiet, func() { m.Deliver(id) })
	default:
		go m.Deliver(id)
	}
}

// escapeLength is how long the escape sequence at the start of p is: ESC [
// … final byte, ESC O x, or ESC and one more byte. A bracketed paste's
// markers are sequences too; what is pasted between them is text.
func escapeLength(p []byte) int {
	if len(p) < 2 {
		return len(p)
	}
	switch p[1] {
	case '[':
		for i := 2; i < len(p); i++ {
			if p[i] >= 0x40 && p[i] <= 0x7e {
				return i + 1
			}
		}
		return len(p)
	case 'O':
		return min(3, len(p))
	}
	return 2
}

// Drafting reports whether the CEO has typed into the instance's prompt
// without sending it: it keeps the instance awake.
func (m *Messenger) Drafting(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	d := m.drafts[id]
	return d != nil && d.length > 0
}

// PauseCountdown stops or restarts an instance's countdown.
func (m *Messenger) PauseCountdown(id string, paused bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if d := m.drafts[id]; d != nil && d.holding {
		d.paused = paused
		m.publishDraft(id, d)
	}
}

// draft is an instance's draft, made on first use. Called with m.mu held.
func (m *Messenger) draft(id string) *draft {
	d := m.drafts[id]
	if d == nil {
		d = &draft{}
		m.drafts[id] = d
	}
	return d
}

// hold says whether waiting messages must wait on the CEO's draft, starting
// the countdown the first time; after it ran out, they go (after the draft).
func (m *Messenger) hold(id string, waiting int) (wait, after bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	d := m.drafts[id]
	switch {
	case d == nil || d.length == 0:
		return false, false
	case d.expired:
		return false, true
	}
	d.waiting = waiting
	if !d.holding {
		d.holding, d.left, d.paused = true, m.Hold, false
		go m.countdown(id, d)
	}
	m.publishDraft(id, d)
	return true, false
}

// countdown runs while messages wait on a draft: it counts down only while
// the CEO isn't typing and hasn't paused it, and at zero lets them through.
func (m *Messenger) countdown(id string, d *draft) {
	tick := time.NewTicker(m.Tick)
	defer tick.Stop()
	for range tick.C {
		m.mu.Lock()
		if !d.holding {
			m.mu.Unlock()
			return
		}
		if m.counting(d) {
			d.left -= m.Tick
		}
		if d.left <= 0 {
			d.expired = true
			m.mu.Unlock()
			go m.Deliver(id)
			return
		}
		m.publishDraft(id, d)
		m.mu.Unlock()
	}
}

func (m *Messenger) counting(d *draft) bool {
	return !d.paused && m.now().Sub(d.lastKey) >= m.Quiet
}

// release ends a draft and its countdown. Called with m.mu held.
func (m *Messenger) release(id string, d *draft) {
	wasHolding := d.holding
	*d = draft{lastKey: d.lastKey}
	if wasHolding {
		m.publishDraft(id, d)
	}
}

// forget drops an instance's draft: its session ended or restarted, so its
// prompt is empty again.
func (m *Messenger) forget(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if d := m.drafts[id]; d != nil {
		m.release(id, d)
		delete(m.drafts, id)
	}
}

func (m *Messenger) publishDraft(id string, d *draft) {
	if m.Publish == nil {
		return
	}
	waiting := d.waiting
	if !d.holding {
		waiting = 0
	}
	m.Publish(events.Event{Type: "instance.draft", Data: Draft{
		ID: id, Waiting: waiting, LeftMs: max(d.left, 0).Milliseconds(), Counting: d.holding && m.counting(d), Paused: d.paused,
	}})
}

// Line types the CEO's line into an agent's session and sends it: the same
// typing as a delivery (never mixed with one), so the harness takes the Enter
// as sending, not as part of a paste. An empty line sends Enter alone (it
// answers a harness's prompt).
func (m *Messenger) Line(id, text string) error {
	m.delivering.Lock()
	err := m.typeLines(id, splitLines(text))
	m.delivering.Unlock()
	if err != nil {
		return err
	}
	m.Typed(id, []byte("\r")) // the line, and any draft before it, went out
	return nil
}
