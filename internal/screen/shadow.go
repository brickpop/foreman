package screen

import (
	"fmt"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/vt"
)

// shadow mirrors the parts of the emulator's state that it keeps private but a
// snapshot must restore: the pen (the style of the next characters), the
// scroll region and the saved cursor, per screen (main and alternate). It is
// fed by handlers chained in front of the emulator's own (they return false,
// so the emulator still handles every sequence) and parses with the same
// library functions, so it can't disagree with the emulator.
type shadow struct {
	alt    bool
	pen    [2]uv.Style
	region [2][2]int // 1-based top and bottom margins; zero means the whole screen
	saved  [2]savedCursor
}

type savedCursor struct {
	set bool
	pos uv.Position
	pen uv.Style
}

func (sh *shadow) cur() int {
	if sh.alt {
		return 1
	}
	return 0
}

func (sh *shadow) attach(e *vt.Emulator) {
	e.RegisterCsiHandler('m', func(p ansi.Params) bool { // SGR
		uv.ReadStyle(p, &sh.pen[sh.cur()])
		return false
	})
	e.RegisterCsiHandler('r', func(p ansi.Params) bool { // DECSTBM, as the emulator reads it
		h := e.Height()
		top, _, _ := p.Param(0, 1)
		bottom, _, _ := p.Param(1, h)
		top, bottom = max(top, 1), min(bottom, h)
		if bottom < 1 {
			bottom = h
		}
		if top < bottom {
			if top == 1 && bottom == h {
				sh.region[sh.cur()] = [2]int{}
			} else {
				sh.region[sh.cur()] = [2]int{top, bottom}
			}
		}
		return false
	})
	e.RegisterEscHandler('7', func() bool { // DECSC
		sh.saved[sh.cur()] = savedCursor{set: true, pos: e.CursorPosition(), pen: sh.pen[sh.cur()]}
		return false
	})
	e.RegisterEscHandler('8', func() bool { // DECRC
		if s := sh.saved[sh.cur()]; s.set {
			sh.pen[sh.cur()] = s.pen
		}
		return false
	})
	e.RegisterEscHandler('c', func() bool { // RIS
		*sh = shadow{}
		return false
	})
	e.RegisterCsiHandler(ansi.Command(0, '!', 'p'), func(ansi.Params) bool { // DECSTR
		sh.pen[sh.cur()] = uv.Style{}
		sh.region[sh.cur()] = [2]int{}
		return false
	})
}

// altScreen follows the switch: the emulator copies the main cursor, pen
// included, into the alternate screen when entering it.
func (sh *shadow) altScreen(on bool) {
	if on && !sh.alt {
		sh.pen[1] = sh.pen[0]
	}
	sh.alt = on
}

// resized: the emulator resets the scroll regions to the new size.
func (sh *shadow) resized() { sh.region = [2][2]int{} }

// restoreRegion is the DECSTBM that recreates the scroll region (it homes the
// cursor, so it is emitted before the cursor position).
func (sh *shadow) restoreRegion() string {
	r := sh.region[sh.cur()]
	if r == ([2]int{}) {
		return ""
	}
	return fmt.Sprintf("\x1b[%d;%dr", r[0], r[1])
}

// restoreSaved recreates the saved cursor (DECSC): move there with its pen,
// save, and let the caller restore the current position and pen afterwards.
func (sh *shadow) restoreSaved() string {
	s := sh.saved[sh.cur()]
	if !s.set {
		return ""
	}
	return ansi.CursorPosition(s.pos.X+1, s.pos.Y+1) + ansi.ResetStyle + penSGR(s.pen) + "\x1b7"
}

// restorePen sets the pen for the characters that follow.
func (sh *shadow) restorePen() string { return ansi.ResetStyle + penSGR(sh.pen[sh.cur()]) }

func penSGR(s uv.Style) string {
	if s.IsZero() {
		return ""
	}
	return s.String()
}
