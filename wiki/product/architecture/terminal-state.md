---
type: spec
status: planned
tags: [architecture, sessions, ui]
---

# Terminal state

Agents' harnesses are full-screen terminal programs: they switch to the alternate screen, move the cursor anywhere, repaint regions, enable mouse tracking and bracketed paste, and ask the terminal questions. Foreman must show them exactly as a real terminal would, live **and** after a reload or a late attach, and must keep them working while **nobody is watching**, which is the normal case for an agent.

Live output is easy: bytes go to xterm.js untouched ([API → PTY WebSocket](./api.md#pty-websocket)). The two hard parts are below, and the answer to both is a **terminal emulator on the server**, one per session, fed every byte the session prints.

## Snapshots instead of byte replay

Replaying the tail of the raw log cannot restore a full-screen program: the tail may start inside an escape sequence, after the switch to the alternate screen, or long after the status line was drawn. Instead, on attach the server sends a **snapshot**: bytes that recreate the current state on a fresh terminal of the same size, then the live output.

A snapshot restores:
- the scrollback (main screen only), then the visible screen with its styles
- the alternate screen when the program is in it
- the cursor position, visibility and style
- the modes the program enabled (mouse tracking, bracketed paste, application cursor keys, focus events…), tracked from the emulator's mode callbacks
- the window title

**Not restored** (accepted): what was on the main screen *underneath* the alternate screen (the emulator doesn't expose the inactive screen), and the scroll region. Both come back as soon as the program redraws, which full-screen programs do on their own.

The raw session log ([Persistence](./persistence.md#session-logs)) stays: it is the history and the audit trail, not the replay source.

## Terminal queries are answered by the server

Programs ask the terminal questions (cursor position, device attributes, status, mode state) and some wait for the answer before going on. In Foreman a session often has no viewer, so a browser-only answer means a harness frozen at startup. So:

- The server's emulator **always** answers queries, and its answers are written to the session as input.
- The browser's xterm.js is told **never** to answer (its handlers for those queries are overridden), so a session with viewers doesn't get two answers.
- The emulator writes its answers to a synchronous pipe: Foreman drains it continuously, otherwise the next query would block the emulator and stall the session.

## Size

The emulator follows the session's terminal size: every resize goes to both the PTY and the emulator, so a snapshot always matches the size the program draws for.

## Design

- The [Terminal](./building-blocks.md#2-terminal-terminal) consumes a `Screen` interface (feed bytes, resize, snapshot, answers to queries). Its implementation wraps [`github.com/charmbracelet/x/vt`](./tech-stack.md#server-go), pinned; nothing else imports the library.
- **Round-trip tests** guard it: feed a byte stream into emulator A, snapshot it, feed the snapshot into a fresh emulator B, and compare the two cell by cell (characters, styles, cursor, alternate screen, modes, scrollback). They run on synthetic streams and on real recordings (`vim`, `htop`, `less`, harness TUIs), and they are what makes upgrading the library safe.
