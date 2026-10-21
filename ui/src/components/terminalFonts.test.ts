import { afterEach, describe, expect, it, vi } from "vitest";
import { loadTerminalFont, terminalFont, terminalFontSizes, terminalFonts } from "./terminalFonts";

afterEach(() => vi.unstubAllGlobals());

describe("terminal fonts", () => {
  it("offers exactly the fonts and sizes the server accepts", () => {
    // userdata.TerminalFonts, MinFontSize and MaxFontSize: a font the server
    // refuses would fail to save; one it accepts but this list lacks would
    // show as JetBrains Mono.
    expect(terminalFonts.map((f) => f.id)).toEqual([
      "jetbrains-mono",
      "fira-code",
      "source-code-pro",
      "system",
    ]);
    expect([terminalFontSizes[0], terminalFontSizes.at(-1)]).toEqual([10, 24]);
  });

  it("falls back to JetBrains Mono for an unknown, retired or missing font", () => {
    expect(terminalFont("comic-sans").id).toBe("jetbrains-mono");
    // Offered before 2026-10-20: a preferences.toml may still name them.
    expect(terminalFont("geist-mono").id).toBe("jetbrains-mono");
    expect(terminalFont("ibm-plex-mono").id).toBe("jetbrains-mono");
    expect(terminalFont(undefined).id).toBe("jetbrains-mono");
  });

  it("waits for the glyphs at the size drawn, then names the family", async () => {
    const load = vi.fn(async () => []);
    vi.stubGlobal("document", { fonts: { load } });
    expect(await loadTerminalFont("fira-code", 15)).toBe('"Fira Code Variable", monospace');
    expect(load).toHaveBeenCalledWith('15px "Fira Code Variable"');
  });

  it("still draws, in monospace, when the font can't load", async () => {
    vi.stubGlobal("document", { fonts: { load: () => Promise.reject(new Error("offline")) } });
    expect(await loadTerminalFont("source-code-pro", 13)).toBe(
      '"Source Code Pro Variable", monospace',
    );
  });
});
