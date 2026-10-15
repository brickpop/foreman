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
};

const token = (name: string) =>
  getComputedStyle(document.documentElement).getPropertyValue(name).trim();

export function Terminal({ instance, ref }: { instance: string; ref?: Ref<TerminalHandle> }) {
  const client = useClient();
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

    const conn = client.agents.openTerminal(instance, {
      output: (data) => term.write(data),
      reset: () => term.reset(),
      exit: (code) => term.write(`\r\n\x1b[90m[session exited with code ${code}]\x1b[0m\r\n`),
    });
    connection.current = conn;
    conn.resize(term.cols, term.rows);
    const typing = term.onData((data) => conn.send(data));
    const resizing = term.onResize(({ cols, rows }) => conn.resize(cols, rows));
    const observer = new ResizeObserver(() => fit.fit());
    observer.observe(host.current);

    return () => {
      observer.disconnect();
      typing.dispose();
      resizing.dispose();
      conn.close();
      connection.current = null;
      term.dispose();
      xterm.current = null;
    };
  }, [client, instance]);

  return <div ref={host} className="h-full w-full" />;
}
