import { afterEach, describe, expect, it, vi } from "vitest";
import { loadTerminalFont, terminalFont, terminalFontSizes, terminalFonts } from "./terminalFonts";

afterEach(() => vi.unstubAllGlobals());

describe("terminal fonts", () => {
  it("offers exactly the fonts and sizes the server accepts", () => {
    // userdata.TerminalFonts, MinFontSize and MaxFontSize: a font the server
    // refuses would fail to save; one it accepts but this list lacks would
    // show as Geist Mono.
    expect(terminalFonts.map((f) => f.id)).toEqual([
      "geist-mono",
      "jetbrains-mono",
      "ibm-plex-mono",
      "fira-code",
      "source-code-pro",
      "system",
    ]);
    expect([terminalFontSizes[0], terminalFontSizes.at(-1)]).toEqual([10, 24]);
  });

  it("falls back to Geist Mono for an unknown or missing font", () => {
    expect(terminalFont("comic-sans").id).toBe("geist-mono");
    expect(terminalFont(undefined).id).toBe("geist-mono");
  });

  it("waits for the glyphs at the size drawn, then names the family", async () => {
    const load = vi.fn(async () => []);
    vi.stubGlobal("document", { fonts: { load } });
    expect(await loadTerminalFont("ibm-plex-mono", 15)).toBe('"IBM Plex Mono", monospace');
    expect(load).toHaveBeenCalledWith('15px "IBM Plex Mono"');
  });

  it("still draws, in monospace, when the font can't load", async () => {
    vi.stubGlobal("document", { fonts: { load: () => Promise.reject(new Error("offline")) } });
    expect(await loadTerminalFont("jetbrains-mono", 13)).toBe(
      '"JetBrains Mono Variable", monospace',
    );
  });
});
