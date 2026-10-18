import * as Menu from "@radix-ui/react-dropdown-menu";
import type { ReactNode } from "react";

// The one dropdown for choosing a value (a repo, a harness, a machine's size),
// over Radix's dropdown menu, styled like the Board's menu: arrow keys and
// typing a letter move through it, Escape closes it.

export type Option<T> = { value: T; label: ReactNode; detail?: ReactNode };

export function Select<T extends string | number>({
  label,
  value,
  options,
  onChange,
  mono = false,
  className = "",
}: {
  /** What it chooses, for screen readers. */
  label: string;
  value: T;
  options: Option<T>[];
  onChange: (value: T) => void;
  mono?: boolean;
  className?: string;
}) {
  const current = options.find((o) => o.value === value);
  return (
    <Menu.Root>
      <Menu.Trigger
        aria-label={label}
        className={`flex min-w-0 items-center gap-2 rounded-[7px] border border-line2 bg-panel px-2.5 py-1 text-left text-[13px] text-tx hover:border-line3 focus-visible:outline-2 focus-visible:outline-acc data-[state=open]:border-acc ${mono ? "font-mono" : ""} ${className}`}
      >
        <span className="min-w-0 flex-1 truncate">{current?.label ?? String(value)}</span>
        <span className="text-[10px] text-tx4">▾</span>
      </Menu.Trigger>
      <Menu.Portal>
        <Menu.Content
          align="start"
          sideOffset={4}
          className="z-50 flex max-h-[min(360px,var(--radix-dropdown-menu-content-available-height))] min-w-[var(--radix-dropdown-menu-trigger-width)] flex-col overflow-auto rounded-lg border border-line2 bg-panel p-1 shadow-overlay"
        >
          <Menu.RadioGroup
            value={String(value)}
            onValueChange={(v) => {
              const chosen = options.find((o) => String(o.value) === v);
              if (chosen) onChange(chosen.value);
            }}
          >
            {options.map((o) => (
              <Menu.RadioItem
                key={String(o.value)}
                value={String(o.value)}
                className={`flex cursor-pointer items-baseline gap-2 rounded px-2 py-1 text-[13px] text-tx2 outline-none data-[highlighted]:bg-hover data-[state=checked]:text-tx ${mono ? "font-mono" : ""}`}
              >
                <span className="w-3 shrink-0 text-accTx">
                  <Menu.ItemIndicator>✓</Menu.ItemIndicator>
                </span>
                <span className="flex-1">{o.label}</span>
                {o.detail && <span className="font-sans text-[11.5px] text-tx4">{o.detail}</span>}
              </Menu.RadioItem>
            ))}
          </Menu.RadioGroup>
        </Menu.Content>
      </Menu.Portal>
    </Menu.Root>
  );
}
