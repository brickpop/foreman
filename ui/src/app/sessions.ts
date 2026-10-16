import type { Instance, Request, Team } from "../api/client";
import { roster } from "./model";

// The Sessions screen's logic (wiki/product/features/sessions.md), apart from
// its components so it can be tested.

export type DeploymentTab = { id: string; label: string; needsYou: boolean };

/** One tab per active deployment: `<Team> · <project>`, with its needs-you dot. */
export function deploymentTabs(
  instances: Instance[],
  teams: Team[],
  requests: Request[],
): DeploymentTab[] {
  const ids = [...new Set(instances.map((i) => i.deploymentId).filter(Boolean))];
  return ids.map((id) => {
    const [team, project] = id.split("@");
    const name = teams.find((t) => t.id === team)?.config.name || team;
    return {
      id,
      label: `${name} · ${project}`,
      needsYou: requests.some((r) => r.status === "open" && r.deploymentId === id),
    };
  });
}

/** A deployment's instances in tab order: the lead first, then the roster's order. */
export function rosterOrder(deployment: string, instances: Instance[], teams: Team[]): Instance[] {
  const team = teams.find((t) => t.id === deployment.split("@")[0]);
  const mine = instances.filter((i) => i.deploymentId === deployment);
  if (!team) return mine;
  const order = roster(team.config);
  const rank = (i: Instance) => {
    const r = order.indexOf(i.agentId);
    return r < 0 ? order.length : r;
  };
  return [...mine].sort((a, b) => rank(a) - rank(b));
}

/** The panes of one deployment: one or two instance ids, and the active one. */
export type Panes = { open: string[]; active: number };

/** A deployment opens on its lead, in a single pane. */
export function initialPanes(roster: Instance[]): Panes {
  return { open: roster.length > 0 ? [roster[0].id] : [], active: 0 };
}

/** Clicking an agent tab: activate its pane, or show it in the active pane. */
export function clickTab(p: Panes, id: string): Panes {
  const at = p.open.indexOf(id);
  if (at >= 0) return { ...p, active: at };
  const open = [...p.open];
  open[p.active] = id;
  return { ...p, open };
}

/** Split: the first member not in pane 1 becomes pane 2, and active. Single
 * keeps the active pane's agent. */
export function toggleSplit(p: Panes, roster: Instance[]): Panes {
  if (p.open.length === 2) return { open: [p.open[p.active]], active: 0 };
  const other = roster.find((i) => i.id !== p.open[0]);
  if (!other) return p;
  return { open: [p.open[0], other.id], active: 1 };
}

/** Closing a pane (when split): the other stays, and pane 1 is active. */
export function closePane(p: Panes, index: number): Panes {
  return { open: p.open.filter((_, i) => i !== index), active: 0 };
}

/** The line input's placeholder (wiki/product/features/sessions.md#panes). */
export function linePlaceholder(name: string, status: string, lead: boolean): string {
  if (status === "awaiting") return `Reply to ${name}…`;
  return lead ? `Tell ${name} what you need. They’ll delegate.` : `Direct instruction to ${name}…`;
}

/** A countdown as m:ss, rounded up (it reaches 0:00 only at the end). */
export function countdown(ms: number): string {
  const s = Math.max(0, Math.ceil(ms / 1000));
  return `${Math.floor(s / 60)}:${String(s % 60).padStart(2, "0")}`;
}
/** Whether this is a Mac, where shortcuts use Cmd instead of Ctrl. */
export const isMac = typeof navigator !== "undefined" && /Mac|iPhone|iPad/.test(navigator.platform);

/** The line input's shortcut: Cmd+Shift+L on a Mac, Ctrl+Shift+L elsewhere.
 * Ctrl+L alone (and on a Mac, every Ctrl key) still reaches the agent. */
export function lineShortcut(mac = isMac): string {
  return mac ? "⌘⇧L" : "Ctrl+Shift+L";
}

type Keys = {
  code: string;
  ctrlKey: boolean;
  shiftKey: boolean;
  altKey: boolean;
  metaKey: boolean;
};

export function isLineShortcut(e: Keys, mac = isMac): boolean {
  const mod = mac ? e.metaKey && !e.ctrlKey : e.ctrlKey && !e.metaKey;
  return e.code === "KeyL" && mod && e.shiftKey && !e.altKey;
}
