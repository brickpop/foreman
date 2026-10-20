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
