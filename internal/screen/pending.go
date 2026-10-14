package screen

// pending tracks the unfinished escape sequence or UTF-8 character at the end
// of the stream written so far. A snapshot taken between two chunks ends with
// these bytes, so a viewer's parser is in the same state as the emulator's
// when the rest of the sequence arrives live.
type pending struct {
	state   int
	need    int    // UTF-8 continuation bytes still expected
	bytes   []byte // since the stream left the ground state
	tooLong bool   // the sequence outgrew maxPending and is no longer replayed
}

const (
	ground    = iota
	escape    // after ESC
	escInter  // ESC followed by intermediate bytes
	csi       // ESC [ … until a final byte
	str       // OSC, DCS, SOS, PM, APC: until BEL or ESC \
	strEscape // ESC inside a string: "\" ends it
)

// maxPending bounds the bytes kept: a sequence longer than this (an inline
// image, say) is rare, and losing it only garbles that one sequence.
const maxPending = 64 * 1024

func (p *pending) write(b []byte) {
	for _, c := range b {
		p.advance(c)
	}
}

func (p *pending) advance(c byte) {
	if p.state == ground && p.need == 0 {
		p.bytes = p.bytes[:0]
		p.tooLong = false
	}
	if !p.tooLong {
		if len(p.bytes) < maxPending {
			p.bytes = append(p.bytes, c)
		} else {
			p.bytes, p.tooLong = p.bytes[:0], true
		}
	}

	if c == 0x18 || c == 0x1a { // CAN, SUB: abort any sequence
		p.state, p.need = ground, 0
		return
	}
	switch p.state {
	case ground:
		switch {
		case p.need > 0 && c&0xc0 == 0x80:
			p.need--
		case c == 0x1b:
			p.state, p.need = escape, 0
		case c&0xe0 == 0xc0:
			p.need = 1
		case c&0xf0 == 0xe0:
			p.need = 2
		case c&0xf8 == 0xf0:
			p.need = 3
		default:
			p.need = 0
		}
	case escape:
		switch {
		case c == '[':
			p.state = csi
		case c == ']' || c == 'P' || c == 'X' || c == '^' || c == '_':
			p.state = str
		case c >= 0x20 && c <= 0x2f:
			p.state = escInter
		case c == 0x1b:
			// a new escape replaces the unfinished one
		default:
			p.state = ground
		}
	case escInter:
		if c >= 0x30 && c <= 0x7e {
			p.state = ground
		}
	case csi:
		if c >= 0x40 && c <= 0x7e {
			p.state = ground
		}
	case str:
		switch c {
		case 0x07:
			p.state = ground
		case 0x1b:
			p.state = strEscape
		}
	case strEscape:
		if c == '\\' {
			p.state = ground
		} else {
			p.state = str
		}
	}
	if p.state == ground && p.need == 0 {
		p.bytes, p.tooLong = p.bytes[:0], false
	}
}

// tail is the unfinished sequence, to append to a snapshot.
func (p *pending) tail() []byte {
	if p.tooLong {
		return nil
	}
	return p.bytes
}
