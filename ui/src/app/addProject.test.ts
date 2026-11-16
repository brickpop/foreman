import { describe, expect, it } from "vitest";
import {
  createdBody,
  defaultToSend,
  folderCrumbs,
  inside,
  projectSlug,
  repoName,
} from "./addProject";

describe("repoName", () => {
  it("is the folder's own name", () => {
    expect(repoName("/Users/ceo/dev/ledger-api")).toBe("ledger-api");
    expect(repoName("/Users/ceo/dev/app/")).toBe("app");
    expect(repoName("/Users/ceo/dev/v1.2_core")).toBe("v1.2_core");
  });

  it("follows the server's rule", () => {
    expect(repoName("/x/My Repo (old)")).toBe("My-Repo-old-");
    expect(repoName("/x/.dotfiles")).toBe("dotfiles");
    expect(repoName("/x/-_lead")).toBe("lead");
    expect(repoName("/x/a..b")).toBe("a.b");
    expect(repoName("/x/café")).toBe("caf-");
    expect(repoName(`/x/${"a".repeat(80)}`)).toHaveLength(63);
  });

  it("is empty when nothing is left", () => {
    expect(repoName("/")).toBe("");
    expect(repoName("")).toBe("");
    expect(repoName("/x/...")).toBe("");
  });
});

describe("inside", () => {
  it("joins without doubling the slash", () => {
    expect(inside("/Users/ceo/dev", "api")).toBe("/Users/ceo/dev/api");
    expect(inside("/Users/ceo/dev/", "api")).toBe("/Users/ceo/dev/api");
  });
});

describe("createdBody", () => {
  const path = "/Users/ceo/foreman/projects/ledger/hq";
  it("says a taken-over HQ was kept as it was, whatever was asked", () => {
    expect(createdBody(false, false, path)).toBe(`Took over the HQ left at ${path}, as it was.`);
  });

  it("tells a clone from a new HQ", () => {
    expect(createdBody(true, true, path)).toBe(`Cloned its HQ into ${path}.`);
    expect(createdBody(true, false, path)).toContain("with project.toml and a starter wiki");
  });
});

describe("defaultToSend", () => {
  const poc = { branches: ["feature", "master"], current: "feature" };
  it("sends nothing for a repo with a main: the server takes it", () => {
    expect(defaultToSend({ branches: ["main", "master"], current: "master" })).toBeUndefined();
    expect(defaultToSend({ branches: ["main"], current: "main" }, "main")).toBeUndefined();
  });
  it("sends nothing until the branches have loaded", () => {
    expect(defaultToSend(undefined, "master")).toBeUndefined();
  });
  it("defaults to the checkout's branch, else the first", () => {
    expect(defaultToSend(poc)).toBe("feature");
    expect(defaultToSend({ branches: ["dev", "master"], current: "" })).toBe("dev");
  });
  it("takes the CEO's pick, but only one the repo has", () => {
    expect(defaultToSend(poc, "master")).toBe("master");
    expect(defaultToSend(poc, "gone")).toBe("feature");
  });
});

describe("projectSlug", () => {
  it("makes what the CEO typed the slug the server takes", () => {
    expect(projectSlug("My Portfolio")).toBe("my-portfolio");
    expect(projectSlug("  Café  Crème! ")).toBe("cafe-creme");
    expect(projectSlug("ledger_v2.1")).toBe("ledger-v2-1");
    expect(projectSlug("--already-a-slug--")).toBe("already-a-slug");
    expect(projectSlug("!!!")).toBe("");
    expect(projectSlug("日本")).toBe("");
  });

  it("never ends on a hyphen when cut to 63", () => {
    const long = projectSlug(`${"a".repeat(62)} b`);
    expect(long).toBe("a".repeat(62));
    expect(projectSlug("x".repeat(80))).toHaveLength(63);
  });
});

describe("folderCrumbs", () => {
  it("starts at the home folder, nothing above it", () => {
    expect(folderCrumbs("/Users/ana/dev/app", "/Users/ana")).toEqual([
      { name: "ana", path: "/Users/ana" },
      { name: "dev", path: "/Users/ana/dev" },
      { name: "app", path: "/Users/ana/dev/app" },
    ]);
    expect(folderCrumbs("/Users/ana", "/Users/ana/")).toEqual([
      { name: "ana", path: "/Users/ana" },
    ]);
  });
  it("doesn't take a sibling sharing the home folder's prefix for inside it", () => {
    expect(folderCrumbs("/Users/anabel/x", "/Users/ana")).toEqual([
      { name: "x", path: "/Users/anabel/x" },
    ]);
  });
  it("keeps names with spaces and dots whole", () => {
    expect(folderCrumbs("/home/u/My Repos/v1.2", "/home/u").map((c) => c.name)).toEqual([
      "u",
      "My Repos",
      "v1.2",
    ]);
  });
  it("works with / as home", () => {
    expect(folderCrumbs("/srv/a", "/")).toEqual([
      { name: "/", path: "/" },
      { name: "srv", path: "/srv" },
      { name: "a", path: "/srv/a" },
    ]);
  });
});
