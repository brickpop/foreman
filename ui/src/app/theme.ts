// The theme (wiki/product/features/settings.md#appearance): System follows the
// operating system live (styles.css, prefers-color-scheme); Dark and Light set
// data-theme. Kept in this browser until UI state is stored on the server
// (wiki/technical-debt.md).

export type Theme = "system" | "dark" | "light";

const key = "foreman.theme";

export function savedTheme(): Theme {
  try {
    const t = localStorage.getItem(key);
    return t === "dark" || t === "light" ? t : "system";
  } catch {
    return "system";
  }
}

export function applyTheme(theme: Theme) {
  if (theme === "system") delete document.documentElement.dataset.theme;
  else document.documentElement.dataset.theme = theme;
  try {
    if (theme === "system") localStorage.removeItem(key);
    else localStorage.setItem(key, theme);
  } catch {
    // Storage blocked: the theme holds for this visit.
  }
}
