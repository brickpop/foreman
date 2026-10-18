import { describe, expect, it } from "vitest";
import type { Soul, Team } from "../api/client";
import type { TeamConfig } from "../api/types/workspace";
import { cosSearch, memberChanges, openerOf } from "./chiefOfStaff";

describe("the Chief of Staff's entry points", () => {
  it("keeps what each one needs", () => {
    expect(cosSearch({ start: "create-team", team: "atlas" })).toEqual({ start: "create-team" });
    expect(cosSearch({ start: "hire", team: "atlas" })).toEqual({ start: "hire", team: "atlas" });
    expect(cosSearch({ start: "refine", team: "atlas", agent: "mara" })).toEqual({
      start: "refine",
      team: "atlas",
      agent: "mara",
    });
  });

  it("opens with no opening line when an entry is incomplete or unknown", () => {
    expect(cosSearch({})).toEqual({});
    expect(cosSearch({ start: "hire" })).toEqual({});
    expect(cosSearch({ start: "hire", team: "" })).toEqual({});
    expect(cosSearch({ start: "refine", team: "atlas" })).toEqual({});
    expect(cosSearch({ start: "project-config", project: "ledger" })).toEqual({});
    expect(cosSearch({ start: "hire", team: 7 })).toEqual({});
  });
});

describe("the session's opening line", () => {
  const teams = [
    { id: "atlas", config: { name: "Atlas", agents: { mara: { name: "Mara" } } } },
  ] as unknown as Team[];
  it("says what the page was opened for, by name", () => {
    expect(openerOf({ start: "create-team" }, teams)).toBe("Create a new team.");
    expect(openerOf({ start: "hire", team: "atlas" }, teams)).toBe("Hire someone for Atlas.");
    expect(openerOf({ start: "refine", team: "atlas", agent: "mara" }, teams)).toBe(
      "Refine Mara of Atlas.",
    );
    expect(openerOf({}, teams)).toBe("");
  });
  it("falls back to ids for what it doesn't know (yet)", () => {
    expect(openerOf({ start: "hire", team: "vesper" }, teams)).toBe("Hire someone for vesper.");
    expect(openerOf({ start: "refine", team: "atlas", agent: "ghost" }, [])).toBe(
      "Refine ghost of atlas.",
    );
  });
});

describe("what a proposal changes", () => {
  const soul = (name: string, extra: Partial<Soul> = {}) =>
    ({ name, role: "engineer", personality: "", skills: [], instruction: "", ...extra }) as Soul;
  const team = (agents: Record<string, Soul>) => ({ team: "atlas", agents }) as TeamConfig;

  it("marks nothing in a new team", () => {
    const { members, leaving } = memberChanges({ id: 1, team: team({ mara: soul("Mara") }) });
    expect(members.mara).toEqual({ status: "same", was: {} });
    expect(leaving).toEqual([]);
  });

  it("marks the new member of a hire, the changed fields of a refine, and who leaves", () => {
    const current = team({
      mara: soul("Mara"),
      tove: soul("Tove", { skills: ["go"], instruction: "Build." }),
      ilya: soul("Ilya"),
    });
    const proposed = team({
      mara: soul("Mara"),
      tove: soul("Tove", { skills: ["go", "sql"], instruction: "Build. Adjustment: push back." }),
      arden: soul("Arden"),
    });
    const { members, leaving } = memberChanges({ id: 2, team: proposed, current });
    expect(members.mara).toEqual({ status: "same", was: {} });
    expect(members.arden).toEqual({ status: "new", was: {} });
    expect(members.tove).toEqual({
      status: "changed",
      was: { skills: "go", instruction: "Build." },
    });
    expect(leaving).toEqual(["ilya"]);
  });
});
