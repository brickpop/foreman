// Package screen keeps a session's terminal state on the server, so that a
// viewer attaching late (or reloading) gets an exact snapshot of a full-screen
// program, and so that terminal queries are answered while nobody watches
// (wiki/product/architecture/terminal-state.md). It wraps
// github.com/charmbracelet/x/vt; nothing else imports that library.
package screen

import (
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"

	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/vt"

	"github.com/brickpop/foreman/internal/runner"
)

// ScrollbackLines is how much history a screen keeps for snapshots.
const ScrollbackLines = 10000

// answerQueue bounds the answers waiting to be written to the session. Answers
// are tiny and rare; overflowing means the session isn't reading its input.
const answerQueue = 256

// VT is a server-side terminal: it is fed everything the session prints.
type VT struct {
	mu    sync.Mutex
	emu   *vt.Emulator
	modes map[ansi.Mode]bool // modes the program changed from the defaults: true = set
	title string
	pend  pending // unfinished sequence at the end of the stream
	sh    shadow  // pen, scroll region and saved cursor, which the emulator keeps private

	queue chan []byte
	done  chan struct{}
}

// New returns a screen of the given size. answer receives the emulator's
// answers to terminal queries, to be written to the session as input; it is
// called from its own goroutine, never while the screen is locked.
func New(size runner.Size, answer func([]byte)) *VT {
	s := &VT{
		emu:   vt.NewEmulator(int(size.Cols), int(size.Rows)),
		modes: map[ansi.Mode]bool{},
		queue: make(chan []byte, answerQueue),
		done:  make(chan struct{}),
	}
	s.emu.Scrollback().SetMaxLines(ScrollbackLines)
	s.emu.SetCallbacks(vt.Callbacks{
		EnableMode:  func(m ansi.Mode) { s.modes[m] = true },
		DisableMode: func(m ansi.Mode) { s.modes[m] = false },
		Title:       func(t string) { s.title = t },
		AltScreen:   func(on bool) { s.sh.altScreen(on) },
	})
	s.sh.attach(s.emu)
	go s.drain()
	go func() {
		defer close(s.done)
		for p := range s.queue {
			answer(p)
		}
	}()
	return s
}

// drain reads the emulator's answers as soon as they are written: the emulator
// writes them to a synchronous pipe, so an unread answer would block the next
// Write and stall the session.
func (s *VT) drain() {
	defer close(s.queue)
	buf := make([]byte, 4096)
	for {
		n, err := s.emu.Read(buf)
		if n > 0 {
			select {
			case s.queue <- append([]byte(nil), buf[:n]...):
			default:
				slog.Warn("terminal answer dropped: the session isn't reading its input")
			}
		}
		if err != nil {
			return
		}
	}
}

// Write feeds the session's output.
func (s *VT) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pend.write(p)
	return s.emu.Write(p)
}

// Resize follows the session's terminal size.
func (s *VT) Resize(size runner.Size) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.emu.Resize(int(size.Cols), int(size.Rows))
	s.sh.resized()
}

// Close stops the screen; pending answers are still delivered.
func (s *VT) Close() error {
	// vt's Close sets an unguarded flag that its Read checks, so closing while
	// drain is reading is a data race (wiki/technical-debt.md). Ending the
	// answers pipe first lets drain finish; the emulator is closed after.
	s.mu.Lock()
	if pw, ok := s.emu.InputPipe().(*io.PipeWriter); ok {
		_ = pw.CloseWithError(io.EOF)
	} else {
		_ = s.emu.Close() // racy, but drain must end
	}
	s.mu.Unlock()
	<-s.done
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.emu.Close()
}

// modes the snapshot restores in its own way, not by replaying them.
var restoredOtherwise = map[ansi.Mode]bool{
	ansi.ModeAltScreen:           true, // restored as the alternate screen itself
	ansi.ModeAltScreenSaveCursor: true,
	ansi.ModeSaveCursor:          true, // replaying it would move the cursor
	ansi.ModeTextCursorEnable:    true, // restored as cursor visibility
}

// Snapshot returns bytes that recreate the current state on a fresh terminal
// of the same size: scrollback, screen (and the alternate screen), cursor,
// modes, title, and the unfinished sequence the stream is in the middle of. It doesn't restore the main screen underneath the alternate
// screen, nor the scroll region: programs redraw those on their own.
func (s *VT) Snapshot() []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	var b strings.Builder

	// History first: on a fresh terminal, lines written past the bottom scroll
	// up into the scrollback, exactly where they were.
	var lines []string
	for _, l := range s.emu.Scrollback().Lines() {
		lines = append(lines, l.Render())
	}
	screen := s.screenLines()
	if s.emu.IsAltScreen() {
		// The main screen underneath isn't available: push all the history into
		// the scrollback, as on the original, and leave the main screen blank.
		if len(lines) > 0 {
			b.WriteString(strings.Join(lines, "\r\n"))
			b.WriteString(strings.Repeat("\r\n", s.emu.Height()))
		}
		b.WriteString(ansi.SetMode(ansi.ModeAltScreenSaveCursor))
		b.WriteString(ansi.CursorPosition(1, 1))
		b.WriteString(strings.Join(screen, "\r\n"))
	} else {
		b.WriteString(strings.Join(append(lines, screen...), "\r\n"))
	}

	for mode, set := range s.modes {
		if restoredOtherwise[mode] {
			continue
		}
		if set {
			b.WriteString(ansi.SetMode(mode))
		} else {
			b.WriteString(ansi.ResetMode(mode))
		}
	}
	if s.title != "" {
		b.WriteString(ansi.SetWindowTitle(s.title))
	}
	b.WriteString(s.sh.restoreRegion())
	b.WriteString(s.sh.restoreSaved())
	if style, blink := s.emu.CursorStyle(); style != vt.CursorBlock || !blink {
		b.WriteString(ansi.SetCursorStyle(cursorStyleParam(style, blink)))
	}
	pos := s.emu.CursorPosition()
	b.WriteString(ansi.CursorPosition(pos.X+1, pos.Y+1))
	if s.emu.CursorHidden() {
		b.WriteString(ansi.ResetMode(ansi.ModeTextCursorEnable))
	}
	b.WriteString(s.sh.restorePen())
	// Last: the viewer's parser must be mid-sequence exactly where the stream is.
	b.Write(s.pend.tail())
	return []byte(b.String())
}

// screenLines renders each visible row, exactly height of them.
func (s *VT) screenLines() []string {
	rows := strings.Split(s.emu.Render(), "\n")
	for len(rows) < s.emu.Height() {
		rows = append(rows, "")
	}
	return rows[:s.emu.Height()]
}

// cursorStyleParam maps a cursor style to the DECSCUSR parameter.
func cursorStyleParam(style vt.CursorStyle, blink bool) int {
	n := 2*int(style) + 1 // block 1/2, underline 3/4, bar 5/6: odd = blinking
	if !blink {
		n++
	}
	return n
}

var _ io.Writer = (*VT)(nil)

// String is the visible screen as plain text, for tests and debugging.
func (s *VT) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return fmt.Sprint(s.emu.String())
}
