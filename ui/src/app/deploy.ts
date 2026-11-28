import type { Deployment, Profile, Project, Team } from "../api/client";
import { byName, compareNames, projectName } from "./model";

// The Deploy dialog's choices (wiki/product/features/deploy-team.md): each list
// by name, the options that can't be picked last, greyed, with why.

export type Choice = { value: string; label: string; detail?: string; disabled?: boolean };

/** Whether the team is on the project already: deploying it there again does nothing.
 * Never before a team is picked. */
export function hasTeam(
  deployments: Deployment[],
  project: string,
  team: string | undefined,
): boolean {
  return !!team && deployments.some((d) => d.projectId === project && d.teamId === team);
}

/** The projects. One the team is on already can't be picked; one another team is on
 * can (the team there is replaced), and says which. */
export function projectChoices(
  projects: Project[],
  deployments: Deployment[],
  team: string | undefined,
  teamName: (id: string) => string,
): Choice[] {
  const choices = byName(projects).map((p): Choice => {
    const there = deployments.find((d) => d.projectId === p.id)?.teamId;
    const choice = { value: p.id, label: projectName([p], p.id) };
    if (!there) return choice;
    if (hasTeam(deployments, p.id, team))
      return { ...choice, detail: "already has this team", disabled: true };
    return { ...choice, detail: `${teamName(there)} is on it` };
  });
  return [...choices.filter((c) => !c.disabled), ...choices.filter((c) => c.disabled)];
}

export function teamChoices(teams: Team[]): Choice[] {
  return teams
    .map((t) => ({ value: t.id, label: t.config.name || t.id }))
    .sort((a, b) => compareNames(a.label, b.label));
}

/** The profiles, the connected ones first, each with its harness to tell them apart. A
 * profile not connected can still be picked: the dialog offers to connect it. */
export function profileChoices(profiles: Profile[], harness: (h: string) => string): Choice[] {
  const sorted = [...profiles].sort((a, b) => compareNames(a.name, b.name));
  return [...sorted.filter((p) => p.connected), ...sorted.filter((p) => !p.connected)].map((p) => ({
    value: p.name,
    label: p.name,
    detail: p.connected ? harness(p.harness) : `${harness(p.harness)} · not connected`,
  }));
}
