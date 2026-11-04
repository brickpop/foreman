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

/** A folder with a mark inside: one of Foreman's own (the projects folder, a
 * workspace), in the folder browser. */
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
