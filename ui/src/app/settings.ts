// The Settings screen's tabs (wiki/product/features/settings.md), by topic.

export type SettingsTab = "teams" | "agents" | "git" | "look" | "system";

export const settingsTabs: { tab: SettingsTab; label: string }[] = [
  { tab: "teams", label: "Teams" },
  { tab: "agents", label: "Agents" },
  { tab: "git", label: "Git & forges" },
  { tab: "look", label: "Notifications & look" },
  { tab: "system", label: "System" },
];

/** The tab a URL names (/settings/<tab>, or the old /settings?tab=<tab>);
 * anything else, the old Workspace and Global tabs included, is Teams, the
 * first. */
export function settingsTab(asked: unknown): SettingsTab {
  return settingsTabs.find((t) => t.tab === asked)?.tab ?? "teams";
}

/** The stuck-after choices, in minutes, and the log cap's, in MB. */
export const stuckChoices = [5, 10, 15, 20, 30, 45, 60, 90, 120];
export const logCapChoices = [10, 25, 50, 100, 250, 500, 1000];

/** A select's choices with the current value among them: settings.toml can
 * hold any valid value, written by hand. */
export function withCurrent(choices: number[], value: number): number[] {
  return choices.includes(value) ? choices : [...choices, value].sort((a, b) => a - b);
}
