// The few icons the UI draws, inline: no icon library for a handful of shapes.
// They take the text color and size of where they sit.

/** A chevron pointing right; rotated 90° when open. */
export function Chevron({ open }: { open: boolean }) {
  return (
    <svg
      viewBox="0 0 16 16"
      aria-hidden="true"
      className={`size-3 shrink-0 transition-transform ${open ? "rotate-90" : ""}`}
    >
      <path
        d="M6 4l4 4-4 4"
        fill="none"
        stroke="currentColor"
        strokeWidth="1.6"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </svg>
  );
}

/** A file tree: the Files toggle. */
export function TreeIcon() {
  return (
    <svg viewBox="0 0 16 16" aria-hidden="true" className="size-3.5 shrink-0">
      <path
        d="M3 2.5v9.5h3M3 7h3M8 3h5M8 7h5M8 12h5"
        fill="none"
        stroke="currentColor"
        strokeWidth="1.4"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </svg>
  );
}

const line = {
  fill: "none",
  stroke: "currentColor",
  strokeWidth: 1.5,
  strokeLinecap: "round",
  strokeLinejoin: "round",
} as const;

/** Two sheets: Copy. */
export function CopyIcon() {
  return (
    <svg viewBox="0 0 16 16" aria-hidden="true" className="size-4 shrink-0">
      <rect x="5.5" y="5.5" width="8" height="8" rx="1.5" {...line} />
      <path d="M10.5 5.5V3.5a1 1 0 0 0-1-1h-6a1 1 0 0 0-1 1v6a1 1 0 0 0 1 1h2" {...line} />
    </svg>
  );
}

/** A filled square: Stop (a deployment's sessions end). */
export function StopIcon() {
  return (
    <svg viewBox="0 0 16 16" aria-hidden="true" className="size-3 shrink-0">
      <rect x="3" y="3" width="10" height="10" rx="1.5" fill="currentColor" />
    </svg>
  );
}

/** A filled triangle: Start (an asleep or stopped agent's session). */
export function PlayIcon() {
  return (
    <svg viewBox="0 0 16 16" aria-hidden="true" className="size-3.5 shrink-0">
      <path
        d="M5 3.2v9.6a.6.6 0 0 0 .9.5l7.4-4.8a.6.6 0 0 0 0-1L5.9 2.7a.6.6 0 0 0-.9.5z"
        fill="currentColor"
      />
    </svg>
  );
}

/** A circling arrow: Reload (a deployment's agents restart on their new prompt). */
export function ReloadIcon() {
  return (
    <svg viewBox="0 0 16 16" aria-hidden="true" className="size-3.5 shrink-0">
      <path d="M13 8a5 5 0 1 1-1.5-3.6M13 2.5v3h-3" {...line} />
    </svg>
  );
}

/** A tick: done (Copied). */
export function CheckIcon() {
  return (
    <svg viewBox="0 0 16 16" aria-hidden="true" className="size-4 shrink-0">
      <path d="M3.5 8.5l3 3 6-7" {...line} />
    </svg>
  );
}

/** A prompt, `>_`: the Line input. */
export function PromptIcon() {
  return (
    <svg viewBox="0 0 16 16" aria-hidden="true" className="size-4 shrink-0">
      <path d="M3 4.5l3.5 3.5L3 11.5M8.5 12h4.5" {...line} />
    </svg>
  );
}

/** A box with an arrow leaving it: open elsewhere (on the desktop). */
export function OpenIcon({ size = "size-3.5" }: { size?: string }) {
  return (
    <svg viewBox="0 0 16 16" aria-hidden="true" className={`${size} shrink-0`}>
      <path
        d="M7 3.5H4a1 1 0 0 0-1 1v7.5a1 1 0 0 0 1 1h7.5a1 1 0 0 0 1-1V9M9.5 3H13v3.5M13 3L7.5 8.5"
        {...line}
      />
    </svg>
  );
}

/** A folder, its flap raised when open: a file tree's folder row. */
export function FolderIcon({ open, size = "size-3.5" }: { open: boolean; size?: string }) {
  return (
    <svg viewBox="0 0 16 16" aria-hidden="true" className={`${size} shrink-0`}>
      {open ? (
        <path
          d="M2 12.5V4a1 1 0 0 1 1-1h3l1.5 1.5H12a1 1 0 0 1 1 1V7M2 12.5L4 7h10.5l-2 5.5z"
          {...line}
        />
      ) : (
        <path
          d="M2 12V4a1 1 0 0 1 1-1h3l1.5 1.5H13a1 1 0 0 1 1 1V12a1 1 0 0 1-1 1H3a1 1 0 0 1-1-1z"
          {...line}
        />
      )}
    </svg>
  );
}

/** A page with a folded corner: a file tree's file row. */
export function FileIcon() {
  return (
    <svg viewBox="0 0 16 16" aria-hidden="true" className="size-3.5 shrink-0">
      <path
        d="M4 1.5h5.5L12.5 4.5V14a.5.5 0 0 1-.5.5H4a.5.5 0 0 1-.5-.5V2a.5.5 0 0 1 .5-.5zM9.5 1.5v3h3"
        {...line}
      />
    </svg>
  );
}

/** A triangle with a bang: something to fix (an entry whose header couldn't be read). */
export function WarningIcon() {
  return (
    <svg viewBox="0 0 16 16" aria-hidden="true" className="size-3 shrink-0">
      <path d="M8 2 1.5 13.5h13zM8 6.5v3M8 11.5v.01" {...line} />
    </svg>
  );
}

/** A branch forking off: a git repo (the folder browser's repo rows). */
export function RepoIcon({ size = "size-3.5" }: { size?: string }) {
  return (
    <svg viewBox="0 0 16 16" aria-hidden="true" className={`${size} shrink-0`}>
      <circle cx="5" cy="3.5" r="1.5" {...line} />
      <circle cx="5" cy="12.5" r="1.5" {...line} />
      <circle cx="11" cy="5.5" r="1.5" {...line} />
      <path d="M5 5v6M11 7c0 2.5-6 2-6 4" {...line} />
    </svg>
  );
}

/** A folder with a mark inside: one of Foreman's own (its HQ, projects/,
 * worktrees/), in the folder browser. */
export function ForemanFolderIcon({ size = "size-3.5" }: { size?: string }) {
  return (
    <svg viewBox="0 0 16 16" aria-hidden="true" className={`${size} shrink-0`}>
      <path
        d="M2 12V4a1 1 0 0 1 1-1h3l1.5 1.5H13a1 1 0 0 1 1 1V12a1 1 0 0 1-1 1H3a1 1 0 0 1-1-1z"
        {...line}
      />
      <rect x="6.5" y="7" width="3" height="3" rx="0.5" fill="currentColor" />
    </svg>
  );
}

// The rail's icons (wiki/product/features/app-shell.md#the-rail): one
// meaning each, drawn at the rail's size.

/** A terminal window: a project's Session. */
export function SessionIcon({ size = "size-[18px]" }: { size?: string }) {
  return (
    <svg viewBox="0 0 16 16" aria-hidden="true" className={`${size} shrink-0`}>
      <rect x="1.5" y="2.5" width="13" height="11" rx="1.5" {...line} />
      <path d="M4.5 6.5l2 1.5-2 1.5M8 10h3" {...line} />
    </svg>
  );
}

/** A tray: a project's Inbox. */
export function InboxIcon({ size = "size-[18px]" }: { size?: string }) {
  return (
    <svg viewBox="0 0 16 16" aria-hidden="true" className={`${size} shrink-0`}>
      <path
        d="M1.5 9l2-6h9l2 6v3.5a1 1 0 0 1-1 1h-11a1 1 0 0 1-1-1zM1.5 9h3.5l1 1.5h4l1-1.5h3.5"
        {...line}
      />
    </svg>
  );
}

/** Three columns of cards: a project's Board. */
export function BoardIcon({ size = "size-[18px]" }: { size?: string }) {
  return (
    <svg viewBox="0 0 16 16" aria-hidden="true" className={`${size} shrink-0`}>
      <rect x="1.5" y="2.5" width="3.5" height="11" rx="1" {...line} />
      <rect x="6.25" y="2.5" width="3.5" height="7" rx="1" {...line} />
      <rect x="11" y="2.5" width="3.5" height="9" rx="1" {...line} />
    </svg>
  );
}

/** An open book: a project's Wiki. */
export function WikiIcon({ size = "size-[18px]" }: { size?: string }) {
  return (
    <svg viewBox="0 0 16 16" aria-hidden="true" className={`${size} shrink-0`}>
      <path
        d="M8 4.5C6.5 3.2 4.3 3 1.5 3.5v9c2.8-.5 5-.3 6.5 1 1.5-1.3 3.7-1.5 6.5-1v-9C11.7 3 9.5 3.2 8 4.5zM8 4.5v9"
        {...line}
      />
    </svg>
  );
}

/** Two people: a project's Team. */
export function TeamIcon({ size = "size-[18px]" }: { size?: string }) {
  return (
    <svg viewBox="0 0 16 16" aria-hidden="true" className={`${size} shrink-0`}>
      <circle cx="6" cy="5.5" r="2.5" {...line} />
      <path
        d="M1.5 13.5c.4-2.4 2.2-4 4.5-4s4.1 1.6 4.5 4M10.5 3.2a2.5 2.5 0 0 1 0 4.6M12 9.8c1.3.6 2.2 2 2.5 3.7"
        {...line}
      />
    </svg>
  );
}

/** Sliders: a project's settings (never the gear, which is Foreman's). */
export function SlidersIcon({ size = "size-[18px]" }: { size?: string }) {
  return (
    <svg viewBox="0 0 16 16" aria-hidden="true" className={`${size} shrink-0`}>
      <path d="M2 4h12M2 8h12M2 12h12" {...line} />
      <circle cx="10.5" cy="4" r="1.75" fill="currentColor" />
      <circle cx="5" cy="8" r="1.75" fill="currentColor" />
      <circle cx="9" cy="12" r="1.75" fill="currentColor" />
    </svg>
  );
}

/** A gear: Foreman's own settings. */
export function GearIcon({ size = "size-[18px]" }: { size?: string }) {
  return (
    <svg viewBox="0 0 16 16" aria-hidden="true" className={`${size} shrink-0`}>
      <circle cx="8" cy="8" r="2" {...line} />
      <path
        d="M8 1.5v2M8 12.5v2M1.5 8h2M12.5 8h2M3.4 3.4l1.4 1.4M11.2 11.2l1.4 1.4M3.4 12.6l1.4-1.4M11.2 4.8l1.4-1.4"
        {...line}
      />
      <circle cx="8" cy="8" r="4.5" {...line} />
    </svg>
  );
}

/** Three lines: the menu that opens the rail on a narrow screen. */
export function MenuIcon({ size = "size-[18px]" }: { size?: string }) {
  return (
    <svg viewBox="0 0 16 16" aria-hidden="true" className={`${size} shrink-0`}>
      <path d="M2.5 4h11M2.5 8h11M2.5 12h11" {...line} />
    </svg>
  );
}
