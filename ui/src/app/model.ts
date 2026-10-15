import type { Instance, Project, Request, Team } from "../api/client";
import type { Soul } from "../api/types/workspace";

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
  const deployments = new Set(instances.map((i) => i.deploymentId)).size;
  return `${running} sessions working across ${deployments} deployments.`;
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
