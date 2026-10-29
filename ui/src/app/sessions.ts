import type { Deployment, Doing, Instance, Request, Team } from "../api/client";
import { roster } from "./model";

// The Sessions screen's logic (wiki/product/features/sessions.md), apart from
// its components so it can be tested.

export type ActiveDeployment = {
  id: string;
  team: string; // its name
  project: string;
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

/** A deployment's status, for its dot: its liveliest agent's. */
export function deploymentStatus(instances: Instance[]): string {
  const ranks = instances.map((i) => liveliness.indexOf(i.status)).filter((r) => r >= 0);
  return ranks.length > 0 ? liveliness[Math.min(...ranks)] : "asleep";
}

/** The active deployments, by project, then team name: each with its
 * needs-you dot and its status. */
export function activeDeployments(
  instances: Instance[],
  teams: Team[],
  requests: Request[],
): ActiveDeployment[] {
  const ids = [...new Set(instances.map((i) => i.deploymentId).filter(Boolean))];
  return ids
    .map((id) => {
      const [team, project] = id.split("@");
      return {
        id,
        team: teams.find((t) => t.id === team)?.config.name || team,
        project,
        needsYou: requests.some((r) => r.status === "open" && r.deploymentId === id),
        status: deploymentStatus(instances.filter((i) => i.deploymentId === id)),
      };
    })
    .sort((a, b) => a.project.localeCompare(b.project) || a.team.localeCompare(b.team));
}

export type SessionRow = ActiveDeployment & {
  goal: string;
  state: "working" | "idle" | "asleep" | "stopped";
  /** When it last did something (RFC 3339), "" when nothing says. */
  lastActivity: string;
};

// A time as milliseconds; a blank, a bad one and Go's zero time are 0.
const ms = (t?: string) => Math.max(Date.parse(t ?? "") || 0, 0);
// The latest of some times, "" when none counts.
const latest = (times: (string | undefined)[]) =>
  times.reduce<string>((a, t) => (t && ms(t) > ms(a) ? t : a), "");

/** The Sessions list: one row per active deployment, the latest activity
 * first (ties keep activeDeployments' order). An agent's last step is kept in
 * memory only, so after a restart its start, then the deploy, stand in. */
export function sessionRows(
  active: ActiveDeployment[],
  instances: Instance[],
  deployments: Deployment[],
): SessionRow[] {
  return active
    .map((d) => {
      const mine = instances.filter((i) => i.deploymentId === d.id);
      const deployment = deployments.find((x) => x.id === d.id);
      const lastActivity =
        latest(mine.map((i) => i.doing?.at)) ||
        latest(mine.map((i) => i.startedAt)) ||
        latest([deployment?.createdAt]);
      const working = mine.some((i) => i.status === "running" || i.status === "starting");
      const state: SessionRow["state"] = working
        ? "working"
        : d.status === "asleep" || d.status === "stopped"
          ? d.status
          : "idle";
      return { ...d, goal: deployment?.goal ?? "", state, lastActivity };
    })
    .sort((a, b) => ms(b.lastActivity) - ms(a.lastActivity));
}

/** The agent of a deployment that did something last: where its work is. */
export function mostRecentAgent(instances: Instance[], deployment: string): string | undefined {
  const mine = instances.filter((i) => i.deploymentId === deployment && i.doing);
  const at = (i: Instance) => Date.parse(i.doing?.at ?? "") || 0;
  const latest = mine.reduce<Instance | undefined>(
    (a, i) => (!a || at(i) > at(a) ? i : a),
    undefined,
  );
  return latest && at(latest) > 0 ? latest.agentId : undefined;
}

/** The instances with one's activity line replaced: an instance.doing event,
 * pushed at every step of its work, so never refetched for. */
export function withDoing(
  instances: Instance[] | undefined,
  change: { id: string; doing: Doing },
): Instance[] | undefined {
  return instances?.map((i) => (i.id === change.id ? { ...i, doing: change.doing } : i));
}

/** A deployment's instances in tab order: the lead first, then the team file's order. */
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

/** What the Sessions URL says (wiki/product/features/sessions.md#the-url): the
 * deployment, its agent shown, and whether Activity is open. The agent is
 * named by its id; a full instance id (agent@project) is read too. */
export type SessionsSearch = {
  deployment?: string;
  agent?: string;
  view?: "activity";
};

export type SessionsView = { deployment: string; agent: string; view?: "activity" };

/** The view the URL describes, checked against what is deployed: an unknown
 * agent is the lead. A URL naming only the deployment (the list's link) comes
 * back to its `last` view. Undefined when the URL names no deployment that is
 * deployed: the list of deployments shows instead. */
export function sessionsView(
  search: SessionsSearch,
  deployments: string[],
  rosterOf: (deployment: string) => Instance[],
  last: (deployment: string) => SessionsView | undefined,
): SessionsView | undefined {
  const deployment = deployments.find((d) => d === search.deployment);
  if (!deployment) return undefined;
  const remembered = search.agent || search.view ? undefined : last(deployment);
  const asked = (search.agent ?? remembered?.agent ?? "").split("@")[0];
  const roster = rosterOf(deployment);
  const agent = roster.find((i) => i.agentId === asked)?.agentId ?? roster[0]?.agentId ?? "";
  return { deployment, agent, view: search.view ?? remembered?.view };
}

/** The line input's placeholder (wiki/product/features/sessions.md#the-terminal). */
export function linePlaceholder(name: string, status: string, lead: boolean): string {
  if (status === "awaiting") return `Reply to ${name}…`;
  return lead ? `Tell ${name} what you need. They’ll delegate.` : `Direct instruction to ${name}…`;
}

/** A stopped agent's screen: why Foreman stopped it, when it did (a start that
 * failed, a crash loop), and that its messages wait. */
export function stoppedLine(name: string, reason: string): string {
  const why = reason ? `${name} stopped: ${reason}.` : `${name} stopped.`;
  return `${why} Messages for them wait until you start them again; they pick up their conversation.`;
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
