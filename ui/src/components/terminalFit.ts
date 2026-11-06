import type { FitAddon } from "@xterm/addon-fit";
import type { Terminal } from "@xterm/xterm";

// Fitting the terminal to its box as a native terminal does: a shrink sends
// the screen's top lines into the scrollback, a grow brings them back. A
// harness that repaints the whole screen from its top (Claude Code, once its
// view fills it) then draws over the same lines: nothing lost, nothing doubled.

/**
 * What to write before shrinking to `rows`: the screen scrolled up (the cursor
 * along with its line), leaving blank rows at the bottom. Left alone, xterm.js
 * deletes the rows below the cursor first, blank or not, and the harness's
 * repaint then lost the lines above them for good (found 2026-11-05).
 * Empty when deleting the bottom rows loses nothing.
 */
export function scrollForShrink(term: Terminal, rows: number): string {
  const buffer = term.buffer.active;
  // A full-screen app (the alternate screen) repaints all of it anyway.
  if (buffer.type === "alternate" || rows >= term.rows) return "";
  const blankRow = (y: number) => !buffer.getLine(buffer.baseY + y)?.translateToString(true);
  // Rows blank at the bottom, below the cursor, can go instead, but only on a
  // screen never filled: once it has, a blank last row is the harness's (Claude
  // Code's ends its view), its repaint counts it, and keeping it made every
  // shrink and grow drift one line (found 2026-11-05).
  let blank = 0;
  while (
    buffer.baseY === 0 &&
    term.rows - 1 - blank > buffer.cursorY &&
    blankRow(term.rows - 1 - blank)
  )
    blank++;
  const lines = Math.min(term.rows - rows - blank, buffer.cursorY);
  if (lines <= 0) return "";
  return `\x1b[${term.rows};1H${"\n".repeat(lines)}${cursorAt(buffer.cursorY - lines, buffer.cursorX)}`;
}

/**
 * What to write before growing to `rows`: the cursor on the last row, where
 * xterm.js brings scrollback lines back down as it grows (elsewhere it adds
 * blank rows at the bottom, and the harness's repaint doubled the lines a
 * shrink had sent up, found 2026-11-05). `back` puts the cursor back on its
 * line once grown. Empty when there is nothing to bring back.
 */
export function cursorForGrow(term: Terminal, rows: number): { before: string; back(): string } {
  const buffer = term.buffer.active;
  const { cursorX, cursorY, baseY } = buffer;
  if (buffer.type === "alternate" || rows <= term.rows || baseY === 0 || cursorY === term.rows - 1)
    return { before: "", back: () => "" };
  return {
    before: cursorAt(term.rows - 1, cursorX),
    back: () => cursorAt(cursorY + baseY - term.buffer.active.baseY, cursorX),
  };
}

const cursorAt = (y: number, x: number) => `\x1b[${y + 1};${x + 1}H`;

export type Fitter = {
  /** Fits the terminal to its box now (or after the fit in flight). */
  fit(): void;
  /** Runs what changes the screen (output, a reset) in order with the fits:
   * held while one is in flight, which must see the screen it changes. */
  run(change: () => void): void;
};

export function fitter(term: Terminal, addon: Pick<FitAddon, "proposeDimensions" | "fit">): Fitter {
  let held: (() => void)[] | null = null;
  let again = false;
  const refit = () => {
    if (held) {
      again = true;
      return;
    }
    held = [];
    // The callback runs once all output so far is on the screen.
    term.write("", () => {
      const rows = addon.proposeDimensions()?.rows ?? term.rows;
      const grow = cursorForGrow(term, rows);
      term.write(scrollForShrink(term, rows) + grow.before, () => {
        addon.fit();
        term.write(grow.back(), () => {
          const changes = held ?? [];
          held = null;
          for (const change of changes) change();
          if (again) {
            again = false;
            refit();
          }
        });
      });
    });
  };
  return {
    fit: refit,
    run(change) {
      if (held) held.push(change);
      else change();
    },
  };
}
