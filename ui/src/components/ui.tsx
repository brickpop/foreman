import * as ToggleGroup from "@radix-ui/react-toggle-group";
import { type ButtonHTMLAttributes, type ReactNode, useState } from "react";
import { saysLead } from "../app/model";
import { CheckIcon, CopyIcon } from "./icons";

// The design system's small building blocks (wiki/product/design/design-system.md):
// screens use these, never raw styles for the same thing.

const buttonVariants = {
  primary: "border border-acc bg-acc font-semibold text-accInk hover:brightness-105",
  outline: "border border-acc bg-transparent font-semibold text-accTx hover:bg-accSoft",
  secondary: "border border-line2 bg-transparent font-medium text-tx2 hover:border-line3",
  raised: "border border-line2 bg-raised font-medium text-tx hover:border-line3",
  muted: "border border-line2 bg-transparent font-medium text-tx3 hover:border-line3",
  danger:
    "border border-line2 bg-transparent font-medium text-err hover:border-err hover:bg-err/10",
  ghost: "border border-transparent bg-transparent font-medium text-tx3 hover:text-tx2",
  small: "border border-line2 bg-transparent font-medium text-tx2 hover:border-line3",
  tiny: "border border-line2 bg-transparent text-tx3 hover:border-line3",
} as const;

// Every other variant has the standard size.
const buttonSizes: Partial<Record<keyof typeof buttonVariants, string>> = {
  small: "rounded-[6px] px-2.5 py-1 text-[12.5px]",
  // A view's main action, with its icon before the label.
  outline: "inline-flex items-center gap-2 rounded-[7px] px-4 py-2 text-[13px]",
  tiny: "rounded-[5px] px-2 py-0.5 text-xs",
};

export type ButtonProps = ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: keyof typeof buttonVariants;
};

/** A button; disabled is real (not focusable as an action, dimmed). */
export function Button({ variant = "secondary", className = "", ...props }: ButtonProps) {
  const size = buttonSizes[variant] ?? "rounded-[7px] px-3.5 py-[7px] text-[13px]";
  return (
    <button
      type="button"
      className={`${size} focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-acc disabled:opacity-45 ${buttonVariants[variant]} ${className}`}
      {...props}
    />
  );
}

/** A card (Dashboard and Repos grids, Settings sections): a header band
 * holding its name and status or controls, over its body; no body while
 * collapsed. */
export function Card({
  head,
  opens = false,
  danger = false,
  children,
}: {
  head: ReactNode;
  /** Whether it opens something (its title link covering it): then it shows hover. */
  opens?: boolean;
  /** Irreversible actions inside: its border in --err. */
  danger?: boolean;
  children?: ReactNode;
}) {
  return (
    <article
      className={`relative flex min-w-0 flex-col overflow-hidden rounded-xl border bg-panel ${danger ? "border-err/50" : "border-line"} ${opens ? "hover:border-line3" : ""}`}
    >
      <header
        className={`flex min-h-[44px] items-center gap-2 bg-head px-4 py-2 ${children ? "border-b border-line" : ""}`}
      >
        {head}
      </header>
      {children && <div className="flex flex-1 flex-col gap-3 px-4 py-3.5">{children}</div>}
    </article>
  );
}

/** A group of settings (Settings, Project settings): a card titled in its
 * header band, its rows one under another, a line between them. A collapsible
 * one (reference text) starts closed. */
export function GroupCard({
  title,
  aside,
  collapsible = false,
  danger = false,
  children,
}: {
  title: string;
  /** Where its settings are kept or come from, after the title. */
  aside?: string;
  collapsible?: boolean;
  danger?: boolean;
  children: ReactNode;
}) {
  const [open, setOpen] = useState(!collapsible);
  const label = (
    <>
      <h2 className={`m-0 text-[14px] font-semibold ${danger ? "text-err" : ""}`}>{title}</h2>
      {aside && <span className="text-[12.5px] text-tx4">{aside}</span>}
    </>
  );
  return (
    <Card
      danger={danger}
      head={
        collapsible ? (
          <button
            type="button"
            aria-expanded={open}
            onClick={() => setOpen(!open)}
            className="-mx-4 -my-2 flex flex-1 items-center gap-2 self-stretch px-4 py-2 text-left"
          >
            <span className={`text-tx4 transition-transform ${open ? "rotate-90" : ""}`}>›</span>
            {label}
          </button>
        ) : (
          label
        )
      }
    >
      {open && (
        <div className="flex flex-col divide-y divide-line [&>*]:py-4 [&>*:first-child]:pt-0.5 [&>*:last-child]:pb-0.5">
          {children}
        </div>
      )}
    </Card>
  );
}

/** A status in a card's header: its dot and word on a tint of its color. */
export function StatusPill({ color, children }: { color: string; children: ReactNode }) {
  return (
    <span
      className="flex shrink-0 items-center gap-1.5 rounded-full px-2 py-0.5 text-[12px] text-tx2"
      style={{ background: `color-mix(in srgb, ${color} 16%, transparent)` }}
    >
      <Dot color={color} />
      {children}
    </span>
  );
}

/** A card's title link covering the whole card; its other controls sit above it. */
export const cover = "after:absolute after:inset-0 after:rounded-xl after:content-['']";

/** One choice of a segmented control (board switcher, theme…). */
export function Segmented<T extends string>({
  options,
  value,
  onChange,
  label,
}: {
  options: { value: T; label: ReactNode; disabled?: boolean }[];
  value: T;
  onChange: (value: T) => void;
  label: string;
}) {
  return (
    <ToggleGroup.Root
      type="single"
      value={value}
      onValueChange={(v) => v && onChange(v as T)}
      aria-label={label}
      className="inline-flex gap-1"
    >
      {options.map((o) => (
        <ToggleGroup.Item
          key={o.value}
          value={o.value}
          disabled={o.disabled}
          className={segment(o.value === value)}
        >
          {o.label}
        </ToggleGroup.Item>
      ))}
    </ToggleGroup.Root>
  );
}

function segment(on: boolean) {
  return `rounded-[6px] border px-3 py-1 text-[13px] focus-visible:outline-2 focus-visible:outline-acc disabled:opacity-45 ${
    on
      ? "border-transparent bg-accSoft font-medium text-accTx"
      : "border-transparent bg-transparent text-tx3 hover:text-tx2"
  }`;
}

const kindColors: Record<string, string> = {
  approval: "text-attnTx border-attnTx",
  question: "text-info border-info",
  review: "text-ok border-ok",
  blocked: "text-err border-err",
  finding: "text-team border-team",
};

/** A request's kind (Approval, Question, Review, Blocked, Finding). */
export function KindChip({ kind }: { kind: string }) {
  return (
    <span
      className={`whitespace-nowrap rounded-[4px] border px-1.5 py-px font-mono text-[10.5px] font-semibold uppercase tracking-[.05em] ${kindColors[kind] ?? "text-tx3 border-line2"}`}
    >
      {kind}
    </span>
  );
}

/** The amber count badge (the sidebar's Inbox). */
export function CountBadge({ count }: { count: number }) {
  if (count <= 0) return null;
  return (
    <span className="rounded-[9px] bg-attn px-1.5 py-px font-mono text-[10.5px] font-semibold text-attnInk">
      {count}
    </span>
  );
}

/** A small dot: an instance status, or a "needs you" / "changed" mark. */
export function Dot({
  status,
  color,
  size = 7,
  title,
}: {
  status?: string;
  color?: string;
  size?: number;
  title?: string;
}) {
  const style = status ? dotStyle(status) : { background: color ?? "var(--attn)" };
  return (
    <span
      className="inline-block shrink-0 rounded-full"
      style={{ width: size, height: size, ...style }}
      title={title}
    />
  );
}

/** The "lead" tag beside a lead's name; none when the name says it already. */
export function LeadTag({ name }: { name: string }) {
  if (saysLead(name)) return null;
  return (
    <span className="rounded-[4px] border border-accLine px-[5px] text-[11.5px] text-accTx">
      lead
    </span>
  );
}

/** A panel: request panels, lists. */
export function Panel({ children, className = "" }: { children: ReactNode; className?: string }) {
  return (
    <div className={`overflow-hidden rounded-xl border border-line bg-panel ${className}`}>
      {children}
    </div>
  );
}

/** Where data is still on its way: faint bars pulsing in a panel, in place of
 * the content (never its empty state, which would read as "there's none"). */
export function Loading({ rows = 3, className = "" }: { rows?: number; className?: string }) {
  return (
    <div
      role="status"
      aria-label="Loading"
      className={`flex flex-col gap-3 rounded-xl border border-line bg-panel px-4 py-4 ${className}`}
    >
      {Array.from({ length: rows }, (_, i) => (
        <span
          // biome-ignore lint/suspicious/noArrayIndexKey: placeholder bars, all alike
          key={i}
          className="h-3 rounded bg-hover motion-safe:animate-pulse"
          style={{ width: `${[72, 56, 64][i % 3]}%` }}
        />
      ))}
    </div>
  );
}

/** A dashed empty state. */
export function Empty({ children }: { children: ReactNode }) {
  return (
    <div className="rounded-xl border border-dashed border-line2 px-6 py-8 text-center text-tx3">
      {children}
    </div>
  );
}

/** Where the server doesn't serve an area yet (isNotBuilt). */
export function NotBuilt({ what }: { what: string }) {
  return (
    <Empty>
      <span className="font-medium text-tx2">Not built yet.</span> The server doesn't serve {what}.
    </Empty>
  );
}

/** A section label ("Projects", "Needs you"). */
/** A small label over a group inside a section (a review's repo in its tree). */
export function SectionLabel({ children }: { children: ReactNode }) {
  return <div className="text-[13px] font-medium text-tx4">{children}</div>;
}

/** A page section's title (Needs you, Projects, In flight), with how many it
 * holds when that helps. */
export function SectionTitle({ children, count }: { children: ReactNode; count?: number }) {
  return (
    <h2 className="m-0 flex items-baseline gap-2 text-[15px] font-semibold text-tx">
      {children}
      {count !== undefined && <span className="text-[13px] font-normal text-tx4">{count}</span>}
    </h2>
  );
}

/** A page's H1, with its subtitle. */
export function PageTitle({ title, subtitle }: { title: ReactNode; subtitle?: ReactNode }) {
  return (
    <header className="flex flex-col gap-1.5">
      <h1 className="text-[30px] font-semibold tracking-[-0.02em]">{title}</h1>
      {subtitle && <p className="text-[15px] text-tx3">{subtitle}</p>}
    </header>
  );
}

/** An on/off switch: saves at once, unlike a form checkbox. */
export function Switch({
  checked,
  onChange,
  label,
}: {
  checked: boolean;
  onChange: (on: boolean) => void;
  label: string;
}) {
  return (
    <button
      type="button"
      role="switch"
      aria-checked={checked}
      aria-label={label}
      onClick={() => onChange(!checked)}
      className={`relative h-5 w-[34px] shrink-0 rounded-[10px] transition-colors focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-acc ${
        checked ? "bg-acc" : "bg-line3"
      }`}
    >
      <span
        className={`absolute top-0.5 size-4 rounded-full bg-white transition-[left] duration-150 ${
          checked ? "left-4" : "left-0.5"
        }`}
      />
    </button>
  );
}

/** The status color of an instance (wiki/product/design/design-system.md#status-colors). */
export function statusColor(status: string): string {
  switch (status) {
    case "running":
      return "var(--ok)";
    case "awaiting":
      return "var(--attn)";
    case "done":
      return "var(--info)";
    case "starting":
      return "var(--team)";
    case "stopped":
      return "var(--err)";
    default:
      return "var(--tx4)"; // idle, queued, asleep
  }
}

/** A status dot's fill: queued is a dashed outline and asleep a plain one, so they
 * read differently from idle. */
export function dotStyle(status: string): { background: string; border?: string } {
  if (status === "queued") return { background: "var(--bg)", border: "1.5px dashed var(--tx4)" };
  if (status === "asleep") return { background: "var(--bg)", border: "1.5px solid var(--tx4)" };
  return { background: statusColor(status) };
}

/** An icon button that copies text (read when clicked) to the clipboard: a
 * check, then the icon again, says it took. The clipboard needs a secure page
 * (localhost or https), so elsewhere it says it couldn't. Its icon says what
 * it copies when two sit together (a folder's path beside a name). */
export function CopyButton({
  text,
  label,
  className = "size-6",
  icon = <CopyIcon />,
}: {
  text: () => string;
  label: string;
  className?: string;
  icon?: ReactNode;
}) {
  const [shown, setShown] = useState<"copied" | "failed" | null>(null);
  const copy = async () => {
    try {
      await navigator.clipboard.writeText(text());
      setShown("copied");
    } catch {
      setShown("failed");
    }
    setTimeout(() => setShown(null), 1400);
  };
  const said = shown === "copied" ? "Copied" : shown === "failed" ? "Couldn't copy" : label;
  return (
    <button
      type="button"
      onClick={copy}
      aria-label={said}
      title={said}
      className={`inline-flex shrink-0 items-center justify-center rounded-md text-tx3 hover:bg-hover hover:text-tx ${className}`}
    >
      {shown === "copied" ? <CheckIcon /> : icon}
    </button>
  );
}
