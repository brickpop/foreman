import { describe, expect, it } from "vitest";
import type { Deployment, Instance } from "../api/client";
import { replacingLine, sessionsEnd } from "./model";
import {
  deploymentOn,
  deploymentView,
  latestFirst,
  latestStopped,
  projectHome,
  reviewOf,
  sections,
  sessionOf,
  teamOn,
} from "./project";

const inst = (id: string, deploymentId: string) => ({ id, deploymentId }) as Instance;

describe("a project's page", () => {
  it("lists its sections in the CEO's order", () => {
    expect(sections.map((s) => s.label)).toEqual([
      "Session",
      "Inbox",
      "Wiki",
      "Board",
      "Repos",
      "Deployment",
      "Activity",
      "Settings",
    ]);
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
    expect(replacingLine("Atlas", "big-ledger", 3)).toBe(
      "Replaces Atlas on Big ledger: 3 sessions end. Worktrees and branches are kept.",
    );
    expect(sessionsEnd(1)).toBe("1 session ends");
    expect(sessionsEnd(0)).toBe("0 sessions end");
  });
});

describe("the projects, latest first", () => {
  const p = (...ids: string[]) => ids.map((id) => ({ id }));
  it("orders by last activity, by time rather than text", () => {
    const latest = {
      ledger: "2026-11-01T09:00:00Z",
      relay: "2026-11-01T09:00:00.5Z", // half a second later, though it sorts lower as text
      quiet: "2025-01-01T00:00:00Z",
    };
    expect(latestFirst(p("ledger", "quiet", "relay"), latest).map((x) => x.id)).toEqual([
      "relay",
      "ledger",
      "quiet",
    ]);
  });
  it("puts a project added since the page loaded first, and ties by name", () => {
    const latest = { b: "2026-11-01T09:00:00Z", a: "2026-11-01T09:00:00Z" };
    expect(latestFirst(p("a", "b", "new"), latest).map((x) => x.id)).toEqual(["new", "a", "b"]);
  });
  it("leaves the order as it is until the times are in", () => {
    const list = p("b", "a");
    expect(latestFirst(list, undefined)).toBe(list);
  });
});

describe("deploymentView", () => {
  it("opens on Agents while there are agents, else In flight", () => {
    expect(deploymentView(undefined, true)).toBe("agents");
    expect(deploymentView(undefined, false)).toBe("inflight");
  });

  it("keeps the view asked for, and ignores an unknown one", () => {
    expect(deploymentView("merged", true)).toBe("merged");
    expect(deploymentView("agents", false)).toBe("agents");
    for (const asked of ["Merged", "", 3, ["merged"]]) {
      expect(deploymentView(asked, true)).toBe("agents");
    }
  });
});
