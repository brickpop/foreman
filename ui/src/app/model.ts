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

import type { Deployment, Instance, MergeRequest, Project, Request, Team } from "../api/client";
import type { Look, Soul, TeamConfig } from "../api/types/home";

// Pure derivations the screens share: who an instance is, how requests are
// ordered, what a project's state is. Kept apart from components to be tested.

export type Agent = { id: string; soul: Soul; team: Team };

/** The agent behind an instance id (agent@project), from the team files. */
export function agentOf(teams: Team[], instance: string): Agent | undefined {
  const id = instance.split("@")[0];
  for (const team of teams) {
    const soul = team.config.agents[id];
    if (soul) return { id, soul, team };
  }
  return undefined;
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
 * Overview): every kind but reviews, which are decided in Reviews
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

/** The subtitle of the Dashboard and Sessions: running sessions across active deployments. */
export function workingSummary(instances: Instance[]): string {
  const running = instances.filter((i) => i.status === "running").length;
  const deployments = new Set(instances.map((i) => i.deploymentId).filter(Boolean)).size;
  return `${plural(running, "session")} working across ${plural(deployments, "deployment")}.`;
}

export type SessionGroup = {
  deployment: string;
  project: string;
  team?: Team;
  goal: string;
  /** A request of its team waits on the CEO. */
  needsYou: boolean;
  /** Its awake agents, the lead first, then the team's order. */
  agents: { instance: Instance; agent?: Agent; lead: boolean }[];
  /** Its asleep agents (not started yet, woken when needed), in the same
   * order: only counted, they'd be noise as rows. */
  asleep: { instance: Instance; agent?: Agent; lead: boolean }[];
};

/** Every active deployment, one group per deployment, by project then team,
 * its asleep agents apart from the others. */
export function sessionGroups(
  instances: Instance[],
  deployments: Deployment[],
  teams: Team[],
  requests: Request[],
): SessionGroup[] {
  const groups = new Map<string, SessionGroup>();
  for (const i of instances) {
    if (!i.deploymentId) continue;
    let g = groups.get(i.deploymentId);
    if (!g) {
      const [teamId, project] = i.deploymentId.split("@");
      g = {
        deployment: i.deploymentId,
        project: project ?? "",
        team: teams.find((t) => t.id === teamId),
        goal: deployments.find((d) => d.id === i.deploymentId)?.goal ?? "",
        needsYou: requests.some((r) => r.status === "open" && r.deploymentId === i.deploymentId),
        agents: [],
        asleep: [],
      };
      groups.set(i.deploymentId, g);
    }
    (i.status === "asleep" ? g.asleep : g.agents).push({
      instance: i,
      agent: agentOf(teams, i.id),
      lead: g.team?.config.lead === i.agentId,
    });
  }
  return [...groups.values()]
    .map((g) => {
      const order = g.team ? roster(g.team.config) : [];
      const rank = (id: string) => (order.includes(id) ? order.indexOf(id) : order.length);
      const byRank = (a: SessionGroup["agents"][number], b: SessionGroup["agents"][number]) =>
        rank(a.instance.agentId) - rank(b.instance.agentId) ||
        a.instance.agentId.localeCompare(b.instance.agentId);
      return { ...g, agents: [...g.agents].sort(byRank), asleep: [...g.asleep].sort(byRank) };
    })
    .sort((a, b) => a.project.localeCompare(b.project) || a.deployment.localeCompare(b.deployment));
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

/** One row per project not archived: its teams and working agents, and its state. */
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

/** The avatar's palettes and traits (wiki/product/design/avatar.md#palettes). */
export const palettes = {
  color: ["#D9825B", "#8DB86B", "#6FA8DC", "#B48BD6", "#E07A9A", "#4FB3A9", "#CDB46B", "#8E9AAF"],
  skin: ["#F1C9A5", "#E2AE85", "#C38A62", "#8D5A3B", "#5E3B26"],
  hairColor: ["#2B2420", "#6B4A2F", "#D8B46A", "#A8A8A8", "#A4482C"],
  hair: ["short", "long", "curly", "bun", "bald"],
  eyes: ["dots", "glasses", "shades", "sleepy"],
  beard: ["none", "stubble", "full", "mustache"],
} as const;

/** A random face and background (Shuffle, and a new hire). */
export function randomLook(
  pick: (n: number) => number = (n) => Math.floor(Math.random() * n),
): Look {
  const one = <T>(list: readonly T[]) => list[pick(list.length)];
  return {
    color: one(palettes.color),
    skin: one(palettes.skin),
    hair: one(palettes.hair),
    hairColor: one(palettes.hairColor),
    eyes: one(palettes.eyes),
    beard: one(palettes.beard),
  };
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
export function linkLine(mr: MergeRequest, forge: string, main = "main"): string {
  const pr = mr.forgePrNumber ? `PR #${mr.forgePrNumber} on ${forgeLabel(forge)} · ` : "";
  // No checks status: the project has no checks (or they haven't run): nothing to say.
  const tail = mr.conflicting
    ? `conflicts with ${main}`
    : mr.checksStatus
      ? `checks ${mr.checksStatus}`
      : "";
  // The repo first: a review ticks its repos one by one.
  return `${pr}${mr.repo} · ${mr.branch} → ${main}${tail ? ` · ${tail}` : ""}`;
}

/** The main branch of a project's repo, which its merges go to; main until
 * the project has loaded. */
export function mainOf(project: Project | undefined, repo: string): string {
  return project?.repos.find((r) => r.name === repo)?.main ?? "main";
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

/** Why a message to an agent still waits, as the Activity panel says it
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
