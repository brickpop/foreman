import { describe, expect, it } from "vitest";
import {
  anyWords,
  cloneName,
  createdBody,
  defaultToSend,
  folderCrumbs,
  inside,
  nameFromFolder,
  newProjectHint,
  renameHint,
  repoIdLine,
} from "./addProject";

describe("cloneName", () => {
  it("is the URL's last part, without .git, as words", () => {
    expect(cloneName("git@github.com:me/ledger-api.git")).toBe("Ledger api");
    expect(cloneName("https://gitlab.com/acme/platform/deploy.git/")).toBe("Deploy");
    expect(cloneName("ssh://git@host:2222/~ceo/ledger")).toBe("Ledger");
    expect(cloneName("git@github.com:solo.git")).toBe("Solo");
    expect(cloneName("https://github.com/me/My Repo.git")).toBe("My repo");
    expect(cloneName("")).toBe("");
  });
});

describe("nameFromFolder", () => {
  it("is the folder's own name, as words that make its id", () => {
    expect(nameFromFolder("/Users/ceo/dev/ledger-api")).toBe("Ledger api");
    expect(nameFromFolder("/Users/ceo/dev/app/")).toBe("App");
    expect(nameFromFolder("/Users/ceo/dev/foreman_ui")).toBe("Foreman ui");
    expect(nameFromFolder("/x/My Repo (old)")).toBe("My repo old");
    expect(nameFromFolder("/x/café")).toBe("Cafe");
  });

  it("is empty when nothing is left", () => {
    expect(nameFromFolder("/")).toBe("");
    expect(nameFromFolder("")).toBe("");
    expect(nameFromFolder("/x/...")).toBe("");
  });
});

describe("repo hints", () => {
  it("shows the id words make, and the words as it shows", () => {
    expect(repoIdLine(" Ledger API ")).toBe("Its id: ledger-api. It shows as “Ledger API”.");
    expect(repoIdLine("!!!")).toBeUndefined();
  });

  it("says on rename whether the id stays or follows the words", () => {
    expect(renameHint("ledger-api", "Ledger API v2", "Ledger API")).toBe(
      "Its id becomes ledger-api-v2: agents' worktrees and open merge requests follow.",
    );
    expect(renameHint("ledger-api", "Ledger.API", "Ledger API")).toBe("Its id stays ledger-api.");
    expect(renameHint("Ledger_API", "ledger api", "Ledger API")).toBe(
      "Its id becomes ledger-api: agents' worktrees and open merge requests follow.",
    );
    expect(renameHint("ledger-api", "日本", "Ledger API")).toBe(anyWords);
  });

  it("changes nothing while the words are how the repo shows now, an older id too", () => {
    expect(renameHint("Ledger_API", " Ledger API ", "Ledger API")).toBe("Its id stays Ledger_API.");
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

describe("newProjectHint", () => {
  it("shows the id the typed words make, and the words, trimmed, as how it shows", () => {
    expect(newProjectHint("  My Café! ")).toBe(
      "Its id: my-cafe, in projects/my-cafe/ of Foreman’s folder. It shows as “My Café!”.",
    );
  });

  it("explains what the words become while they make no id", () => {
    expect(newProjectHint("")).toContain("Any words");
    expect(newProjectHint("!!!")).toContain("Any words");
  });
});
