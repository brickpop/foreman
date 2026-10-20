import { describe, expect, it } from "vitest";
import { settingsSearch } from "./settings";

describe("settingsSearch", () => {
  it("keeps a topic tab, Agents left out of the URL", () => {
    expect(settingsSearch({ tab: "git" })).toEqual({ tab: "git" });
    expect(settingsSearch({ tab: "look" })).toEqual({ tab: "look" });
    expect(settingsSearch({ tab: "system" })).toEqual({ tab: "system" });
    expect(settingsSearch({ tab: "agents" })).toEqual({ tab: undefined });
  });

  it("opens Agents for the old tabs and anything else", () => {
    for (const tab of ["workspace", "global", "Git", "", 3, undefined, ["git"]]) {
      expect(settingsSearch({ tab })).toEqual({ tab: undefined });
    }
  });
});
