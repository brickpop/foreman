import { describe, expect, it } from "vitest";
import { accents } from "./app/theme";
import css from "./styles.css?raw";

// The palettes in styles.css stay readable: each theme's text on its page,
// each accent's text on the page and its ink on its fill (WCAG AA, 4.5:1).
// Hand-tuned values, so a later tweak is checked here, not by eye.

/** A block's tokens as [light, dark] (a plain value is both). */
function tokens(selector: string): Record<string, [string, string]> {
  const at = css.indexOf(`${selector} {`);
  if (at < 0) throw new Error(`no ${selector} block`);
  const body = css.slice(at, css.indexOf("}", at));
  const out: Record<string, [string, string]> = {};
  for (const [, name, value] of body.matchAll(/--(\w+):\s*([^;]+);/g)) {
    const pair = value.match(/^light-dark\((#\w{6}),\s*(#\w{6})\)$/);
    if (pair) out[name] = [pair[1], pair[2]];
    else if (/^#\w{6}$/.test(value)) out[name] = [value, value];
  }
  return out;
}

function luminance(hex: string): number {
  const [r, g, b] = [1, 3, 5].map((i) => {
    const c = Number.parseInt(hex.slice(i, i + 2), 16) / 255;
    return c <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4;
  });
  return 0.2126 * r + 0.7152 * g + 0.0722 * b;
}

function contrast(a: string, b: string): number {
  const [hi, lo] = [luminance(a), luminance(b)].sort((x, y) => y - x);
  return (hi + 0.05) / (lo + 0.05);
}

const root = tokens(":root");
const blue = tokens(':root,\n[data-accent="blue"]');
const themes = [
  ["light", 0],
  ["dark", 1],
] as const;

describe("the palettes", () => {
  it("defines a block for each accent the settings offer, and no other", () => {
    const defined = [...css.matchAll(/\[data-accent="(\w+)"\]/g)].map((m) => m[1]);
    expect(defined.sort()).toEqual([...accents].sort());
  });

  for (const [theme, i] of themes) {
    it(`keeps ${theme}'s text readable on its page`, () => {
      for (const name of ["tx", "tx2", "tx3", "tx4", "ok", "info", "team", "err", "attnTx"]) {
        expect(contrast(root[name][i], root.bg[i]), name).toBeGreaterThanOrEqual(4.5);
      }
      expect(contrast(root.attnInk[i], root.attn[i]), "attnInk").toBeGreaterThanOrEqual(4.5);
      // A card's header band: its name and its status pill's word.
      for (const name of ["tx", "tx2"]) {
        expect(contrast(root[name][i], root.head[i]), `${name} on head`).toBeGreaterThanOrEqual(
          4.5,
        );
      }
    });

    it(`keeps every accent readable in ${theme}`, () => {
      for (const accent of accents) {
        const own = accent === "blue" ? blue : { ...blue, ...tokens(`[data-accent="${accent}"]`) };
        expect(contrast(own.accTx[i], root.bg[i]), `${accent} text`).toBeGreaterThanOrEqual(4.5);
        expect(contrast(own.accInk[i], own.acc[i]), `${accent} ink`).toBeGreaterThanOrEqual(4.5);
      }
    });
  }
});
