import { describe, expect, it } from "vitest";
import { settingsSearch, sleepLabel, withCurrent } from "./settings";

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

describe("withCurrent", () => {
  it("keeps a hand-written value among the choices, in order", () => {
    expect(withCurrent([10, 50, 100], 50)).toEqual([10, 50, 100]);
    expect(withCurrent([10, 50, 100], 37)).toEqual([10, 37, 50, 100]);
    expect(withCurrent([10, 50, 100], 5000)).toEqual([10, 50, 100, 5000]);
  });
});

describe("sleepLabel", () => {
  it("names 0 never, any other value its minutes", () => {
    expect(sleepLabel(0)).toBe("Never");
    expect(sleepLabel(20)).toBe("After 20 min idle");
    expect(sleepLabel(7)).toBe("After 7 min idle");
  });
});
