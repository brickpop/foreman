package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/coder/websocket"

	"github.com/brickpop/foreman/internal/runner"
)

// Terminals gives access to running sessions' terminals (implemented by the
// supervisor, through an adapter in cmd/foreman).
type Terminals interface {
	Attach(instance string) (Terminal, error)
	// List returns the ids of the instances that have a session, sorted.
	List() []string
}

// Terminal is one viewer's attachment to a session.
type Terminal interface {
	Replay() []byte        // recent output, sent first
	Output() <-chan []byte // live output; closed on exit, or when the viewer fell behind
	Exit() (int, bool)     // exit code, once exited
	Input(p []byte) error  // bytes typed by the CEO
	Resize(runner.Size) error
	Close()
}

// ErrNoTerminal is returned by Terminals.Attach for an instance without a session.
var ErrNoTerminal = errors.New("no such terminal")

// Close codes and control messages of the PTY WebSocket
// (wiki/product/architecture/api.md#pty-websocket).
const (
	// closeBehind tells the client it fell behind: reconnect to get a fresh replay.
	closeBehind = websocket.StatusTryAgainLater
	// maxInput bounds one input message (a large paste).
	maxInput = 1 << 20
)

type control struct {
	Type string `json:"type"`           // "resize" from the client; "exit" from the server
	Cols uint16 `json:"cols,omitempty"` // resize
	Rows uint16 `json:"rows,omitempty"` // resize
	Code *int   `json:"code,omitempty"` // exit
}

// handlePTY streams one session's terminal: binary frames carry raw bytes in
// both directions; text frames carry JSON control messages.
func handlePTY(terms Terminals) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		term, err := terms.Attach(r.PathValue("instance"))
		if errors.Is(err, ErrNoTerminal) {
			http.Error(w, "no such session", http.StatusNotFound)
			return
		}
		if err != nil {
			http.Error(w, "couldn't attach to the session", http.StatusInternalServerError)
			return
		}
		defer term.Close()
		// The access guard already checked Origin; Accept checks it again.
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.CloseNow()
		conn.SetReadLimit(maxInput)

		ctx, cancel := context.WithCancel(r.Context())
		defer cancel()
		go readInput(ctx, cancel, conn, term)

		if err := writeChunks(ctx, conn, term.Replay()); err != nil {
			return
		}
		// Watch the client too: a quiet session must not keep a gone viewer attached.
	live:
		for {
			select {
			case <-ctx.Done():
				return
			case chunk, ok := <-term.Output():
				if !ok {
					break live
				}
				if err := conn.Write(ctx, websocket.MessageBinary, chunk); err != nil {
					return
				}
			}
		}
		code, exited := term.Exit()
		if !exited {
			_ = conn.Close(closeBehind, "fell behind; reconnect")
			return
		}
		msg, _ := json.Marshal(control{Type: "exit", Code: &code})
		_ = conn.Write(ctx, websocket.MessageText, msg)
		_ = conn.Close(websocket.StatusNormalClosure, "session exited")
	}
}

// writeChunks sends the replay in frames of a bounded size.
func writeChunks(ctx context.Context, conn *websocket.Conn, data []byte) error {
	const frame = 64 * 1024
	for len(data) > 0 {
		n := min(frame, len(data))
		if err := conn.Write(ctx, websocket.MessageBinary, data[:n]); err != nil {
			return err
		}
		data = data[n:]
	}
	return nil
}

// readInput forwards the client's keystrokes and resizes until it disconnects.
func readInput(ctx context.Context, cancel context.CancelFunc, conn *websocket.Conn, term Terminal) {
	defer cancel()
	for {
		kind, data, err := conn.Read(ctx)
		if err != nil {
			return
		}
		switch kind {
		case websocket.MessageBinary:
			_ = term.Input(data) // after the exit, input goes nowhere; the exit frame says why
		case websocket.MessageText:
			var c control
			if json.Unmarshal(data, &c) == nil && c.Type == "resize" && c.Cols > 0 && c.Rows > 0 {
				_ = term.Resize(runner.Size{Cols: c.Cols, Rows: c.Rows})
			}
		}
	}
}
