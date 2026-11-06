import type { Doing, Instance, Request, Team } from "../api/client";
import { roster } from "./model";

// A project's Session section's logic (wiki/product/features/sessions.md),
// apart from its components so it can be tested.

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

/** The agents something open waits on the CEO for (an escalation, a review):
 * each tab's needs-you mark. Their status stays their harness's: an agent
 * waiting on an answer still gets its messages (wiki/product/features/requests.md). */
export function waitingOnYou(requests: Request[]): Set<string> {
  return new Set(
    requests.filter((r) => r.status === "open" && r.instanceId).map((r) => r.instanceId),
  );
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

/** The agent a Session shows (wiki/product/features/sessions.md#the-url):
 * the one asked, by id or full instance id; else the last one shown on the
 * project; else the lead, first in the roster. Undefined with no roster. */
export function sessionAgent(
  asked: string | undefined,
  roster: Instance[],
  last?: string,
): string | undefined {
  const known = (id?: string) => roster.find((i) => i.agentId === id?.split("@")[0])?.agentId;
  return known(asked) ?? known(last) ?? roster[0]?.agentId;
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

/** The notice of messages held by the CEO's draft: on it, they wait for the
 * line; once it went out, for the agent's turn to end. */
export function draftNotice(waiting: number, name: string, onDraft: boolean): string {
  const messages = waiting === 1 ? "1 message" : `${waiting} messages`;
  return onDraft
    ? `${messages} waiting for your line: send or clear it.`
    : `${messages} waiting: ${waiting === 1 ? "it goes" : "they go"} in when ${name}'s turn ends.`;
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

/** Foreman's own keys on a session: Cmd+Shift on a Mac, Ctrl+Shift elsewhere
 * (on a Mac, Ctrl is the terminal's). */
const sessionKeys = (e: Keys, mac: boolean) =>
  (mac ? e.metaKey && !e.ctrlKey : e.ctrlKey && !e.metaKey) && e.shiftKey && !e.altKey;

export function isLineShortcut(e: Keys, mac = isMac): boolean {
  return e.code === "KeyL" && sessionKeys(e, mac);
}

/** The agent tab a key moves to: Cmd+Shift+← / → (Ctrl+Shift off a Mac) the
 * previous / next one (the CEO, 2026-10-30), 0 for any other key. While the
 * Line input holds text, 0 too: the keys do what the system makes them do,
 * selecting in a text box (the CEO, 2026-11-05). */
export function agentStep(e: Keys, lineHeld: boolean, mac = isMac): -1 | 0 | 1 {
  if (lineHeld || !sessionKeys(e, mac)) return 0;
  return e.code === "ArrowLeft" ? -1 : e.code === "ArrowRight" ? 1 : 0;
}

/** The agent `step` tabs away from `shown`, round the ends. */
export function stepAgent(roster: string[], shown: string, step: number): string | undefined {
  const at = roster.indexOf(shown);
  if (at < 0 || roster.length === 0) return roster[0];
  return roster[(at + step + roster.length) % roster.length];
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
