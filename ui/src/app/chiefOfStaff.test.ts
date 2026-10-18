import { describe, expect, it } from "vitest";
import { cosSearch } from "./chiefOfStaff";

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
