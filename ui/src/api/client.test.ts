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
    expect(calls.map((c) => [c.method, c.url, JSON.parse(c.body ?? "")])).toEqual([
      ["POST", "http://x/api/projects", { folder: "cal", repo: "cal-go" }],
      ["POST", "http://x/api/projects", { folder: "ledger" }],
      ["POST", "http://x/api/projects", { folder: "notes", repo: "notes", new: true }],
      ["POST", "http://x/api/projects/my%20project/repos", { name: "web" }],
    ]);
  });

  it("refuses the areas the server doesn't serve yet, without asking it", async () => {
    const calls = stubFetch(200, []);
    const c = httpClient("http://x");
    for (const call of [
      () => c.requests.list(),
      () => c.pipeline.merge("mr-1"),
      () => c.wiki.tree("ledger"),
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
