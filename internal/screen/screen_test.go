package screen

import (
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/brickpop/foreman/internal/runner"
)

var size = runner.Size{Cols: 24, Rows: 6}

func newScreen(t *testing.T) *VT {
	t.Helper()
	s := New(size, func([]byte) {})
	t.Cleanup(func() { _ = s.Close() })
	return s
}

// sameTerminal fails unless a and b look and behave the same: every cell
// (content and style), cursor, cursor style and visibility, alternate screen,
// scrollback, modes and title.
func sameTerminal(t *testing.T, label string, a, b *VT) {
	t.Helper()
	a.mu.Lock()
	defer a.mu.Unlock()
	b.mu.Lock()
	defer b.mu.Unlock()
	for y := 0; y < a.emu.Height(); y++ {
		for x := 0; x < a.emu.Width(); x++ {
			ca, cb := a.emu.CellAt(x, y), b.emu.CellAt(x, y)
			if !reflect.DeepEqual(ca, cb) {
				t.Fatalf("%s: cell (%d,%d) differs: %+v vs %+v\nA:\n%s\nB:\n%s", label, x, y, ca, cb, a.emu.String(), b.emu.String())
			}
		}
	}
	if pa, pb := a.emu.CursorPosition(), b.emu.CursorPosition(); pa != pb {
		t.Errorf("%s: cursor %v vs %v", label, pa, pb)
	}
	if a.emu.CursorHidden() != b.emu.CursorHidden() {
		t.Errorf("%s: cursor visibility differs", label)
	}
	sa, ba := a.emu.CursorStyle()
	sb, bb := b.emu.CursorStyle()
	if sa != sb || ba != bb {
		t.Errorf("%s: cursor style %v/%v vs %v/%v", label, sa, ba, sb, bb)
	}
	if a.emu.IsAltScreen() != b.emu.IsAltScreen() {
		t.Errorf("%s: alternate screen differs", label)
	}
	var la, lb []string
	for _, l := range a.emu.Scrollback().Lines() {
		la = append(la, l.Render())
	}
	for _, l := range b.emu.Scrollback().Lines() {
		lb = append(lb, l.Render())
	}
	if !reflect.DeepEqual(la, lb) {
		t.Errorf("%s: scrollback differs:\n%q\n%q", label, la, lb)
	}
	for m, set := range a.modes {
		if !restoredOtherwise[m] && b.modes[m] != set {
			t.Errorf("%s: mode %v is %v in A, %v in B", label, m, set, b.modes[m])
		}
	}
	if a.title != b.title {
		t.Errorf("%s: title %q vs %q", label, a.title, b.title)
	}
}

func roundTrip(t *testing.T, label, stream string) {
	t.Helper()
	a, b := newScreen(t), newScreen(t)
	_, _ = a.Write([]byte(stream))
	_, _ = b.Write(a.Snapshot())
	sameTerminal(t, label, a, b)
}

const (
	colouredHistory = "" +
		"\x1b[1;31mone\x1b[m two \x1b[38;5;214mthree\x1b[m\n" +
		"\x1b[44mblue background\x1b[m\n" +
		"\x1b[38;2;10;200;30mtruecolor\x1b[m \x1b[4munderlined\x1b[m\n" +
		"plain 4\nplain 5\nplain 6\nplain 7\nplain 8\nplain 9\nlast line, no newline"

	// A full-screen program: alternate screen, cursor addressing, a reverse
	// status line, modes, title, hidden bar cursor, wide characters.
	fullScreen = "" +
		"before the app\r\n" +
		"\x1b[?1049h\x1b[2J\x1b[H" +
		"\x1b]2;agent · ledger\x07" +
		"\x1b[1;1H┌──────────┐" +
		"\x1b[2;1H│ \x1b[32m✓\x1b[m tests   │" +
		"\x1b[3;1H│ 日本 → ok │" +
		"\x1b[4;1H└──────────┘" +
		"\x1b[6;1H\x1b[7m status: running        \x1b[m" +
		"\x1b[?1000h\x1b[?1006h\x1b[?2004h\x1b[?1h\x1b[6 q\x1b[?25l" +
		"\x1b[3;14H"

	backFromAlt = fullScreen + "\x1b[?1049l\x1b[?25hback on main\r\n"

	edges = "" +
		"exactly-twenty-four-cols\r\n" + // a full row: pending wrap, then CRLF
		"\x1b[41m    \x1b[m trailing styled spaces\r\n" +
		"\x1b[?7lno wrap: this line is far too long to fit\x1b[?7h\r\n" +
		"combining: é and emoji 👍\r\n"
)

func TestSnapshotRoundTrips(t *testing.T) {
	for label, stream := range map[string]string{
		"empty":            "",
		"coloured history": colouredHistory,
		"full screen":      fullScreen,
		"back from alt":    backFromAlt,
		"edges":            edges,
	} {
		roundTrip(t, label, stream)
	}
}

// A snapshot can be taken between any two chunks, even in the middle of an
// escape sequence or a UTF-8 character: snapshot + the rest of the stream must
// end exactly where the uninterrupted stream does.
func TestSnapshotAtEveryByteOffset(t *testing.T) {
	// No scroll region or saved cursor here: those aren't restored (see the
	// package doc), and the stream after the split would depend on them.
	stream := colouredHistory + "\x1b]2;title\x07" + fullScreen + "\x1b[2;3H\x1b[1;35mpainted\x1b[m" + edges
	skipped := 0
	for k := 0; k <= len(stream); k++ {
		a, b := New(size, func([]byte) {}), New(size, func([]byte) {})
		_, _ = a.Write([]byte(stream[:k]))
		if pendingWrap(a, stream[:k]) {
			// Known limitation (wiki/backlog/snapshot-pending-wrap.md): the emulator
			// doesn't expose a pending line wrap, so the snapshot can't restore it.
			skipped++
			_ = a.Close()
			_ = b.Close()
			continue
		}
		_, _ = b.Write(a.Snapshot())
		_, _ = a.Write([]byte(stream[k:]))
		_, _ = b.Write([]byte(stream[k:]))
		sameTerminal(t, "split at "+strconv.Itoa(k)+" ("+describe(stream, k)+")", a, b)
		_ = a.Close()
		_ = b.Close()
	}
	t.Logf("%d of %d offsets skipped: a pending wrap (exact, see pendingWrap)", skipped, len(stream)+1)
}

func describe(s string, k int) string {
	lo, hi := max(0, k-6), min(len(s), k+6)
	return strings.ReplaceAll(s[lo:k]+"|"+s[k:hi], "\x1b", "ESC")
}

func TestQueriesAreAnswered(t *testing.T) {
	answers := make(chan string, 8)
	s := New(size, func(p []byte) { answers <- string(p) })
	defer s.Close()
	_, _ = s.Write([]byte("abc\x1b[6n"))
	expect := func(prefix string) {
		t.Helper()
		select {
		case got := <-answers:
			if !strings.HasPrefix(got, prefix) {
				t.Errorf("answer %q, want prefix %q", got, prefix)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("no answer (want %q)", prefix)
		}
	}
	expect("\x1b[1;4R") // cursor position report: row 1, column 4
	_, _ = s.Write([]byte("\x1b[c"))
	expect("\x1b[?") // primary device attributes
}

// A session that never reads its input must not stall the screen: answers
// queue up to a bound, then are dropped.
func TestUnreadAnswersNeverBlockTheScreen(t *testing.T) {
	block := make(chan struct{})
	defer close(block)
	s := New(size, func([]byte) { <-block })
	done := make(chan struct{})
	go func() {
		for i := 0; i < answerQueue*4; i++ {
			_, _ = s.Write([]byte("\x1b[6n"))
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("writes blocked behind unread answers")
	}
}

func TestResizeIsReflectedInSnapshots(t *testing.T) {
	a := newScreen(t)
	_, _ = a.Write([]byte("hello"))
	a.Resize(runner.Size{Cols: 40, Rows: 10})
	b := New(runner.Size{Cols: 40, Rows: 10}, func([]byte) {})
	defer b.Close()
	_, _ = b.Write(a.Snapshot())
	sameTerminal(t, "after resize", a, b)
}

// pendingWrap reports whether the terminal is waiting to wrap: the next
// printable character would go to the next line. It replays written into a
// fresh screen and prints one more character to find out exactly.
func pendingWrap(_ *VT, written string) bool {
	probe := New(size, func([]byte) {})
	defer probe.Close()
	_, _ = probe.Write([]byte(written))
	probe.mu.Lock()
	defer probe.mu.Unlock()
	if probe.pend.state != ground || probe.pend.need != 0 || probe.emu.CursorPosition().X != probe.emu.Width()-1 {
		return false
	}
	_, _ = probe.emu.Write([]byte("X"))
	return probe.emu.CursorPosition().X != probe.emu.Width()-1
}
