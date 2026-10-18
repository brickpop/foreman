import * as ToggleGroup from "@radix-ui/react-toggle-group";
import type { ButtonHTMLAttributes, DragEvent, ReactNode } from "react";
import { dotStyle } from "./Avatar";

// The design system's small building blocks (wiki/product/design/design-system.md):
// screens use these, never raw styles for the same thing.

const buttonVariants = {
  primary: "border border-acc bg-acc font-semibold text-accInk hover:brightness-105",
  secondary: "border border-line2 bg-transparent font-medium text-tx2 hover:border-line3",
  raised: "border border-line2 bg-raised font-medium text-tx hover:border-line3",
  muted: "border border-line2 bg-transparent font-medium text-tx3 hover:border-line3",
  danger: "border border-err bg-transparent font-medium text-err hover:bg-err/10",
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

/** A link that looks like an outline button: the ways between Sessions and a project. */
export const outlineLink =
  "rounded-[7px] border border-line2 px-3 py-[5px] text-[13px] font-medium text-tx2 hover:border-line3";

/** One choice of a segmented control (board switcher, Split, theme…). */
export function Segmented<T extends string>({
  options,
  value,
  onChange,
  label,
  onClose,
  onReorder,
}: {
  options: { value: T; label: ReactNode; disabled?: boolean }[];
  value: T;
  onChange: (value: T) => void;
  label: string;
  /** Gives each option a × (Sessions' deployment tabs: Stop). */
  onClose?: { label: string; close: (value: T) => void };
  /** Lets the options be dragged onto one another to reorder them. */
  onReorder?: (dragged: T, target: T) => void;
}) {
  // Native drag and drop: an option dropped on another takes its place.
  const drag = (value: T) =>
    onReorder && {
      draggable: true,
      onDragStart: (e: DragEvent) => e.dataTransfer.setData("text/plain", value),
      onDragOver: (e: DragEvent) => e.preventDefault(),
      onDrop: (e: DragEvent) => {
        e.preventDefault();
        onReorder(e.dataTransfer.getData("text/plain") as T, value);
      },
    };
  return (
    <ToggleGroup.Root
      type="single"
      value={value}
      onValueChange={(v) => v && onChange(v as T)}
      aria-label={label}
      className="inline-flex gap-1"
    >
      {options.map((o) => {
        const on = o.value === value;
        if (!onClose)
          return (
            <ToggleGroup.Item
              key={o.value}
              value={o.value}
              disabled={o.disabled}
              className={segment(on)}
              {...drag(o.value)}
            >
              {o.label}
            </ToggleGroup.Item>
          );
        // The × sits beside the option, not in it: a button can't hold a button.
        return (
          <span
            key={o.value}
            className={`${segment(on)} inline-flex items-center gap-2 pr-1.5`}
            {...drag(o.value)}
          >
            <ToggleGroup.Item
              value={o.value}
              disabled={o.disabled}
              className="focus-visible:outline-2 focus-visible:outline-acc"
            >
              {o.label}
            </ToggleGroup.Item>
            <button
              type="button"
              aria-label={`${onClose.label} ${o.value}`}
              title={onClose.label}
              onClick={() => onClose.close(o.value)}
              className="rounded px-1 text-tx4 hover:text-tx focus-visible:outline-2 focus-visible:outline-acc"
            >
              ×
            </button>
          </span>
        );
      })}
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
  approval: "text-accTx border-accTx",
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

/** The amber count badge (sidebar, tabs). */
export function CountBadge({ count, small }: { count: number; small?: boolean }) {
  if (count <= 0) return null;
  return (
    <span
      className={`bg-acc font-mono font-semibold text-accInk ${small ? "rounded-[9px] px-1.5 py-px text-[10.5px]" : "rounded-[10px] px-[7px] py-px text-[11px]"}`}
    >
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
  const style = status ? dotStyle(status) : { background: color ?? "var(--acc)" };
  return (
    <span
      className="inline-block shrink-0 rounded-full"
      style={{ width: size, height: size, ...style }}
      title={title}
    />
  );
}

/** A frontmatter chip, `key: value`. */
export function FieldChip({ name, value }: { name: string; value: string }) {
  return (
    <span className="rounded-[5px] bg-raised px-2 py-[3px] text-[12.5px] text-tx3">
      {name}: <span className="text-tx">{value}</span>
    </span>
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
