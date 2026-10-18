import type { Instance, MergeRequest, Project, Request, Settings, Team } from "../api/client";
import type { Look, Soul } from "../api/types/workspace";

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

/** Home's title (wiki/product/features/home.md#header). */
export function homeTitle(open: number): string {
  if (open === 0) return "All clear";
  return open === 1 ? "1 thing needs you" : `${open} things need you`;
}

/** Home's subtitle: running sessions across active deployments. */
export function homeSubtitle(instances: Instance[]): string {
  const running = instances.filter((i) => i.status === "running").length;
  const deployments = new Set(instances.map((i) => i.deploymentId).filter(Boolean)).size;
  return `${plural(running, "session")} working across ${plural(deployments, "deployment")}.`;
}

export type ProjectRow = {
  id: string;
  line: string;
  state: "needs you" | "on track" | "idle";
};

/** One row per project: its teams and working agents, and its state. */
export function projectRows(
  projects: Project[],
  teams: Team[],
  instances: Instance[],
  requests: Request[],
): ProjectRow[] {
  return projects.map((p) => {
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

/** A checks result as words (Home's review text). */
export function checksSentence(status: string): string {
  switch (status) {
    case "pass":
      return " Checks pass.";
    case "fail":
      return " Checks fail.";
    case "running":
      return " Checks running.";
    default:
      return "";
  }
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

/** The profile a project's agents run on: its own (settings.toml's
 * project_profiles), else the default. */
export function profileOf(settings: Settings | undefined, project: string): string {
  return settings?.projectProfiles?.[project] || "default";
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
export function resolveWho(team: Team, who: string): { text: string; known: boolean } {
  const agents = team.config.agents;
  if (who === "lead") return { text: agents[team.config.lead]?.name ?? who, known: true };
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
 * file's order, then any it doesn't name, sorted (as workspace.TeamConfig.Roster). */
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

/** A merge request's link line: its PR, its branch, and its checks. */
export function linkLine(mr: MergeRequest, forge: string, main = "main"): string {
  const pr = mr.forgePrNumber ? `PR #${mr.forgePrNumber} on ${forgeLabel(forge)} · ` : "";
  // No checks status: the project has no checks (or they haven't run): nothing to say.
  const tail = mr.conflicting
    ? `conflicts with ${main}`
    : mr.checksStatus
      ? `checks ${mr.checksStatus}`
      : "";
  return `${pr}${mr.branch} → ${main}${tail ? ` · ${tail}` : ""}`;
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
