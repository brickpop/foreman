// Client side of the PTY WebSocket (wiki/product/architecture/api.md#pty-websocket):
// binary frames carry raw terminal bytes both ways; text frames carry JSON
// control messages ({"type":"resize"} up, {"type":"exit"} down).

export interface PtyHandlers {
  /** Raw output to write to the terminal. */
  output(data: Uint8Array): void;
  /** The connection is about to replay the session from the start: clear the screen. */
  reset(): void;
  /** The session exited; no reconnect follows. */
  exit(code: number): void;
}

export interface PtyConnection {
  /** Keystrokes or pasted text, as typed. */
  send(input: string): void;
  /** The viewer's size; resent after every reconnect. */
  resize(cols: number, rows: number): void;
  /** Detach for good. */
  close(): void;
}

/** The subset of WebSocket the client uses, so tests can provide their own. */
export interface SocketLike {
  binaryType: string;
  readyState: number;
  onopen: (() => void) | null;
  onmessage: ((e: { data: unknown }) => void) | null;
  onclose: ((e: { code: number }) => void) | null;
  send(data: string | Uint8Array): void;
  close(): void;
}

export type SocketFactory = (url: string) => SocketLike;

const OPEN = 1;
/** The server closes with this code when the viewer fell behind: reconnect at once. */
const FELL_BEHIND = 1013;
const MAX_BACKOFF_MS = 5000;

export function openPty(
  url: string,
  handlers: PtyHandlers,
  makeSocket: SocketFactory = (u) => new WebSocket(u) as unknown as SocketLike,
  schedule: (fn: () => void, ms: number) => void = (fn, ms) => setTimeout(fn, ms),
): PtyConnection {
  const encoder = new TextEncoder();
  let socket: SocketLike;
  let size: { cols: number; rows: number } | undefined;
  let closed = false; // by the viewer
  let exited = false; // by the session
  let backoff = 250;

  const sendSize = () => {
    if (size && socket.readyState === OPEN) {
      socket.send(JSON.stringify({ type: "resize", ...size }));
    }
  };

  const connect = () => {
    socket = makeSocket(url);
    socket.binaryType = "arraybuffer";
    let replaying = true; // the first frames of every connection are the replay
    socket.onopen = () => {
      backoff = 250;
      sendSize();
    };
    socket.onmessage = (e) => {
      if (e.data instanceof ArrayBuffer) {
        if (replaying) {
          handlers.reset();
          replaying = false;
        }
        handlers.output(new Uint8Array(e.data));
        return;
      }
      try {
        const msg = JSON.parse(String(e.data));
        if (msg.type === "exit" && typeof msg.code === "number") {
          exited = true;
          handlers.exit(msg.code);
        }
      } catch {
        // Unknown control messages are ignored, for forward compatibility.
      }
    };
    socket.onclose = (e) => {
      if (closed || exited) return;
      // Fell behind: reconnect now and replay. Anything else (server restart,
      // network): retry with backoff.
      const delay = e.code === FELL_BEHIND ? 0 : backoff;
      backoff = Math.min(backoff * 2, MAX_BACKOFF_MS);
      schedule(connect, delay);
    };
  };

  connect();
  return {
    send(input) {
      if (socket.readyState === OPEN) socket.send(encoder.encode(input));
    },
    resize(cols, rows) {
      size = { cols, rows };
      sendSize();
    },
    close() {
      closed = true;
      socket.close();
    },
  };
}
