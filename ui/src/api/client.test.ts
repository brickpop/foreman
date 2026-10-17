import { afterEach, describe, expect, it, vi } from "vitest";
import { ApiError, httpClient, isNotBuilt } from "./client";

type Call = { url: string; method: string; body?: string; headers?: Record<string, string> };

function stubFetch(status: number, body: unknown): Call[] {
  const calls: Call[] = [];
  vi.stubGlobal("fetch", async (url: string, init: RequestInit) => {
    calls.push({
      url,
      method: init.method ?? "GET",
      body: init.body as string | undefined,
      headers: init.headers as Record<string, string> | undefined,
    });
    return new Response(body === undefined ? null : JSON.stringify(body), { status });
  });
  return calls;
}

afterEach(() => vi.unstubAllGlobals());

describe("httpClient", () => {
  it("deploys with a JSON body", async () => {
    const calls = stubFetch(201, { id: "atlas@ledger" });
    const dep = await httpClient("http://x").agents.deploy("atlas", "ledger", "Ship it");
    expect(dep.id).toBe("atlas@ledger");
    expect(calls[0]).toMatchObject({ url: "http://x/api/deployments", method: "POST" });
    expect(JSON.parse(calls[0].body ?? "")).toEqual({
      team: "atlas",
      project: "ledger",
      goal: "Ship it",
    });
    expect(calls[0].headers).toEqual({ "Content-Type": "application/json" });
  });

  it("stops with DELETE, escaping the id", async () => {
    const calls = stubFetch(204, undefined);
    await httpClient("http://x").agents.stop("atlas@ledger");
    expect(calls[0]).toMatchObject({
      url: "http://x/api/deployments/atlas%40ledger",
      method: "DELETE",
    });
  });

  it("connects and returns the login terminal", async () => {
    stubFetch(200, { terminal: "connect:claude-code" });
    expect(await httpClient("http://x").harnesses.connect("claude-code")).toBe(
      "connect:claude-code",
    );
  });

  it("surfaces the server's error message", async () => {
    stubFetch(409, {
      error: "This team is already deployed on this project",
      code: "ALREADY_DEPLOYED",
    });
    await expect(httpClient("http://x").agents.deploy("a", "b", "c")).rejects.toThrow(
      "already deployed",
    );
  });

  it("falls back to the status when the error isn't JSON", async () => {
    vi.stubGlobal("fetch", async () => new Response("oops", { status: 500 }));
    await expect(httpClient("http://x").agents.instances()).rejects.toThrow(
      "/api/instances: HTTP 500",
    );
  });

  it("lists a deployment's messages", async () => {
    const calls = stubFetch(200, []);
    await httpClient("http://x").messages.list("atlas@ledger");
    expect(calls[0]).toMatchObject({
      url: "http://x/api/messages?deployment=atlas%40ledger",
      method: "GET",
    });
  });

  it("ends a login with DELETE", async () => {
    const calls = stubFetch(204, undefined);
    await httpClient("http://x").harnesses.endLogin("claude-code");
    expect(calls[0]).toMatchObject({
      url: "http://x/api/harnesses/claude-code/connect",
      method: "DELETE",
    });
  });

  it("wakes an instance with POST", async () => {
    const calls = stubFetch(204, undefined);
    await httpClient("http://x").agents.wake("tove@ledger");
    expect(calls[0]).toMatchObject({
      url: "http://x/api/instances/tove%40ledger/wake",
      method: "POST",
    });
  });

  it("sends the CEO's line and pauses the countdown", async () => {
    const calls = stubFetch(204, undefined);
    const client = httpClient("http://x");
    await client.agents.line("mara@ledger", "two\nlines");
    await client.agents.pauseCountdown("mara@ledger", true);
    expect(calls.map((c) => [c.method, c.url, JSON.parse(c.body ?? "")])).toEqual([
      ["POST", "http://x/api/instances/mara%40ledger/line", { text: "two\nlines" }],
      ["PUT", "http://x/api/instances/mara%40ledger/countdown", { paused: true }],
    ]);
  });

  it("adds or creates a project with an optional first repo, and a repo to a project", async () => {
    const calls = stubFetch(200, { id: "cal" });
    const client = httpClient("http://x");
    await client.workspace.addProject("cal", "cal-go");
    await client.workspace.addProject("ledger");
    await client.workspace.newProject("notes", "notes");
    await client.workspace.addRepo("my project", "web");
    await client.workspace.scaffold("my project");
    expect(calls.map((c) => [c.method, c.url, c.body && JSON.parse(c.body)])).toEqual([
      ["POST", "http://x/api/projects", { folder: "cal", repo: "cal-go" }],
      ["POST", "http://x/api/projects", { folder: "ledger" }],
      ["POST", "http://x/api/projects", { folder: "notes", repo: "notes", new: true }],
      ["POST", "http://x/api/projects/my%20project/repos", { name: "web" }],
      ["POST", "http://x/api/projects/my%20project/scaffold", undefined],
    ]);
  });

  it("reads the wiki, escaping each segment of a path but not its slashes", async () => {
    const calls = stubFetch(200, []);
    const c = httpClient("http://x");
    await c.wiki.tree("my project");
    await c.wiki.entry("ledger", "backlog/a b#1.md");
    expect(calls.map((c) => c.url)).toEqual([
      "http://x/api/wiki/my%20project/tree",
      "http://x/api/wiki/ledger/entry/backlog/a%20b%231.md",
    ]);
    expect(c.wiki.imageUrl("ledger", "img/d?.png")).toBe(
      "http://x/api/wiki/ledger/image/img/d%3F.png",
    );
  });

  it("decides on merge requests", async () => {
    const calls = stubFetch(204, undefined);
    const c = httpClient("http://x");
    await c.pipeline.merge("mr-1");
    await c.pipeline.reject("mr-1", "split it");
    expect(calls.map((c) => [c.method, c.url, c.body && JSON.parse(c.body)])).toEqual([
      ["POST", "http://x/api/merge-requests/mr-1/merge", undefined],
      ["POST", "http://x/api/merge-requests/mr-1/reject", { reason: "split it" }],
    ]);
  });

  it("reads a project's pipeline", async () => {
    const calls = stubFetch(200, { workItems: [], mergeRequests: [] });
    await httpClient("http://x").pipeline.list("my project");
    expect(calls[0].url).toBe("http://x/api/pipeline?project=my%20project");
  });

  it("writes the wiki with the version it read", async () => {
    const calls = stubFetch(204, undefined);
    const c = httpClient("http://x");
    await c.wiki.setCheckbox("ledger", "backlog/a b.md", 12, true, "v1");
    await c.wiki.moveCard("ledger", "sprint 7", "backlog/x.md", "done", "", "v2");
    expect(calls.map((c) => [c.method, c.url, JSON.parse(c.body ?? "")])).toEqual([
      [
        "PUT",
        "http://x/api/wiki/ledger/checkbox/backlog/a%20b.md",
        { line: 12, done: true, version: "v1" },
      ],
      [
        "PUT",
        "http://x/api/wiki/ledger/card/sprint%207/backlog/x.md",
        { value: "done", lane: "", version: "v2" },
      ],
    ]);
  });

  it("refuses the areas the server doesn't serve yet, without asking it", async () => {
    const calls = stubFetch(200, []);
    const c = httpClient("http://x");
    for (const call of [
      () => c.requests.list(),
      () => c.pipeline.createPr("mr-1"),
      () => c.chiefOfStaff.send("hi"),
    ]) {
      const e = await call().catch((e: unknown) => e);
      expect(isNotBuilt(e)).toBe(true);
    }
    expect(calls).toEqual([]);
  });

  it("tells a refusal that isn't NOT_BUILT apart", () => {
    expect(isNotBuilt(new ApiError("taken", "CONFLICT"))).toBe(false);
    expect(isNotBuilt(new Error("NOT_BUILT"))).toBe(false);
    expect(isNotBuilt(undefined)).toBe(false);
  });
});
