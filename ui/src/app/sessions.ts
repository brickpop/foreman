import type { Instance, Request, Team } from "../api/client";
import { roster } from "./model";

// The Sessions screen's logic (wiki/product/features/sessions.md), apart from
// its components so it can be tested.

export type DeploymentTab = {
  id: string;
  team: string; // its name
  project: string;
  label: string;
  needsYou: boolean;
  status: string;
};

// A team's state is its liveliest agent's: one awaiting the CEO in its
// session first, then one at work, down to all asleep.
const liveliness = [
  "awaiting",
  "running",
  "starting",
  "queued",
  "idle",
  "done",
  "asleep",
  "stopped",
];

/** A deployment's status, for its tab's dot: its liveliest agent's. */
export function deploymentStatus(instances: Instance[]): string {
  const ranks = instances.map((i) => liveliness.indexOf(i.status)).filter((r) => r >= 0);
  return ranks.length > 0 ? liveliness[Math.min(...ranks)] : "asleep";
}

/** One tab per active deployment: `<Team> · <project>`, its needs-you dot and its status. */
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
      team: name,
      project,
      label: `${name} · ${project}`,
      needsYou: requests.some((r) => r.status === "open" && r.deploymentId === id),
      status: deploymentStatus(instances.filter((i) => i.deploymentId === id)),
    };
  });
}

/** Tabs in the order the CEO dragged them to; new ones keep their place at the end. */
export function inOrder<T extends { id: string }>(tabs: T[], order: string[]): T[] {
  const rank = (t: T) => {
    const r = order.indexOf(t.id);
    return r < 0 ? order.length : r;
  };
  return [...tabs].sort((a, b) => rank(a) - rank(b));
}

/** The order after dropping dragged on target: dragged takes target's place. */
export function moved(ids: string[], dragged: string, target: string): string[] {
  if (dragged === target || !ids.includes(dragged) || !ids.includes(target)) return ids;
  const rest = ids.filter((id) => id !== dragged);
  const at = rest.indexOf(target) + (ids.indexOf(dragged) < ids.indexOf(target) ? 1 : 0);
  return [...rest.slice(0, at), dragged, ...rest.slice(at)];
}

/** The agent of a deployment with the latest output: where its work is. */
export function mostRecentAgent(instances: Instance[], deployment: string): string | undefined {
  const mine = instances.filter((i) => i.deploymentId === deployment && i.lastOutputAt);
  const at = (i: Instance) => Date.parse(i.lastOutputAt) || 0;
  const latest = mine.reduce<Instance | undefined>(
    (a, i) => (!a || at(i) > at(a) ? i : a),
    undefined,
  );
  return latest && at(latest) > 0 ? latest.agentId : undefined;
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

/** What the Sessions URL says (wiki/product/features/sessions.md#the-url): the
 * deployment, its active agent, both agents when split (left, right), and
 * whether Activity is open. Agents are named by their id; a full instance id
 * (agent@project) is read too. */
export type SessionsSearch = {
  deployment?: string;
  agent?: string;
  panes?: string;
  view?: "activity";
};

const agentOfId = (id: string) => id.split("@")[0];

/** The panes the URL describes, checked against the roster: an agent that
 * isn't in it falls back to the lead, a split that can't be made to one pane. */
export function panesFrom(search: SessionsSearch, roster: Instance[]): Panes {
  const find = (agent?: string) => roster.find((i) => agent && i.agentId === agentOfId(agent))?.id;
  const active = find(search.agent) ?? roster[0]?.id;
  if (!active) return { open: [], active: 0 };
  const [left, right] = (search.panes ?? "").split(",").map((a) => find(a));
  if (left && right && left !== right) {
    const open = [left, right];
    const at = open.indexOf(active);
    // An agent asked for outside the split takes the left pane.
    return at >= 0
      ? { open, active: at }
      : { open: [active, right === active ? left : right], active: 0 };
  }
  return { open: [active], active: 0 };
}

/** The URL for these panes: what panesFrom reads back. */
export function searchFor(deployment: string, p: Panes, activity: boolean): SessionsSearch {
  const ids = p.open.map(agentOfId);
  return {
    deployment,
    agent: ids[p.active],
    panes: ids.length === 2 ? ids.join(",") : undefined,
    view: activity ? "activity" : undefined,
  };
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

/** The @mention being typed just before the caret, if any: where its `@` is,
 * and what follows it so far. */
export function mentionAt(text: string, caret: number): { start: number; query: string } | null {
  const m = /(^|\s)@([\p{L}\p{N}_-]*)$/u.exec(text.slice(0, caret));
  return m ? { start: caret - m[2].length - 1, query: m[2] } : null;
}

/** The names a mention could be, in order: those that start with what is typed. */
export function mentionMatches(names: string[], query: string): string[] {
  const q = query.toLowerCase();
  return names.filter((n) => n.toLowerCase().startsWith(q));
}

/** The text with the mention replaced by the plain name (no `@`: a harness may
 * read `@` as its own file mention) and a space; and where the caret goes. */
export function applyMention(
  text: string,
  caret: number,
  start: number,
  name: string,
): { text: string; caret: number } {
  const before = `${text.slice(0, start)}${name} `;
  return { text: before + text.slice(caret), caret: before.length };
}
