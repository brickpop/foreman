import { afterEach, describe, expect, it, vi } from "vitest";
import type { Instance, Request, Team } from "../api/client";
import { type Notice, noticeFor, notifiesBrowser, permission } from "./notifications";

const team = {
  id: "atlas",
  config: { lead: "mara", agents: { mara: { name: "Mara" }, tove: { name: "Tove" } } },
} as unknown as Team;
const instances = [{ id: "tove@ledger", deploymentId: "atlas@ledger" }] as Instance[];

const request = (r: Partial<Request>) =>
  ({
    type: "request.created",
    data: {
      kind: "question",
      projectId: "ledger",
      deploymentId: "atlas@ledger",
      summary: "Which budget?",
      instanceId: "tove@ledger",
      ...r,
    },
  }) as const;

describe("noticeFor", () => {
  it("tells who needs the CEO, where, and links the session", () => {
    expect(noticeFor(request({}), [team], instances)).toMatchObject({
      toggle: "needsMe",
      title: "Tove needs you on Ledger",
      body: "Which budget?",
      instance: "tove@ledger",
      deployment: "atlas@ledger",
    });
  });

  it("names the agent as its own team has it, when another team shares the id", () => {
    const orion = { id: "orion", config: { lead: "tove", agents: { tove: { name: "QA" } } } };
    const both = [orion as unknown as Team, team];
    expect(noticeFor(request({}), both, instances)?.title).toBe("Tove needs you on Ledger");
    expect(noticeFor(request({ deploymentId: "orion@ledger" }), both, instances)?.title).toBe(
      "QA needs you on Ledger",
    );
  });

  it("calls a review a merge request, gated by its own toggle", () => {
    const n = noticeFor(
      request({
        kind: "review",
        instanceId: "mara@ledger",
        summary: "Settlement v2",
        reviewId: "rev-3",
      }),
      [team],
      instances,
    );
    expect(n).toMatchObject({
      toggle: "mergeRequested",
      title: "Mara requested a merge on Ledger",
      review: "rev-3",
    });
  });

  it("tells a withdrawal, warning of the drafts lost with it", () => {
    const withdrawn = (by: string, lostDrafts: number) =>
      noticeFor(
        {
          type: "review.withdrawn",
          data: {
            review: {
              id: "rev-3",
              branch: "lead/ledger/atlas/small",
              withdrawnReason: "all of it is in big",
            },
            deploymentId: "atlas@ledger",
            lead: "mara@ledger",
            by,
            lostDrafts,
          },
        },
        [team],
        instances,
      );
    expect(withdrawn("lead", 0)).toEqual({
      title: "Mara withdrew lead/ledger/atlas/small",
      body: "all of it is in big.",
      instance: "mara@ledger",
      deployment: "atlas@ledger",
      review: "rev-3",
      color: "var(--tx4)",
    });
    expect(withdrawn("foreman", 1)).toMatchObject({
      title: "Foreman withdrew lead/ledger/atlas/small",
      body: "all of it is in big. Your draft comment on it is lost.",
      color: "var(--err)",
    });
    expect(withdrawn("lead", 3)?.body).toBe(
      "all of it is in big. Your 3 draft comments on it are lost.",
    );
    expect(withdrawn("lead", 3)?.toggle).toBeUndefined();
  });

  it("names Foreman when it raised the request itself", () => {
    const n = noticeFor(request({ kind: "blocked", instanceId: "" }), [team], instances);
    expect(n?.title).toBe("Foreman needs you on Ledger");
  });

  it("tells when an instance is done, and only then", () => {
    const done = noticeFor(
      { type: "instance.status", data: { id: "tove@ledger", status: "done" } },
      [team],
      instances,
    );
    expect(done).toMatchObject({
      toggle: "finishes",
      title: "Tove is done on Ledger",
      deployment: "atlas@ledger",
    });
    for (const status of ["running", "idle", "awaiting", "stopped"]) {
      expect(
        noticeFor(
          { type: "instance.status", data: { id: "tove@ledger", status } },
          [team],
          instances,
        ),
      ).toBeUndefined();
    }
  });

  it("warns, as a toast only, when an agent waits for a session slot", () => {
    const queued = noticeFor(
      { type: "instance.status", data: { id: "tove@ledger", status: "queued" } },
      [team],
      instances,
    );
    expect(queued).toMatchObject({
      title: "Tove is waiting for a session slot",
      instance: "tove@ledger",
    });
    expect(queued?.toggle).toBeUndefined();
    vi.stubGlobal("Notification", { permission: "granted" });
    expect(
      notifiesBrowser(
        queued as Notice,
        { needsMe: true, finishes: true, mergeRequested: true },
        true,
      ),
    ).toBe(false);
    vi.unstubAllGlobals();
  });

  it("tells, as a toast only, when Foreman stopped a deployment on its own", () => {
    const stopped = (stopReason: string) =>
      noticeFor(
        {
          type: "deployment.stopped",
          data: { id: "atlas@ledger", teamId: "atlas", projectId: "ledger", stopReason },
        },
        [{ ...team, config: { ...team.config, name: "Atlas" } } as Team],
        instances,
      );
    expect(stopped("ledger is gone: all its code repos are missing")).toMatchObject({
      title: "Foreman stopped Atlas on Ledger",
      body: "ledger is gone: all its code repos are missing",
      deployment: "atlas@ledger",
      instance: "",
    });
    expect(stopped("ledger is gone: x")?.toggle).toBeUndefined();
    // The CEO stopped it: nothing to tell.
    expect(stopped("")).toBeUndefined();
  });

  it("ignores everything else", () => {
    for (const type of ["request.resolved", "team.message", "wiki.changed", "config.changed"]) {
      expect(noticeFor({ type, data: {} }, [team], instances)).toBeUndefined();
    }
  });
});

describe("browser notifications", () => {
  afterEach(() => vi.unstubAllGlobals());
  const notice = { toggle: "finishes" } as Notice;
  const toggles = { needsMe: true, finishes: true, mergeRequested: true };

  it("reads the permission, n/a without the API", () => {
    vi.stubGlobal("Notification", undefined);
    expect(permission()).toBe("n/a");
    for (const [browser, shown] of [
      ["granted", "on"],
      ["denied", "blocked"],
      ["default", "turn on"],
    ]) {
      vi.stubGlobal("Notification", { permission: browser });
      expect(permission()).toBe(shown);
    }
  });

  it("go out only with permission, the toggle on, and the tab hidden", () => {
    vi.stubGlobal("Notification", { permission: "granted" });
    expect(notifiesBrowser(notice, toggles, true)).toBe(true);
    expect(notifiesBrowser(notice, toggles, false)).toBe(false);
    expect(notifiesBrowser(notice, { ...toggles, finishes: false }, true)).toBe(false);
    vi.stubGlobal("Notification", { permission: "default" });
    expect(notifiesBrowser(notice, toggles, true)).toBe(false);
  });
});
