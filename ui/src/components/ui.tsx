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
  danger: "border border-err bg-transparent font-medium text-err hover:bg-err/10",
  ghost: "border border-transparent bg-transparent font-medium text-tx3 hover:text-tx2",
  tiny: "border border-line2 bg-transparent text-xs text-tx3 hover:border-line3",
} as const;

type ButtonProps = ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: keyof typeof buttonVariants;
};

/** A button; disabled is real (not focusable as an action, dimmed). */
export function Button({ variant = "secondary", className = "", ...props }: ButtonProps) {
  const size = variant === "tiny" ? "rounded-[5px] px-2 py-0.5" : "rounded-[7px] px-3.5 py-[7px]";
  return (
    <button
      type="button"
      className={`${size} text-[13px] focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-acc disabled:opacity-45 ${buttonVariants[variant]} ${className}`}
      {...props}
    />
  );
}

/** One choice of a segmented control (board switcher, Split, theme…). */
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
          className={`rounded-[6px] border px-3 py-1 text-[13px] focus-visible:outline-2 focus-visible:outline-acc disabled:opacity-45 ${
            o.value === value
              ? "border-line2 bg-raised text-tx"
              : "border-transparent bg-transparent text-tx3 hover:text-tx2"
          }`}
        >
          {o.label}
        </ToggleGroup.Item>
      ))}
    </ToggleGroup.Root>
  );
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
      className={`relative h-5 w-9 shrink-0 rounded-full border transition-colors focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-acc ${
        checked ? "border-acc bg-acc" : "border-line3 bg-raised"
      }`}
    >
      <span
        className={`absolute top-0.5 size-3.5 rounded-full transition-[left] ${
          checked ? "left-[18px] bg-accInk" : "left-0.5 bg-tx3"
        }`}
      />
    </button>
  );
}
