import { FitAddon } from "@xterm/addon-fit";
import { WebLinksAddon } from "@xterm/addon-web-links";
import { Terminal as XTerm } from "@xterm/xterm";
import "@xterm/xterm/css/xterm.css";
import { type Ref, useEffect, useImperativeHandle, useRef } from "react";
import { useClient } from "../api/client";
import type { PtyConnection } from "../api/pty";

// Our terminal component. Screens use this, never xterm.js directly, so the
// emulator can be replaced in one file (wiki/product/architecture/overview.md#in-the-ui).

export type TerminalHandle = {
  /** The whole buffer (scrollback included) as plain text, for Copy. */
  text(): string;
  /** Types text into the session, as the keyboard would (line input). */
  send(text: string): void;
  /** Puts the keyboard in the terminal. */
  focus(): void;
};

const token = (name: string) =>
  getComputedStyle(document.documentElement).getPropertyValue(name).trim();

export function Terminal({
  instance,
  newline,
  ref,
}: {
  instance: string;
  /** The harness's keystroke for a new line without sending: what Shift+Enter types. */
  newline?: string;
  ref?: Ref<TerminalHandle>;
}) {
  const client = useClient();
  const newlineKey = useRef(newline);
  newlineKey.current = newline;
  const host = useRef<HTMLDivElement>(null);
  const xterm = useRef<XTerm | null>(null);
  const connection = useRef<PtyConnection | null>(null);

  useImperativeHandle(ref, () => ({
    text() {
      const buffer = xterm.current?.buffer.active;
      if (!buffer) return "";
      const lines: string[] = [];
      for (let i = 0; i < buffer.length; i++) {
        lines.push(buffer.getLine(i)?.translateToString(true) ?? "");
      }
      return lines.join("\n").trimEnd();
    },
    send(text) {
      connection.current?.send(text);
    },
    focus() {
      xterm.current?.focus();
    },
  }));

  useEffect(() => {
    if (!host.current) return;
    const term = new XTerm({
      fontFamily: '"Geist Mono Variable", monospace',
      fontSize: 13,
      lineHeight: 1.25,
      scrollback: 10000,
      cursorBlink: true,
      theme: {
        background: token("--term"),
        foreground: token("--tx2"),
        cursor: token("--acc"),
        selectionBackground: "rgba(232, 163, 61, 0.35)",
      },
    });
    xterm.current = term;
    const fit = new FitAddon();
    term.loadAddon(fit);
    term.loadAddon(new WebLinksAddon());
    term.open(host.current);
    fit.fit();

    // The replay (the screen and its scrollback, sent first on every connect)
    // is drawn hidden, then shown at once: no flash of it scrolling by.
    const view = host.current;
    let replaying = true;
    const show = () => {
      replaying = false;
      view.style.opacity = "1";
    };
    let fallback = setTimeout(show, 600); // nothing came: show what there is
    const conn = client.agents.openTerminal(instance, {
      output: (data) => {
        if (!replaying) return term.write(data);
        term.write(data, () => {
          term.scrollToBottom();
          show();
        });
      },
      reset: () => {
        replaying = true;
        view.style.opacity = "0";
        clearTimeout(fallback);
        fallback = setTimeout(show, 600);
        term.reset();
      },
      exit: (code) => term.write(`\r\n\x1b[90m[session exited with code ${code}]\x1b[0m\r\n`),
    });
    connection.current = conn;
    conn.resize(term.cols, term.rows);
    const typing = term.onData((data) => conn.send(data));
    // A terminal can't tell Shift+Enter from Enter: type the harness's newline instead.
    term.attachCustomKeyEventHandler((e) => {
      const key = newlineKey.current;
      if (!key || e.key !== "Enter" || !e.shiftKey || e.altKey || e.ctrlKey || e.metaKey)
        return true;
      if (e.type === "keydown") conn.send(key);
      return false;
    });
    const resizing = term.onResize(({ cols, rows }) => conn.resize(cols, rows));
    const observer = new ResizeObserver(() => fit.fit());
    observer.observe(host.current);

    return () => {
      clearTimeout(fallback);
      observer.disconnect();
      typing.dispose();
      resizing.dispose();
      conn.close();
      connection.current = null;
      term.dispose();
      xterm.current = null;
    };
  }, [client, instance]);

  return <div ref={host} className="h-full w-full opacity-0 transition-opacity duration-150" />;
}
