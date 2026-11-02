import { describe, expect, it } from "vitest";
import { settingsTab, withCurrent } from "./settings";

describe("settingsTab", () => {
  it("keeps a topic tab", () => {
    for (const tab of ["teams", "agents", "git", "look", "system"]) {
      expect(settingsTab(tab)).toBe(tab);
    }
  });

  it("opens Teams for the old tabs and anything else", () => {
    for (const tab of ["workspace", "global", "Git", "", 3, undefined, ["git"]]) {
      expect(settingsTab(tab)).toBe("teams");
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
