// The terminal's fonts (wiki/product/features/settings.md#appearance), one of
// this machine's settings. The ids are the ones the server accepts
// (home.TerminalFonts). They are the terminal's only: the rest of the UI
// keeps Geist Mono (the CEO, 2026-10-21). JetBrains Mono, the default, comes
// with the terminal; each other font is fetched only once it is picked, so
// unused ones cost nothing.

export type TerminalFont = {
  id: string;
  label: string;
  family: string;
  load?: () => Promise<unknown>;
};

export const terminalFonts: TerminalFont[] = [
  { id: "jetbrains-mono", label: "JetBrains Mono", family: '"JetBrains Mono Variable"' },
  {
    id: "fira-code",
    label: "Fira Code",
    family: '"Fira Code Variable"',
    load: () => import("@fontsource-variable/fira-code"),
  },
  {
    id: "source-code-pro",
    label: "Source Code Pro",
    family: '"Source Code Pro Variable"',
    load: () => import("@fontsource-variable/source-code-pro"),
  },
  {
    id: "system",
    label: "System monospace",
    family: 'ui-monospace, "SF Mono", Menlo, Consolas, "DejaVu Sans Mono"',
  },
];

/** The font with that id; JetBrains Mono for anything else. */
export function terminalFont(id: string | undefined): TerminalFont {
  return terminalFonts.find((f) => f.id === id) ?? terminalFonts[0];
}

/** The font sizes offered, in px (the server's MinFontSize to MaxFontSize). */
export const terminalFontSizes = Array.from({ length: 15 }, (_, i) => 10 + i);

/** The CSS font-family to draw with, once its glyphs are there: the terminal
 * measures its cells on the font it has, so it must not switch before. */
export async function loadTerminalFont(id: string | undefined, size: number): Promise<string> {
  const font = terminalFont(id);
  try {
    await font.load?.();
    await document.fonts?.load(`${size}px ${font.family}`);
  } catch {
    // Not loaded: the browser falls back to monospace, which still works.
  }
  return `${font.family}, monospace`;
}
