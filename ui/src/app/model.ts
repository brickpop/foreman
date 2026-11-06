/** The project's agents whose run started on another profile than its own
 * now: they switch at their next start. A run from before the profile was
 * kept is judged by its harness (wiki/product/features/project-settings.md). */
export function onOtherProfile(
  instances: Instance[],
  project: string,
  profile: { name: string; harness: string },
): Instance[] {
  const up = ["starting", "running", "idle", "awaiting"];
  return instances.filter(
    (i) =>
      i.id.endsWith(`@${project}`) &&
      up.includes(i.status) &&
      (i.profile ? i.profile !== profile.name : i.harness !== profile.harness),
  );
}

/** A deployment's Reload (wiki/product/models/team-definition.md#reload):
 * its live agents whose run started on an older prompt, and the first one
 * busy (on its way up, working, or waiting on the CEO), which holds it back
 * so no turn is ever cut. */
export function reloadOf(instances: Instance[]): { behind: Instance[]; busy?: Instance } {
  return {
    behind: instances.filter((i) => i.behind),
    busy: instances.find((i) => ["queued", "starting", "running", "awaiting"].includes(i.status)),
  };
}

/** What Reload would do, or why it waits. */
export function reloadLine(reload: ReturnType<typeof reloadOf>, teams: Team[]): string {
  const n = reload.behind.length;
  if (n === 0) return "Every agent runs on the current prompt.";
  const runs = n === 1 ? "1 agent runs" : `${n} agents run`;
  if (!reload.busy) {
    return `${runs} on an older prompt: Reload restarts ${n === 1 ? "it" : "them"} on the new one, continuing ${n === 1 ? "its conversation" : "their conversations"}.`;
  }
  const name =
    agentOf(teams, reload.busy.deploymentId, reload.busy.id)?.soul.name || reload.busy.agentId;
  const why = reload.busy.status === "awaiting" ? "is waiting on you" : "is busy";
  return `${runs} on an older prompt. ${name} ${why}: the change applies at each agent's next start. Reload once they're idle.`;
}

import type { Instance, MergeRequest, Project, Repo, Request, Team } from "../api/client";
import type { Soul, TeamConfig } from "../api/types/home";

// Pure derivations the screens share: who an instance is, how requests are
// ordered, what a project's state is. Kept apart from components to be tested.

export type Agent = { id: string; soul: Soul; team: Team };

/** The agent behind an instance id (agent@project) in a deployment
 * (team@project), from the team files. Through the deployment's team: an
 * agent id is only unique within its team (wiki/product/models/team-definition.md). */
export function agentOf(teams: Team[], deployment: string, instance: string): Agent | undefined {
  const id = instance.split("@")[0];
  const team = teams.find((t) => t.id === deployment.split("@")[0]);
  const soul = team?.config.agents[id];
  return team && soul ? { id, soul, team } : undefined;
}

const kindOrder = ["blocked", "approval", "review", "question", "finding"];

/** Requests in list order: blocked, approvals, reviews, questions, findings;
 * oldest first within a kind (wiki/product/features/requests.md). */
export function ordered(requests: Request[]): Request[] {
  const rank = (k: string) => {
    const i = kindOrder.indexOf(k);
    return i < 0 ? kindOrder.length : i;
  };
  return [...requests].sort(
    (a, b) => rank(a.kind) - rank(b.kind) || a.createdAt.localeCompare(b.createdAt),
  );
}

/** The open requests answered where they show (Dashboard, a project's
 * Inbox): every kind but reviews, which are decided on their own page
 * (wiki/product/features/requests.md). In list order. */
export function inbox(requests: Request[]): Request[] {
  return ordered(requests.filter((r) => r.status === "open" && r.kind !== "review"));
}

/** The Dashboard's title, by the time of day (wiki/product/features/dashboard.md#header). */
export function greeting(now = new Date()): string {
  const h = now.getHours();
  return h < 5
    ? "Good night"
    : h < 12
      ? "Good morning"
      : h < 18
        ? "Good afternoon"
        : "Good evening";
}

/** The Dashboard's subtitle: running sessions across the projects with a team. */
export function workingSummary(instances: Instance[]): string {
  const running = instances.filter((i) => i.status === "running").length;
  const projects = new Set(instances.map((i) => i.deploymentId).filter(Boolean)).size;
  return `${plural(running, "session")} working across ${plural(projects, "project")}.`;
}

export type TeamCard = {
  id: string;
  name: string;
  purpose: string;
  /** Its members' souls, the lead first. */
  members: Soul[];
  /** The projects it is deployed on. */
  on: string[];
};

/** One card per team: who is in it and where it works. */
export function teamCards(teams: Team[], instances: Instance[]): TeamCard[] {
  return teams.map((t) => ({
    id: t.id,
    name: t.config.name || t.id,
    purpose: t.config.purpose,
    members: roster(t.config).map((id) => t.config.agents[id]),
    on: [
      ...new Set(
        instances
          .filter((i) => i.deploymentId.startsWith(`${t.id}@`))
          .map((i) => i.deploymentId.split("@")[1]),
      ),
    ].sort(),
  }));
}

export type ProjectRow = {
  id: string;
  line: string;
  state: "needs you" | "on track" | "idle";
};

/** One row per project not archived: its team and working agents, and its state. */
export function projectRows(
  projects: Project[],
  teams: Team[],
  instances: Instance[],
  requests: Request[],
): ProjectRow[] {
  return projects
    .filter((p) => !p.archived)
    .map((p) => {
      const mine = instances.filter((i) => i.id.endsWith(`@${p.id}`));
      const teamIds = [...new Set(mine.map((i) => i.deploymentId.split("@")[0]))];
      const names = teamIds.map((t) => teams.find((x) => x.id === t)?.config.name || t);
      const working = mine.filter((i) => i.status === "running").length;
      const open = requests.some((r) => r.status === "open" && r.projectId === p.id);
      return {
        id: p.id,
        line:
          names.length === 0
            ? "No team deployed"
            : `${names.join(", ")} · ${working} agent${working === 1 ? "" : "s"} working`,
        state: open ? "needs you" : working > 0 ? "on track" : "idle",
      };
    });
}

const harnessLabels: Record<string, string> = {
  "claude-code": "Claude Code",
  codex: "Codex CLI",
  pi: "Pi",
  hermes: "Hermes Agent",
  custom: "Custom",
};

/** A harness's label (wiki/product/models/agent-soul.md). */
export function harnessLabel(harness: string): string {
  return harnessLabels[harness] ?? harness;
}

/** A new profile's name as typed, made into one (lowercase letters,
 * digits and dashes, at most 40: "My Work" → "my-work"), or "" when nothing
 * is left; the server checks the same in settings.toml. */
export function profileName(typed: string): string {
  return typed
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "")
    .slice(0, 40)
    .replace(/-+$/, "");
}

const roleLabels: Record<string, string> = {
  team_lead: "Team Lead",
  architect: "Architect",
  engineer: "Engineer",
  qa: "QA / Tester",
  reviewer: "Reviewer",
};

/** A role as it reads: a built-in label, or a custom one with spaces and a capital. */
export function roleLabel(role: string): string {
  if (roleLabels[role]) return roleLabels[role];
  const words = role.replaceAll("_", " ");
  return words.charAt(0).toUpperCase() + words.slice(1);
}

/** Who a playbook step or escalation names, as the Team page shows it
 * (wiki/product/models/team-definition.md#playbook-and-escalation). Unknown
 * is shown as written, flagged. */
export function resolveWho(team: TeamConfig, who: string): { text: string; known: boolean } {
  const agents = team.agents;
  if (who === "lead") return { text: agents[team.lead]?.name ?? who, known: true };
  if (who === "members") return { text: "Members", known: true };
  if (who === "ceo") return { text: "You", known: true };
  if (agents[who]) return { text: agents[who].name, known: true };
  const names = Object.values(agents)
    .filter((s) => s.role === who)
    .map((s) => s.name);
  return names.length > 0 ? { text: names.join(", "), known: true } : { text: who, known: false };
}

/** A team's agent ids as the team presents them: the lead first, then the
 * file's order, then any it doesn't name, sorted (as home.TeamConfig.Roster). */
export function roster(c: Team["config"]): string[] {
  const ids: string[] = [];
  const add = (id: string) => {
    if (c.agents[id] && !ids.includes(id)) ids.push(id);
  };
  add(c.lead);
  for (const id of c.order ?? []) add(id);
  for (const id of Object.keys(c.agents).sort()) add(id);
  return ids;
}

/** A count with its noun: "1 session", "3 sessions". */
export function plural(n: number, noun: string): string {
  return `${n} ${noun}${n === 1 ? "" : "s"}`;
}

/** What stopping a team ends: `1 session ends`, `3 sessions end`. */
export function sessionsEnd(n: number): string {
  return `${plural(n, "session")} ${n === 1 ? "ends" : "end"}`;
}

/** The Deploy dialog's warning when another team is on the project: it is
 * replaced, as by Stop (a project has one team at most). */
export function replacingLine(team: string, project: string, sessions: number): string {
  return `Replaces ${team} on ${project}: ${sessionsEnd(sessions)}. Worktrees and branches are kept.`;
}

const forgeLabels: Record<string, string> = {
  github: "GitHub",
  forgejo: "Forgejo",
  gitea: "Gitea",
  none: "No forge",
};

/** A forge kind as it reads. */
export function forgeLabel(kind: string): string {
  return forgeLabels[kind] ?? kind;
}

/** A merge request's line on its review's decision bar: its PR, its repo and branch, and its checks. */
export function linkLine(mr: MergeRequest, forge: string): string {
  const pr = mr.forgePrNumber ? `PR #${mr.forgePrNumber} on ${forgeLabel(forge)} · ` : "";
  // No checks status: the project has no checks (or they haven't run): nothing to say.
  const tail = mr.conflicting
    ? `conflicts with ${mr.target}`
    : mr.checksStatus
      ? `checks ${mr.checksStatus}`
      : "";
  // The repo first: a review ticks its repos one by one.
  return `${pr}${mr.repo} · ${mr.branch} → ${mr.target}${tail ? ` · ${tail}` : ""}`;
}

/** Whether a branch is an agent's (wt/*, lead/*): agents write those, so a
 * merge never goes into one, and none is a repo's default branch. */
export const agentBranch = (name: string): boolean =>
  name.startsWith("wt/") || name.startsWith("lead/");

/** The branches a repo's merge can go into: any of its own but an agent's, as
 * the server lists them (the default first, then newest commit first). None
 * for a repo not listed (or before they have loaded). */
export function mergeTargets(repos: Repo[] | undefined, repo: string): string[] {
  const r = repos?.find((x) => x.name === repo && !x.project);
  return r?.branches.map((b) => b.name).filter((b) => !agentBranch(b)) ?? [];
}

/** The forge a merge request can become a PR on ("none": no Create PR): its
 * project's, since merge requests are on the project's code repos. */
export function forgeOf(project?: Project): string {
  return project?.config.forge.kind ?? "none";
}

const months = ["Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"];

/** A stored time as the CEO reads it, in local time: `14:05` today, `Oct 15
 * 14:05` earlier this year, `Oct 15 2025 14:05` before. */
export function when(iso: string, now = new Date()): string {
  const t = new Date(iso);
  const hm = `${String(t.getHours()).padStart(2, "0")}:${String(t.getMinutes()).padStart(2, "0")}`;
  if (t.toDateString() === now.toDateString()) return hm;
  const day = `${months[t.getMonth()]} ${t.getDate()}`;
  return t.getFullYear() === now.getFullYear() ? `${day} ${hm}` : `${day} ${t.getFullYear()} ${hm}`;
}

/** How long ago a stored time was: `just now`, `5m ago`, `3h ago`, `2d ago`.
 * A time ahead of `now` (another clock) is `just now`. */
export function ago(iso: string, now = new Date()): string {
  const minutes = Math.floor((now.getTime() - Date.parse(iso)) / 60_000);
  if (!(minutes >= 1)) return "just now";
  if (minutes < 60) return `${minutes}m ago`;
  if (minutes < 24 * 60) return `${Math.floor(minutes / 60)}h ago`;
  return `${Math.floor(minutes / (24 * 60))}d ago`;
}

/** How many session slots are taken: every instance starting, running, idle
 * or awaiting holds one (wiki/product/architecture/agent-runtime.md). */
export function slotsTaken(instances: Instance[]): number {
  return instances.filter((i) => ["starting", "running", "idle", "awaiting"].includes(i.status))
    .length;
}

/** Why a message to an agent still waits, as its project's Activity says it
 * (messages are delivered when their recipient is idle). */
export function waitingReason(
  to: Instance | undefined,
  name: string,
  instances: Instance[],
  maxSessions: number,
  draft: boolean,
): string {
  switch (to?.status) {
    case "queued": {
      return `${name} hasn't started: ${slotsTaken(instances)} of ${maxSessions} session slots are taken, idle agents included`;
    }
    case "asleep":
      return `${name} is asleep, and is being woken for it`;
    case "starting":
      return `${name} is starting`;
    case "running":
      return `${name} is busy: it gets it when its turn ends`;
    case "awaiting":
      return `${name} is waiting for you first`;
    case "idle":
      return draft ? `${name}'s terminal has your unsent line: it waits for it` : "Being delivered";
    default:
      return `${name} isn't running`;
  }
}

export type TeamTab = "deployments" | "members" | "playbook" | "file";
export const teamTabs: TeamTab[] = ["deployments", "members", "playbook", "file"];

/** The Team page's tab: the one in the URL, else Deployments while the team is
 * deployed and Members while it isn't (nothing to show on Deployments). */
export function teamTab(asked: unknown, deployed: boolean): TeamTab {
  if (teamTabs.includes(asked as TeamTab)) return asked as TeamTab;
  return deployed ? "deployments" : "members";
}

/** A project's or repo's name as people read it (the CEO, 2026-11-02):
 * hyphens and underscores as spaces, the first letter capital; my-portfolio
 * is "My portfolio". Only how it shows: its id, folder and URL stay the name. */
export function readable(name: string): string {
  const words = name.replace(/[-_]+/g, " ").trim();
  return words.charAt(0).toUpperCase() + words.slice(1);
}

/** The projects by the name they show (projectName), as people sort words:
 * case and accents aside, numbers by value (ledger 2 before ledger 10). */
export function byName<T extends { id: string; name: string }>(projects: T[]): T[] {
  const name = (p: T) => p.name || readable(p.id);
  return [...projects].sort((a, b) =>
    name(a).localeCompare(name(b), undefined, { sensitivity: "base", numeric: true }),
  );
}

/** The other project already showing as what project would show as with
 * name (its readable id when empty), in any case, which the server refuses:
 * two projects never read the same. Undefined when it is free. */
export function nameTakenBy(
  projects: readonly { id: string; name: string }[],
  project: string,
  name: string,
): string | undefined {
  const shown = (name.trim() || readable(project)).toLowerCase();
  const other = projects.find(
    (p) => p.id !== project && projectName([p], p.id).toLowerCase() === shown,
  );
  return other && projectName([other], other.id);
}

/** How a project shows: the name the CEO gave it, else its id made readable
 * (an id not among projects, too: a stale row, the list not read yet). */
export function projectName(
  projects: readonly { id: string; name: string }[] | undefined,
  id: string,
): string {
  return projects?.find((p) => p.id === id)?.name || readable(id);
}

/** Whether an agent's name already says it leads ("Lead", "Tech lead"),
 * so a "lead" tag beside it would only repeat it. */
export function saysLead(name: string): boolean {
  return /(^|[^a-z])lead/i.test(name); // not \b: QA_lead says it too
}
