import { describe, expect, it } from "vitest";
import { hasUnsentLine, setUnsentLine, unsentLine } from "./unsentLines";

describe("unsent lines", () => {
  it("keeps each agent's line apart, until it is emptied", () => {
    setUnsentLine("mara@p", "check the tests");
    setUnsentLine("tove@p", "and you, the docs");
    expect(unsentLine("mara@p")).toBe("check the tests");
    expect(unsentLine("tove@p")).toBe("and you, the docs");
    setUnsentLine("mara@p", "");
    expect(unsentLine("mara@p")).toBe("");
    expect(unsentLine("tove@p")).toBe("and you, the docs");
    setUnsentLine("tove@p", "");
  });

  it("counts for a reload only the agents still there, and only non-empty lines", () => {
    expect(hasUnsentLine(["mara@p"])).toBe(false);
    setUnsentLine("mara@p", "x");
    expect(hasUnsentLine(["tove@p", "mara@p"])).toBe(true);
    expect(hasUnsentLine(["tove@p"])).toBe(false); // mara left the roster
    expect(hasUnsentLine([])).toBe(false);
    setUnsentLine("mara@p", "");
    expect(hasUnsentLine(["mara@p"])).toBe(false);
  });

  it("keeps whitespace-only and multi-line text as typed", () => {
    // An empty Line still sends Enter, but a space or a line break is text a reload would lose.
    setUnsentLine("mara@p", " ");
    expect(hasUnsentLine(["mara@p"])).toBe(true);
    setUnsentLine("mara@p", "one\ntwo");
    expect(unsentLine("mara@p")).toBe("one\ntwo");
    setUnsentLine("mara@p", "");
  });
});
