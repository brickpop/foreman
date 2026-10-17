import { describe, expect, it } from "vitest";
import type { Team } from "../api/client";
import { branchOwner, bytes, crumbs, folderOf, isMarkdown } from "./repos";

const atlas = {
  id: "atlas",
  config: {
    lead: "mara",
    agents: { mara: { name: "Mara" }, tove: { name: "Tove" } },
  },
} as unknown as Team;

describe("the Repositories tab's model", () => {
  it("names whose branch it is", () => {
    expect(branchOwner("wt/tove/LED-44", [atlas])).toBe("Tove");
    expect(branchOwner("lead/atlas/integration", [atlas])).toBe("Mara");
    expect(branchOwner("wt/gone/x", [atlas])).toBe("gone"); // a soul since removed
    expect(branchOwner("main", [atlas])).toBeUndefined();
    expect(branchOwner("feature/x", [atlas])).toBeUndefined();
  });

  it("breaks a path into crumbs and finds its folder", () => {
    expect(crumbs("src/lib/a.go")).toEqual([
      { name: "src", path: "src" },
      { name: "lib", path: "src/lib" },
      { name: "a.go", path: "src/lib/a.go" },
    ]);
    expect(crumbs("")).toEqual([]);
    expect(folderOf("src/lib/a.go")).toBe("src/lib");
    expect(folderOf("README.md")).toBe("");
  });

  it("tells Markdown and sizes", () => {
    expect(isMarkdown("docs/README.MD")).toBe(true);
    expect(isMarkdown("main.go")).toBe(false);
    expect([bytes(12), bytes(3482), bytes(1258291)]).toEqual(["12 B", "3.4 KB", "1.2 MB"]);
  });
});
