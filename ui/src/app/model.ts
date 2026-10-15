import type { Instance, MergeRequest, Project, Request, Team } from "../api/client";
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

/** How many files a unified diff touches. */
export function filesChanged(diff: string): number {
  return diff.split("\n").filter((l) => l.startsWith("+++ ")).length;
}

/** A merge request's link line: its PR, its branch, and its checks, or for a
 * Chief of Staff change (no checks) how many files it changes. */
export function linkLine(mr: MergeRequest, forge: string, files?: number): string {
  const pr = mr.forgePrNumber ? `PR #${mr.forgePrNumber} on ${forgeLabel(forge)} · ` : "";
  const tail = mr.conflicting
    ? "conflicts with main"
    : mr.authorInstance === "cos"
      ? `${plural(files ?? 0, "file")} changed`
      : `checks ${mr.checksStatus || "running"}`;
  return `${pr}${mr.branch} → main · ${tail}`;
}
