// The theme (wiki/product/features/settings.md#appearance): System follows the
// operating system live (styles.css, prefers-color-scheme); Dark and Light set
// data-theme. It is one of the CEO's preferences, the same in every workspace
// (GET/PUT /api/preferences); this browser keeps a copy only to paint the
// first frame in it, before the server answers.

export type Theme = "system" | "dark" | "light";

const key = "foreman.theme";

/** A stored or served value as a theme: System unless Dark or Light. */
export function asTheme(t: string | null | undefined): Theme {
  return t === "dark" || t === "light" ? t : "system";
}

/** The copy from the last visit, for the first frame. */
export function savedTheme(): Theme {
  try {
    return asTheme(localStorage.getItem(key));
  } catch {
    return "system";
  }
}

/** Shows the theme and keeps the copy for the next first frame. */
export function applyTheme(theme: Theme) {
  if (theme === "system") delete document.documentElement.dataset.theme;
  else document.documentElement.dataset.theme = theme;
  try {
    if (theme === "system") localStorage.removeItem(key);
    else localStorage.setItem(key, theme);
  } catch {
    // Storage blocked: the next visit paints System until the server answers.
  }
}
