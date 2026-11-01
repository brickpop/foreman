import * as ToggleGroup from "@radix-ui/react-toggle-group";
import type { ButtonHTMLAttributes, ReactNode } from "react";
import { dotStyle } from "./Avatar";

// The design system's small building blocks (wiki/product/design/design-system.md):
// screens use these, never raw styles for the same thing.

const buttonVariants = {
  primary: "border border-acc bg-acc font-semibold text-accInk hover:brightness-105",
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
  tiny: "rounded-[5px] px-2 py-0.5 text-xs",
};

type ButtonProps = ButtonHTMLAttributes<HTMLButtonElement> & {
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

/** A card on a grid of cards (Dashboard). */
export const card =
  "relative flex flex-col gap-3 rounded-xl border border-line bg-panel px-4 py-3.5";
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
      ? "border-line2 bg-raised text-tx"
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

/** The "lead" tag. */
export function LeadTag() {
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
export function SectionLabel({ children }: { children: ReactNode }) {
  return <div className="text-[13px] font-medium text-tx4">{children}</div>;
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
