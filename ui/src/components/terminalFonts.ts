// The terminal's fonts (wiki/product/features/settings.md#appearance), one of
// the CEO's preferences. The ids are the ones the server accepts
// (userdata.TerminalFonts). Geist Mono comes with the app; each other bundled
// font is fetched only once it is picked, so unused ones cost nothing.

export type TerminalFont = {
  id: string;
  label: string;
  family: string;
  load?: () => Promise<unknown>;
};

export const terminalFonts: TerminalFont[] = [
  { id: "geist-mono", label: "Geist Mono", family: '"Geist Mono Variable"' },
  {
    id: "jetbrains-mono",
    label: "JetBrains Mono",
    family: '"JetBrains Mono Variable"',
    load: () => import("@fontsource-variable/jetbrains-mono"),
  },
  {
    id: "ibm-plex-mono",
    label: "IBM Plex Mono",
    family: '"IBM Plex Mono"',
    // Not variable: regular and bold, so bold text isn't faked.
    load: () =>
      Promise.all([
        import("@fontsource/ibm-plex-mono/400.css"),
        import("@fontsource/ibm-plex-mono/700.css"),
      ]),
  },
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

/** The font with that id; Geist Mono for anything else. */
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
