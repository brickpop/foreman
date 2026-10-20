import { describe, expect, it } from "vitest";
import { inside, repoName } from "./addProject";

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
