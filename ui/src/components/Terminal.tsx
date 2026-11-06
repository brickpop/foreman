import { FitAddon } from "@xterm/addon-fit";
import { WebLinksAddon } from "@xterm/addon-web-links";
import { Terminal as XTerm } from "@xterm/xterm";
import "@xterm/xterm/css/xterm.css";
// The terminal's default font, always at hand (the UI uses Geist Mono).
import "@fontsource-variable/jetbrains-mono";
import { type Ref, useEffect, useImperativeHandle, useRef } from "react";
import { useClient } from "../api/client";
import type { PtyConnection } from "../api/pty";
import { usePreferences } from "../app/queries";
import { resolvedColor } from "../app/theme";
import { type Fitter, fitter } from "./terminalFit";
import { loadTerminalFont } from "./terminalFonts";

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

/** The terminal's colors from the tokens, as the page shows them now. */
const colors = () => ({
  background: resolvedColor("var(--term)"),
  foreground: resolvedColor("var(--tx2)"),
  cursor: resolvedColor("var(--acc)"),
  selectionBackground: resolvedColor("color-mix(in srgb, var(--acc) 35%, transparent)"),
});

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
  const fit = useRef<Fitter | null>(null);
  const prefs = usePreferences().data;
  const font = prefs?.terminalFont;
  const fontSize = prefs?.terminalFontSize ?? 13;

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
      fontFamily: '"JetBrains Mono Variable", monospace',
      fontSize: 13,
      lineHeight: 1.25,
      scrollback: 10000,
      cursorBlink: true,
      // When the harness turns on mouse reporting, a drag goes to it, not to
      // the selection: Option-drag (Shift-drag off the Mac) still selects.
      macOptionClickForcesSelection: true,
      theme: colors(),
    });
    xterm.current = term;
    const fitAddon = new FitAddon();
    term.loadAddon(fitAddon);
    term.loadAddon(new WebLinksAddon());
    term.open(host.current);
    fitAddon.fit();
    const fitting = fitter(term, fitAddon);
    fit.current = fitting;

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
      output: (data) =>
        fitting.run(() => {
          if (!replaying) return term.write(data);
          term.write(data, () => {
            term.scrollToBottom();
            show();
          });
        }),
      reset: () =>
        fitting.run(() => {
          replaying = true;
          view.style.opacity = "0";
          clearTimeout(fallback);
          fallback = setTimeout(show, 600);
          term.reset();
        }),
      exit: (code) =>
        fitting.run(() => term.write(`\r\n\x1b[90m[session exited with code ${code}]\x1b[0m\r\n`)),
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
    const observer = new ResizeObserver(() => fitting.fit());
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
      fit.current = null;
    };
  }, [client, instance]);

  // The CEO's font, applied once it has loaded (the cells are measured on it),
  // and again whenever it changes, to every open terminal.
  // biome-ignore lint/correctness/useExhaustiveDependencies: a new terminal (another instance) needs it too
  useEffect(() => {
    let current = true;
    loadTerminalFont(font, fontSize).then((family) => {
      const term = xterm.current;
      if (!current || !term) return;
      term.options.fontFamily = family;
      term.options.fontSize = fontSize;
      fit.current?.fit();
    });
    return () => {
      current = false;
    };
  }, [font, fontSize, instance]);

  // The colors follow the theme and the accent as they change: watched on the
  // page itself, which a preferences effect (Shell's, after this one) or the
  // operating system (System) changes.
  useEffect(() => {
    const paint = () => {
      if (xterm.current) xterm.current.options.theme = colors();
    };
    const page = new MutationObserver(paint);
    page.observe(document.documentElement, { attributeFilter: ["data-theme", "data-accent"] });
    const os = matchMedia("(prefers-color-scheme: dark)");
    os.addEventListener("change", paint);
    return () => {
      page.disconnect();
      os.removeEventListener("change", paint);
    };
  }, []);

  return <div ref={host} className="h-full w-full opacity-0 transition-opacity duration-150" />;
}
