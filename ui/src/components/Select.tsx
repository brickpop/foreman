import * as Menu from "@radix-ui/react-dropdown-menu";
import { Fragment, type ReactNode } from "react";
import { CheckIcon, Chevron } from "./icons";
import type { FieldInput } from "./ui";

// The one dropdown for choosing a value (a repo, a harness, a machine's size, a
// team, project or profile to deploy),
// over Radix's dropdown menu, styled like the Board's menu: arrow keys and
// typing a letter move through it, Escape closes it.

/** An option; a disabled one shows greyed, its detail saying why, and can't be picked. */
export type Option<T> = { value: T; label: ReactNode; detail?: ReactNode; disabled?: boolean };

export function Select<T extends string | number>({
  label,
  value,
  options,
  onChange,
  mono = false,
  className = "",
  field,
  placeholder,
}: {
  /** What it chooses, for screen readers. */
  label: string;
  value: T;
  options: Option<T>[];
  onChange: (value: T) => void;
  mono?: boolean;
  className?: string;
  /** Its id and description, inside a Field. */
  field?: FieldInput;
  /** Shown while the value is none of the options (nothing picked yet). */
  placeholder?: string;
}) {
  const current = options.find((o) => o.value === value);
  return (
    <Menu.Root>
      <Menu.Trigger
        {...field}
        aria-label={label}
        className={`flex min-w-0 items-center gap-2 rounded-[7px] border border-line2 bg-panel px-2.5 py-1 text-left text-[13px] text-tx hover:border-line3 focus-visible:outline-2 focus-visible:outline-acc data-[state=open]:border-acc ${mono ? "font-mono" : ""} ${className}`}
      >
        <span className={`min-w-0 flex-1 truncate ${current || !placeholder ? "" : "text-tx4"}`}>
          {current?.label ?? placeholder ?? String(value)}
        </span>
        <span className="rotate-90 text-tx4">
          <Chevron open={false} />
        </span>
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
            {options.map((o, i) => (
              <Fragment key={String(o.value)}>
                {/* A divider between the options to pick and the disabled ones after them. */}
                {o.disabled && i > 0 && !options[i - 1].disabled && (
                  <Menu.Separator className="my-1 h-px bg-line" />
                )}
                <Menu.RadioItem
                  value={String(o.value)}
                  disabled={o.disabled}
                  className={`flex cursor-pointer items-baseline gap-2 rounded px-2 py-1 text-[13px] text-tx2 outline-none data-[disabled]:cursor-default data-[disabled]:text-tx4 data-[highlighted]:bg-hover data-[state=checked]:text-tx ${mono ? "font-mono" : ""}`}
                >
                  <span className="w-3 shrink-0 text-accTx">
                    <Menu.ItemIndicator>
                      <CheckIcon size="size-3" />
                    </Menu.ItemIndicator>
                  </span>
                  <span className="flex-1">{o.label}</span>
                  {o.detail && <span className="font-sans text-[11.5px] text-tx4">{o.detail}</span>}
                </Menu.RadioItem>
              </Fragment>
            ))}
          </Menu.RadioGroup>
        </Menu.Content>
      </Menu.Portal>
    </Menu.Root>
  );
}
