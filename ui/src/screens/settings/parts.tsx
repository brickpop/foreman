import type { ReactNode } from "react";
import type { Settings as SettingsData } from "../../api/client";
import { useSaveSettings } from "../../app/queries";
import { useToast } from "../../components/Toasts";

// What every Settings tab is built from: a section, a row, and saving.

/** Where a section's settings are kept. */
export const inGlobalRepo = "Global repo";
export const onThisMachine = "This machine";

/** Saves a change to the settings read last; a failure says why and the screen refetches. */
export function useSave() {
  const save = useSaveSettings();
  const toast = useToast();
  return (change: (s: SettingsData) => SettingsData) =>
    save(change).catch((e: Error) =>
      toast({ title: "Settings not saved", body: e.message, color: "var(--err)" }),
    );
}

export function Section({
  label,
  kept,
  intro,
  children,
}: {
  label: string;
  /** Where what it sets is kept; none for what isn't a setting. */
  kept?: string;
  intro?: string;
  children: ReactNode;
}) {
  return (
    <section className="flex flex-col">
      <h2 className="m-0 mb-1 text-[13px] font-medium text-tx4">
        {label}
        {kept && <span className="font-normal"> · {kept}</span>}
      </h2>
      {intro && <p className="m-0 mb-2 text-[13.5px] text-tx3">{intro}</p>}
      {children}
    </section>
  );
}

export function Row({
  title,
  explanation,
  children,
}: {
  title: string;
  explanation?: ReactNode;
  children: ReactNode;
}) {
  return (
    <div className="grid grid-cols-[1fr_auto] items-center gap-6 border-b border-line py-3.5">
      <div className="flex flex-col gap-0.5">
        <span className="font-medium">{title}</span>
        {explanation && <span className="text-[13.5px] leading-[1.5] text-tx3">{explanation}</span>}
      </div>
      <div className="flex items-center gap-2">{children}</div>
    </div>
  );
}
