import { useNavigate } from "@tanstack/react-router";
import type { Proposal, Soul, Team } from "../api/client";

// The Chief of Staff's page, opened from an entry point
// (wiki/product/features/chief-of-staff.md#the-page): what it was opened for
// becomes the session's opening line.
export type CosSearch = {
  start?: "create-team" | "hire" | "refine";
  team?: string;
  agent?: string;
};

export function cosSearch(s: Record<string, unknown>): CosSearch {
  const text = (v: unknown) => (typeof v === "string" && v !== "" ? v : undefined);
  const team = text(s.team);
  const agent = text(s.agent);
  if (s.start === "create-team") return { start: "create-team" };
  if (s.start === "hire" && team) return { start: "hire", team };
  if (s.start === "refine" && team && agent) return { start: "refine", team, agent };
  return {};
}

export function useOpenChiefOfStaff(): (search: CosSearch) => void {
  const navigate = useNavigate();
  return (search) => navigate({ to: "/chief-of-staff", search });
}

/** The session's opening line for what the page was opened for ("" for
 * none), typed in as the CEO's first words. */
export function openerOf(search: CosSearch, teams: Team[]): string {
  const team = teams.find((t) => t.id === search.team);
  const teamName = team?.config.name || search.team;
  switch (search.start) {
    case "create-team":
      return "Create a new team.";
    case "hire":
      return `Hire someone for ${teamName}.`;
    case "refine": {
      const agent = search.agent ?? "";
      const name = team?.config.agents[agent]?.name || agent;
      return `Refine ${name} of ${teamName}.`;
    }
    default:
      return "";
  }
}

/** The soul fields the preview shows, as they read. */
export const soulFields: { key: keyof Soul; label: string }[] = [
  { key: "name", label: "Name" },
  { key: "role", label: "Role" },
  { key: "personality", label: "Personality" },
  { key: "skills", label: "Skills" },
  { key: "instruction", label: "Job" },
];

/** A soul field as the preview reads it. */
export function soulText(soul: Soul, key: keyof Soul): string {
  const v = soul[key];
  return Array.isArray(v) ? v.join(", ") : typeof v === "string" ? v : "";
}

/** What a proposal does to each member: new, changed (with what each changed
 * field was), or the same; and who leaves. */
export function memberChanges(proposal: Proposal): {
  members: Record<
    string,
    { status: "new" | "changed" | "same"; was: Partial<Record<keyof Soul, string>> }
  >;
  leaving: string[];
} {
  const before = proposal.current?.agents ?? {};
  const members: ReturnType<typeof memberChanges>["members"] = {};
  for (const [id, soul] of Object.entries(proposal.team.agents ?? {})) {
    const old = before[id];
    if (!proposal.current || !old) {
      members[id] = { status: proposal.current ? "new" : "same", was: {} };
      continue;
    }
    const was: Partial<Record<keyof Soul, string>> = {};
    for (const { key } of soulFields) {
      if (soulText(soul, key) !== soulText(old, key)) was[key] = soulText(old, key);
    }
    members[id] = { status: Object.keys(was).length > 0 ? "changed" : "same", was };
  }
  const leaving = Object.keys(before).filter((id) => !proposal.team.agents?.[id]);
  return { members, leaving };
}
