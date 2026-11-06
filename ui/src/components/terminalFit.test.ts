import { Terminal } from "@xterm/xterm";
import { describe, expect, it } from "vitest";
import { cursorForGrow, fitter, scrollForShrink } from "./terminalFit";

const write = (term: Terminal, data: string) => new Promise<void>((done) => term.write(data, done));

/** Every line, scrollback included, trailing blank lines dropped. */
const lines = (term: Terminal) => {
  const b = term.buffer.active;
  const all = Array.from({ length: b.length }, (_, i) => b.getLine(i)?.translateToString(true));
  while (all.length && !all[all.length - 1]) all.pop();
  return all;
};

/** A fitter whose box is `rows` tall; `fitted` resolves on each fit. */
function box(term: Terminal, rows: number) {
  const size = { rows };
  let fitted = () => {};
  const addon = {
    proposeDimensions: () => ({ cols: term.cols, rows: size.rows }),
    fit: () => {
      term.resize(term.cols, size.rows);
      fitted();
    },
  };
  const fitting = fitter(term, addon);
  return {
    fitting,
    resize(rows: number) {
      size.rows = rows;
      const done = new Promise<void>((r) => {
        fitted = r;
      });
      fitting.fit();
      return done;
    },
  };
}

// A harness's screen: its transcript, then a 4-line frame (an input box and a
// footer) with the cursor parked inside the box, as Claude Code keeps it.
const transcript = ["line 1", "line 2", "line 3", "line 4", "line 5", "line 6"];
const frame = "+box+\r\n| > _\r\n+---+\r\nfooter";
const screen = `${transcript.join("\r\n")}\r\n${frame}\x1b[8;6H`;
// Its redraw on a resize: down to the frame's end, up to its start, erase, draw.
const redraw = `\x1b[2B\r\x1b[3A\x1b[J${frame}`;

describe("a shrinking terminal", () => {
  it("loses its top lines into the scrollback, never lines from the middle", async () => {
    const term = new Terminal({ cols: 20, rows: 10 });
    await write(term, screen);
    await box(term, 10).resize(7);
    await write(term, redraw);
    expect(lines(term)).toEqual([...transcript, "+box+", "| > _", "+---+", "footer"]);
    expect(term.buffer.active.baseY).toBe(3); // the top 3 lines scrolled away
  });

  it("is what xterm.js alone gets wrong (the reason for the fitter)", async () => {
    const term = new Terminal({ cols: 20, rows: 10 });
    await write(term, screen);
    term.resize(20, 7);
    await write(term, redraw);
    expect(lines(term)).not.toContain("line 6");
  });

  it("keeps the cursor on its line and column", async () => {
    const term = new Terminal({ cols: 20, rows: 10 });
    await write(term, screen);
    await box(term, 10).resize(7);
    const b = term.buffer.active;
    expect(b.getLine(b.baseY + b.cursorY)?.translateToString(true)).toBe("| > _");
    expect(b.cursorX).toBe(5);
  });

  it("drops blank rows below the cursor first, scrolling nothing away", async () => {
    const term = new Terminal({ cols: 20, rows: 10 });
    await write(term, "a\r\nb\r\n$ ");
    expect(scrollForShrink(term, 7)).toBe("");
    await box(term, 10).resize(7);
    expect(term.buffer.active.baseY).toBe(0);
    expect(lines(term)).toEqual(["a", "b", "$ "]);
  });

  it("scrolls only what the blank rows don't cover", async () => {
    const term = new Terminal({ cols: 20, rows: 10 });
    // Text on rows 0-7, the cursor on row 6, row 7 written, rows 8-9 blank.
    await write(term, "0\r\n1\r\n2\r\n3\r\n4\r\n5\r\n6\r\n7\x1b[7;2H");
    await box(term, 10).resize(6);
    expect(term.buffer.active.baseY).toBe(2);
    expect(lines(term)).toEqual(["0", "1", "2", "3", "4", "5", "6", "7"]);
  });

  it("can't scroll more lines than sit above the cursor", async () => {
    const term = new Terminal({ cols: 20, rows: 10 });
    await write(term, "0\r\n1\r\n2\r\n3\r\n4\r\n5\r\n6\r\n7\r\n8\r\n9\x1b[2;1H");
    expect(scrollForShrink(term, 5)).toBe(`\x1b[10;1H\n\x1b[1;1H`);
  });

  it("leaves a full-screen app (the alternate screen) and a growing box alone", async () => {
    const term = new Terminal({ cols: 20, rows: 10 });
    await write(term, screen);
    expect(scrollForShrink(term, 10)).toBe("");
    expect(scrollForShrink(term, 12)).toBe("");
    await write(term, "\x1b[?1049h0\r\n1\r\n2\r\n3\r\n4\r\n5\r\n6\r\n7\r\n8\r\n9\x1b[2;1H");
    expect(scrollForShrink(term, 5)).toBe("");
  });

  it("holds output that arrives during a fit until after it", async () => {
    const term = new Terminal({ cols: 20, rows: 10 });
    await write(term, screen);
    const { fitting, resize } = box(term, 10);
    const fitted = resize(7);
    // It moves the cursor down a line: put after the fit measured the screen
    // but before its scroll, the scroll would take the cursor back up.
    fitting.run(() => term.write("\x1b[B\rX"));
    await fitted;
    await write(term, "Y");
    expect(lines(term)).toEqual([...transcript, "+box+", "| > _", "XY--+", "footer"]);
  });

  it("fits again when the box changes during a fit", async () => {
    const term = new Terminal({ cols: 20, rows: 10 });
    await write(term, screen);
    const { resize } = box(term, 10);
    resize(8);
    await resize(6); // asked while the first is in flight
    await resize(6); // the re-fit it queued has settled before this one ends
    expect(term.rows).toBe(6);
    expect(lines(term)).toEqual([...transcript, "+box+", "| > _", "+---+", "footer"]);
  });
});

// Claude Code once its view fills the screen (recorded 2026-11-05): on every
// resize it repaints the screen from the top with the last rows of its view,
// counting the blank row its view ends with, and takes all above as being in
// the scrollback. The cursor stays in its input box, 3 rows above the end.
const view = [
  ...Array.from({ length: 30 }, (_, i) => `reply ${i + 1}`),
  "",
  "+box+",
  "| > _",
  "+---+",
  "footer",
  "",
];
const atInput = (rows: number) => `\x1b[${rows - 3};5H`;
const repaint = (rows: number) =>
  `\x1b[H${view.slice(-rows).join("\r\n")}${atInput(rows)}`.replaceAll("\r\n", "\x1b[K\r\n");

describe("a harness repainting from the top", () => {
  const start = async (rows: number) => {
    const term = new Terminal({ cols: 20, rows, scrollback: 1000 });
    await write(term, view.join("\r\n") + atInput(rows));
    return term;
  };

  it("neither loses nor doubles a line however often Line opens and closes", async () => {
    const term = await start(20);
    const { resize } = box(term, 20);
    for (const rows of [14, 20, 12, 20, 17, 20]) {
      await resize(rows);
      await write(term, repaint(rows));
    }
    expect(lines(term)).toEqual(view.slice(0, -1));
  });

  it("is what xterm.js alone gets wrong: lines double", async () => {
    const term = await start(20);
    for (const rows of [14, 20]) {
      term.resize(20, rows);
      await write(term, repaint(rows));
    }
    expect(lines(term).length).toBeGreaterThan(view.length - 1);
  });

  it("sends the blank last row up too, once the screen has filled", async () => {
    const term = await start(20);
    expect(scrollForShrink(term, 14)).toBe(`\x1b[20;1H${"\n".repeat(6)}\x1b[11;5H`);
  });

  it("brings the lines back on a grow, the cursor on its line", async () => {
    const term = await start(20);
    const { resize } = box(term, 20);
    await resize(14);
    await resize(20);
    const b = term.buffer.active;
    expect(b.baseY).toBe(view.length - 20);
    expect(b.getLine(b.baseY + b.cursorY)?.translateToString(true)).toBe("| > _");
    expect(b.cursorX).toBe(4);
  });

  it("has nothing to bring back without a scrollback, nor on the alternate screen", async () => {
    const term = new Terminal({ cols: 20, rows: 10 });
    await write(term, "a\r\nb");
    expect(cursorForGrow(term, 14).before).toBe("");
    const full = await start(20);
    await write(full, "\x1b[?1049h");
    expect(cursorForGrow(full, 24).before).toBe("");
  });
});
