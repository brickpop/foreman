import { useNavigate } from "@tanstack/react-router";

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
