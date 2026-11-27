import { describe, expect, it } from "vitest";
import type { Deployment, Instance } from "../api/client";
import { replacingLine, sessionsEnd } from "./model";
import {
  deploymentOn,
  latestStopped,
  projectHome,
  reviewOf,
  sectionOf,
  sections,
  sessionOf,
  teamOn,
  teamView,
  windowTitle,
} from "./project";

const inst = (id: string, deploymentId: string) => ({ id, deploymentId }) as Instance;

describe("a project's page", () => {
  it("names the project in the window's title", () => {
    expect(windowTitle("Evm ecosystem")).toBe("Evm ecosystem · Foreman");
    expect(windowTitle(undefined)).toBe("Foreman");
  });

  it("lists its sections in the CEO's order", () => {
    expect(sections.map((s) => s.label)).toEqual([
      "Session",
      "Inbox",
      "Board",
      "Repos",
      "Wiki",
      "Team",
      "Settings",
    ]);
  });

  it("finds the section a page belongs to, a repo's or a review's page included", () => {
    expect(sectionOf("/projects/a/board")?.path).toBe("board");
    expect(sectionOf("/projects/a/repos/api")?.label).toBe("Repos");
    expect(sectionOf("/projects/a/inbox/rev-3")?.path).toBe("inbox");
    expect(sectionOf("/projects/a")).toBeUndefined();
    expect(sectionOf("/projects/a/deployment")).toBeUndefined();
    expect(sectionOf("/settings/look")).toBeUndefined();
  });

  it("opens on Session while a team is on it, on Wiki otherwise", () => {
    const instances = [inst("mara@ledger", "atlas@ledger"), inst("lumen@relay", "orion@relay")];
    expect(projectHome("ledger", instances)).toBe("session");
    expect(projectHome("harbor", instances)).toBe("wiki");
    expect(projectHome("ledger", [])).toBe("wiki");
  });

  it("finds the team on it by the project, not by a name that ends alike", () => {
    const instances = [inst("mara@ledger", "atlas@ledger")];
    expect(teamOn("ledger", instances)).toBe("atlas@ledger");
    expect(teamOn("dger", instances)).toBeUndefined();
    // An instance outside any deployment (none listed today) names no team.
    expect(teamOn("ledger", [inst("x@ledger", "")])).toBeUndefined();
  });

  it("links an agent's session and a review into their project", () => {
    expect(sessionOf("mara@ledger")).toEqual({
      to: "/projects/$project/session",
      params: { project: "ledger" },
      search: { agent: "mara" },
    });
    expect(reviewOf("ledger", "rev-3")).toEqual({
      to: "/projects/$project/inbox/$review",
      params: { project: "ledger", review: "rev-3" },
    });
  });

  const stopped = (id: string, stoppedAt: string) =>
    ({ id, teamId: id.split("@")[0], projectId: id.split("@")[1], stoppedAt }) as Deployment;
  const list = [
    stopped("atlas@ledger", "2026-10-20T10:00:00Z"),
    stopped("orion@ledger", "2026-10-30T10:00:00Z"),
    stopped("vesper@relay", "2026-10-31T10:00:00Z"),
  ];

  it("finds the team that worked on it last, from its stopped deployments only", () => {
    expect(latestStopped("ledger", list)?.teamId).toBe("orion");
    expect(latestStopped("harbor", list)).toBeUndefined();
  });

  it("finds its deployment: the running one, else the one that stopped last", () => {
    const running = [{ id: "atlas@relay", projectId: "relay" } as Deployment];
    expect(deploymentOn("relay", running, list)?.id).toBe("atlas@relay");
    expect(deploymentOn("ledger", running, list)?.id).toBe("orion@ledger");
    expect(deploymentOn("harbor", running, list)).toBeUndefined();
  });
});

describe("deploying onto a project with a team", () => {
  it("says the team there is replaced, and what ends", () => {
    expect(replacingLine("Atlas", "Big ledger", 3)).toBe(
      "Replaces Atlas on Big ledger: 3 sessions end. Worktrees and branches are kept.",
    );
    expect(sessionsEnd(1)).toBe("1 session ends");
    expect(sessionsEnd(0)).toBe("0 sessions end");
  });
});

describe("teamView", () => {
  it("opens on Agents while there are agents, else In flight", () => {
    expect(teamView(undefined, true)).toBe("agents");
    expect(teamView(undefined, false)).toBe("inflight");
  });

  it("keeps the view asked for, and ignores an unknown one", () => {
    expect(teamView("merged", true)).toBe("merged");
    expect(teamView("agents", false)).toBe("agents");
    expect(teamView("activity", false)).toBe("activity");
    for (const asked of ["Merged", "", 3, ["merged"]]) {
      expect(teamView(asked, true)).toBe("agents");
    }
  });
});
