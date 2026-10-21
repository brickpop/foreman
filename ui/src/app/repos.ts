import type { Repo, RepoBranch, Team } from "../api/client";
import { agentOf } from "./model";

// The Repositories tab's model (wiki/product/features/repositories.md).

/** Whose branch it is, by Foreman's names: wt/<agent>/… is that agent's,
 * lead/<team>/… that team's lead's; anything else is nobody's in particular. */
export function branchOwner(branch: string, teams: Team[]): string | undefined {
  const [kind, who] = branch.split("/");
  if (kind === "wt" && who) return agentOf(teams, who)?.soul.name ?? who;
  if (kind === "lead" && who) {
    const team = teams.find((t) => t.id === who);
    return team ? (team.config.agents[team.config.lead]?.name ?? team.config.lead) : who;
  }
  return undefined;
}

/** The repo and branch Repositories shows: the ones asked for (the URL) when
 * the project has them, else the repo's first branch (main, when it has
 * one), else the first repo's. */
export function shownAt(
  repos: Repo[],
  repo?: string,
  branch?: string,
): { repo: string; branch: string } | undefined {
  const r = repos.find((x) => x.name === repo) ?? repos[0];
  if (!r) return undefined;
  const b = r.branches.find((x) => x.name === branch) ?? r.branches[0];
  return { repo: r.name, branch: b?.name ?? r.main };
}

export type Worktree = {
  agent: string;
  /** The branch checked out in each of its repos, in the repos' order. */
  branches: { repo: string; branch: RepoBranch }[];
};

/** The agents' worktrees, from the branches checked out in each repo: one
 * per agent, by agent id. */
export function worktrees(repos: Repo[]): Worktree[] {
  const by = new Map<string, Worktree>();
  for (const repo of repos) {
    for (const branch of repo.branches) {
      if (!branch.agent) continue;
      const w = by.get(branch.agent) ?? { agent: branch.agent, branches: [] };
      w.branches.push({ repo: repo.name, branch });
      by.set(branch.agent, w);
    }
  }
  return [...by.values()].sort((a, b) => a.agent.localeCompare(b.agent));
}

/** The repos that have a branch, those with commits beyond their main first:
 * where a work item's branch can be read. */
export function reposWith(repos: Repo[], branch: string): string[] {
  const having = repos.filter((r) => r.branches.some((b) => b.name === branch));
  const ahead = (r: Repo) => (r.branches.find((b) => b.name === branch)?.ahead ?? 0) > 0;
  return [...having.filter(ahead), ...having.filter((r) => !ahead(r))].map((r) => r.name);
}

/** A path's breadcrumbs: each folder (and the file) with the path to it. */
export function crumbs(path: string): { name: string; path: string }[] {
  const parts = path.split("/").filter(Boolean);
  return parts.map((name, i) => ({ name, path: parts.slice(0, i + 1).join("/") }));
}

/** The folder a path sits in ("" at the root). */
export function folderOf(path: string): string {
  return path.split("/").slice(0, -1).join("/");
}

export const isMarkdown = (path: string) => /\.(md|markdown)$/i.test(path);

/** A size as people read it: 12 B, 3.4 KB, 1.2 MB. */
export function bytes(n: number): string {
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
  return `${(n / 1024 / 1024).toFixed(1)} MB`;
}
