import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import type { ActionRepo, TeamAction } from "../api/client";
import { ActionRow } from "./ActivityPanel";

const repo = (r: Partial<ActionRepo>): ActionRepo => ({
  repo: "api",
  branch: "wt/tove/main",
  message: "",
  commit: "",
  created: false,
  conflicts: [],
  ...r,
});

const commit = (repos: ActionRepo[], error = ""): TeamAction => ({
  id: 1,
  deploymentId: "atlas@ledger",
  instance: "tove@ledger",
  tool: "commit",
  branch: "",
  repos,
  error,
  createdAt: "2026-10-17T10:00:00Z",
});

const text = (a: TeamAction) =>
  renderToStaticMarkup(<ActionRow action={a} name={() => "Tove"} />).replace(/<[^>]+>/g, "");

// Each repo of a commit has its own message (protocol.md#commit).
describe("a commit's row", () => {
  it("puts one repo's message in the headline", () => {
    const got = text(commit([repo({ message: "cal v0", commit: "1a2b3c4d5e" })]));
    expect(got).toContain("committed “cal v0”");
    expect(got.match(/cal v0/g)).toHaveLength(1);
  });

  it("names several repos, each line with its message", () => {
    const got = text(
      commit([
        repo({ message: "cal v1", commit: "1a2b3c4d5e" }),
        repo({ repo: "project", branch: "main", message: "the board", commit: "5e6f7a8b9c" }),
      ]),
    );
    expect(got).toContain("committed api, project");
    expect(got).toContain("1a2b3c4“cal v1”");
    expect(got).toContain("5e6f7a8“the board”");
  });

  it("keeps the message of a refused one", () => {
    const got = text(
      commit([repo({ message: "wip" })], "nothing_to_commit: nothing changed in api"),
    );
    expect(got).toContain("tried to commit “wip”");
    expect(got).toContain("refused");
  });
});
