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
      title: "Tove needs you on ledger",
      body: "Which budget?",
      instance: "tove@ledger",
      deployment: "atlas@ledger",
    });
  });

  it("calls a review a merge request, gated by its own toggle", () => {
    const n = noticeFor(
      request({ kind: "review", instanceId: "mara@ledger", summary: "Settlement v2" }),
      [team],
      instances,
    );
    expect(n).toMatchObject({
      toggle: "mergeRequested",
      title: "Mara requested a merge on ledger",
    });
  });

  it("names Foreman when it raised the request itself", () => {
    const n = noticeFor(request({ kind: "blocked", instanceId: "" }), [team], instances);
    expect(n?.title).toBe("Foreman needs you on ledger");
  });

  it("tells when an instance is done, and only then", () => {
    const done = noticeFor(
      { type: "instance.status", data: { id: "tove@ledger", status: "done" } },
      [team],
      instances,
    );
    expect(done).toMatchObject({
      toggle: "finishes",
      title: "Tove is done on ledger",
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

describe("noticeFor the Chief of Staff", () => {
  it("names it, and leaves out the project of a team change", () => {
    const team = noticeFor(
      request({ kind: "review", source: "cos", instanceId: "", projectId: "" }),
      [],
      [],
    );
    expect(team?.title).toBe("Chief of Staff requested a merge");
    const config = noticeFor(request({ kind: "review", source: "cos", instanceId: "" }), [], []);
    expect(config?.title).toBe("Chief of Staff requested a merge on ledger");
  });
});
