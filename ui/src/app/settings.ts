// The Settings screen's tabs (wiki/product/features/settings.md), by topic.

export type SettingsTab = "agents" | "git" | "look" | "system";

export const settingsTabs: { tab: SettingsTab; label: string }[] = [
  { tab: "agents", label: "Agents" },
  { tab: "git", label: "Git & forges" },
  { tab: "look", label: "Notifications & look" },
  { tab: "system", label: "System" },
];

/** The tab in the URL; Agents, the first, isn't spelled out. Anything else,
 * the old Workspace and Global tabs included, opens Agents. */
export function settingsSearch(s: Record<string, unknown>): { tab?: SettingsTab } {
  const tab = settingsTabs.find((t) => t.tab === s.tab)?.tab;
  return { tab: tab === "agents" ? undefined : tab };
}

/** The stuck-after choices, in minutes, and the log cap's, in MB. */
export const stuckChoices = [5, 10, 15, 20, 30, 45, 60, 90, 120];
export const logCapChoices = [10, 25, 50, 100, 250, 500, 1000];

/** The sleep-after choices, in minutes; 0 is never. */
export const sleepChoices = [0, 10, 20, 30, 60, 120];

export function sleepLabel(minutes: number): string {
  return minutes === 0 ? "Never" : `After ${minutes} min idle`;
}

/** A select's choices with the current value among them: settings.toml can
 * hold any valid value, written by hand. */
export function withCurrent(choices: number[], value: number): number[] {
  return choices.includes(value) ? choices : [...choices, value].sort((a, b) => a - b);
}
