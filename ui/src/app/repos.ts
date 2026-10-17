import type { Team } from "../api/client";
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
