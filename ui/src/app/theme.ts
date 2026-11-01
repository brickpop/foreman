// The theme and the accent (wiki/product/features/settings.md#appearance):
// System follows the operating system live (styles.css, light-dark() over
// color-scheme); Dark and Light set data-theme; the accent sets data-accent.
// Both are the CEO's preferences (GET/PUT /api/preferences); this browser
// keeps a copy only to paint the first frame in them, before the server answers.

export type Theme = "system" | "dark" | "light";

/** The accents styles.css defines, the default first (the server's list too:
 * internal/userdata). */
export const accents = ["blue", "green", "purple", "teal", "orange", "pink", "amber"] as const;

export type Accent = (typeof accents)[number];

const themeKey = "foreman.theme";
const accentKey = "foreman.accent";

/** A stored or served value as a theme: System unless Dark or Light. */
export function asTheme(t: string | null | undefined): Theme {
  return t === "dark" || t === "light" ? t : "system";
}

/** A stored or served value as an accent: the default unless one of them. */
export function asAccent(a: string | null | undefined): Accent {
  return accents.find((x) => x === a) ?? accents[0];
}

/** The copies from the last visit, for the first frame. */
export function savedTheme(): Theme {
  return asTheme(stored(themeKey));
}

export function savedAccent(): Accent {
  return asAccent(stored(accentKey));
}

/** Shows the theme and keeps the copy for the next first frame. */
export function applyTheme(theme: Theme) {
  if (theme === "system") delete document.documentElement.dataset.theme;
  else document.documentElement.dataset.theme = theme;
  store(themeKey, theme === "system" ? null : theme);
}

/** Shows the accent and keeps the copy for the next first frame. */
export function applyAccent(accent: Accent) {
  document.documentElement.dataset.accent = accent;
  store(accentKey, accent);
}

function stored(key: string): string | null {
  try {
    return localStorage.getItem(key);
  } catch {
    return null; // Storage blocked: the defaults until the server answers.
  }
}

function store(key: string, value: string | null) {
  try {
    if (value === null) localStorage.removeItem(key);
    else localStorage.setItem(key, value);
  } catch {
    // Storage blocked: the next visit paints the defaults until the server answers.
  }
}

/** Any CSS color, tokens and light-dark() included, as the page shows it
 * now, in rgba(): for what can't read CSS, as the terminal. */
export function resolvedColor(css: string): string {
  const probe = document.createElement("span");
  probe.style.color = css;
  document.body.append(probe);
  const computed = getComputedStyle(probe).color;
  probe.remove();
  // A canvas turns any syntax the browser computes (color(srgb …) for a
  // color-mix) into plain channels.
  const ctx = document.createElement("canvas").getContext("2d", { willReadFrequently: true });
  if (!ctx) return computed;
  ctx.fillStyle = computed;
  ctx.fillRect(0, 0, 1, 1);
  const [r, g, b, a] = ctx.getImageData(0, 0, 1, 1).data;
  return `rgba(${r}, ${g}, ${b}, ${(a / 255).toFixed(3)})`;
}
