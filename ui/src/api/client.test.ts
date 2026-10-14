import { afterEach, describe, expect, it, vi } from "vitest";
import { httpClient } from "./client";

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
});
