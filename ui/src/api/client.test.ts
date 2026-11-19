import { afterEach, describe, expect, it, vi } from "vitest";
import { ApiError, httpClient } from "./client";

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
    const dep = await httpClient("http://x").agents.deploy("atlas", "ledger");
    expect(dep.id).toBe("atlas@ledger");
    expect(calls[0]).toMatchObject({ url: "http://x/api/deployments", method: "POST" });
    expect(JSON.parse(calls[0].body ?? "")).toEqual({
      team: "atlas",
      project: "ledger",
      fresh: false,
    });
    expect(calls[0].headers).toEqual({ "Content-Type": "application/json" });
  });

  it("asks which agents would continue, escaping the query", async () => {
    const calls = stubFetch(200, { agents: ["mara"] });
    expect(await httpClient("http://x").agents.continuing("a&b", "ledger")).toEqual(["mara"]);
    expect(calls[0].url).toBe("http://x/api/deployments/continuing?team=a%26b&project=ledger");
  });

  it("stops with DELETE, escaping the id", async () => {
    const calls = stubFetch(204, undefined);
    await httpClient("http://x").agents.stop("atlas@ledger");
    expect(calls[0]).toMatchObject({
      url: "http://x/api/deployments/atlas%40ledger",
      method: "DELETE",
    });
  });

  it("saves the instructions against the version they were read at", async () => {
    const saved = { text: "Be brief.", version: "v2" };
    const calls = stubFetch(200, saved);
    expect(await httpClient("http://x").home.saveInstructions("Be brief.", "v1")).toEqual(saved);
    expect(calls[0]).toMatchObject({ url: "http://x/api/instructions", method: "PUT" });
    expect(JSON.parse(calls[0].body ?? "")).toEqual({ text: "Be brief.", version: "v1" });
  });

  it("connects a profile and returns the login terminal", async () => {
    const calls = stubFetch(200, { terminal: "connect:work" });
    expect(await httpClient("http://x").profiles.connect("work")).toBe("connect:work");
    expect(calls[0]).toMatchObject({ url: "http://x/api/profiles/work/connect", method: "POST" });
  });

  it("creates a profile with its harness", async () => {
    const calls = stubFetch(201, { name: "work", harness: "claude-code", connected: false });
    const p = await httpClient("http://x").profiles.create("work", "claude-code");
    expect(p.name).toBe("work");
    expect(calls[0]).toMatchObject({ url: "http://x/api/profiles", method: "POST" });
    expect(JSON.parse(calls[0].body ?? "")).toEqual({ name: "work", harness: "claude-code" });
  });

  it("switches a profile's harness", async () => {
    const calls = stubFetch(200, { name: "default", harness: "codex", connected: false });
    const p = await httpClient("http://x").profiles.setHarness("default", "codex");
    expect(p.harness).toBe("codex");
    expect(calls[0]).toMatchObject({ url: "http://x/api/profiles/default/harness", method: "PUT" });
    expect(JSON.parse(calls[0].body ?? "")).toEqual({ harness: "codex" });
  });

  it("deletes a profile, escaping its name", async () => {
    const calls = stubFetch(204, undefined);
    await httpClient("http://x").profiles.remove("a/b");
    expect(calls[0]).toMatchObject({ url: "http://x/api/profiles/a%2Fb", method: "DELETE" });
  });

  it("surfaces the server's error message", async () => {
    stubFetch(409, {
      error: "This team is already deployed on this project",
      code: "ALREADY_DEPLOYED",
    });
    await expect(httpClient("http://x").agents.deploy("a", "b")).rejects.toThrow(
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

  it("saves a Pi profile's endpoint with PUT", async () => {
    const calls = stubFetch(204, undefined);
    const change = {
      url: "http://localhost:11434/v1",
      apiKey: "",
      model: "qwen3:30b",
      contextWindow: 262144,
      maxTokens: 131072,
    };
    await httpClient("http://x").profiles.setEndpoint("local", change);
    expect(calls[0]).toMatchObject({ url: "http://x/api/profiles/local/endpoint", method: "PUT" });
    expect(JSON.parse(calls[0].body ?? "")).toEqual(change);
  });

  it("asks a server's models with the key in the body, never the URL", async () => {
    const calls = stubFetch(200, ["qwen3:30b"]);
    expect(await httpClient("http://x").profiles.models("local", "http://gpu/v1", "sk-1")).toEqual([
      "qwen3:30b",
    ]);
    expect(calls[0]).toMatchObject({
      url: "http://x/api/profiles/local/endpoint/models",
      method: "POST",
    });
    expect(JSON.parse(calls[0].body ?? "")).toEqual({ url: "http://gpu/v1", apiKey: "sk-1" });
  });

  it("ends a login with DELETE", async () => {
    const calls = stubFetch(204, undefined);
    await httpClient("http://x").profiles.endLogin("work");
    expect(calls[0]).toMatchObject({
      url: "http://x/api/profiles/work/connect",
      method: "DELETE",
    });
  });

  it("asks whether it can open folders, and names a folder to open, never a path", async () => {
    const calls = stubFetch(200, { canOpen: true });
    expect(await httpClient("http://x").desktop.canOpen()).toEqual({ canOpen: true });
    expect(calls[0]).toMatchObject({ url: "http://x/api/desktop", method: "GET" });
    const opened = stubFetch(204, undefined);
    await httpClient("http://x").desktop.open({ instance: "tove@ledger" });
    await httpClient("http://x").desktop.open({ project: "ledger", folder: "inbox" });
    expect(opened.map((c) => [c.url, c.method, JSON.parse(c.body ?? "")])).toEqual([
      ["http://x/api/desktop/open", "POST", { instance: "tove@ledger" }],
      ["http://x/api/desktop/open", "POST", { project: "ledger", folder: "inbox" }],
    ]);
  });

  it("restarts an instance with POST", async () => {
    const calls = stubFetch(204, undefined);
    await httpClient("http://x").agents.restart("mara@ledger");
    expect(calls[0]).toMatchObject({
      url: "http://x/api/instances/mara%40ledger/restart",
      method: "POST",
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

  it("releases held messages now", async () => {
    const calls = stubFetch(204, undefined);
    await httpClient("http://x").agents.sendNow("mara@ledger");
    expect(calls[0]).toMatchObject({
      method: "POST",
      url: "http://x/api/instances/mara%40ledger/send-now",
    });
  });

  it("reads a project's repos at a ref", async () => {
    const calls = stubFetch(200, []);
    const client = httpClient("http://x");
    await client.repos.list("ledger");
    await client.repos.tree("ledger", "api", "wt/tove/x", "src/lib");
    await client.repos.file("ledger", "api", "main", "a b.md");
    await client.repos.log("ledger", "project", "main", 50);
    await client.repos.log("ledger", "api", "main", 0, true);
    expect(calls.map((c) => c.url)).toEqual([
      "http://x/api/projects/ledger/repos",
      "http://x/api/projects/ledger/repos/api/tree?ref=wt%2Ftove%2Fx&path=src%2Flib",
      "http://x/api/projects/ledger/repos/api/file?ref=main&path=a+b.md",
      "http://x/api/projects/ledger/repos/project/log?ref=main&skip=50",
      "http://x/api/projects/ledger/repos/api/log?ref=main&skip=0&all=1",
    ]);
  });

  it("pages a deployment's activity, and lists stopped deployments", async () => {
    const calls = stubFetch(200, { items: [], next: "" });
    const client = httpClient("http://x");
    await client.messages.activity("atlas@ledger");
    await client.messages.activity("atlas@ledger", "c2");
    await client.agents.stoppedDeployments();
    expect(calls.map((c) => c.url)).toEqual([
      "http://x/api/activity?deployment=atlas%40ledger",
      "http://x/api/activity?deployment=atlas%40ledger&before=c2",
      "http://x/api/deployments?status=stopped",
    ]);
  });

  it("sets a project's profile and name, scaffolds its HQ and hides it", async () => {
    const calls = stubFetch(200, { id: "cal" });
    const client = httpClient("http://x");
    await client.home.setProfile("my project", "work");
    await client.home.scaffold("my project");
    await client.home.setHidden("my project", true);
    await client.home.setName("my project", "Ledger · EUR");
    await client.home.setName("my project", "");
    expect(calls.map((c) => [c.method, c.url, c.body && JSON.parse(c.body)])).toEqual([
      ["PUT", "http://x/api/projects/my%20project/profile", { profile: "work" }],
      ["POST", "http://x/api/projects/my%20project/scaffold", undefined],
      ["PUT", "http://x/api/projects/my%20project/hidden", { hidden: true }],
      ["PUT", "http://x/api/projects/my%20project/name", { name: "Ledger · EUR" }],
      ["PUT", "http://x/api/projects/my%20project/name", { name: "" }],
    ]);
  });

  it("creates a project, new or cloned, and removes one with what was asked", async () => {
    const calls = stubFetch(200, { id: "cal" });
    const client = httpClient("http://x");
    await client.home.newProject("cal", "Cal");
    await client.home.newProject("ops", "Ops HQ", "git@github.com:me/ops-hq.git");
    await client.home.removeProject("a", { deleteHQ: false, deleteWorktrees: false });
    await client.home.removeProject("b", { deleteHQ: true, deleteWorktrees: true });
    await client.home.folders();
    await client.home.folders("/home/ceo/My Repos", "cal");
    await client.home.folders(undefined, "cal");
    expect(calls.map((c) => [c.method, c.url, c.body && JSON.parse(c.body)])).toEqual([
      ["POST", "http://x/api/projects", { id: "cal", name: "Cal" }],
      [
        "POST",
        "http://x/api/projects",
        { id: "ops", name: "Ops HQ", clone: "git@github.com:me/ops-hq.git" },
      ],
      ["DELETE", "http://x/api/projects/a", undefined],
      ["DELETE", "http://x/api/projects/b?deleteHQ=true&deleteWorktrees=true", undefined],
      ["GET", "http://x/api/folders", undefined],
      ["GET", "http://x/api/folders?path=%2Fhome%2Fceo%2FMy+Repos&project=cal", undefined],
      ["GET", "http://x/api/folders?project=cal", undefined],
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
    expect(c.repos.imageUrl("ledger", "api", "wt/tove/x", "img/a b.png")).toBe(
      "http://x/api/projects/ledger/repos/api/image?ref=wt%2Ftove%2Fx&path=img%2Fa+b.png",
    );
  });

  it("lists the open requests", async () => {
    const calls = stubFetch(200, []);
    await httpClient("http://x").requests.list();
    expect(calls[0].url).toBe("http://x/api/requests");
  });

  it("sets a repo's default branch and adds a repo with one", async () => {
    const calls = stubFetch(200, { id: "ledger" });
    const client = httpClient("http://x");
    await client.home.setRepoDefault("ledger", "my api", "trunk");
    await client.home.addRepo("ledger", "web", "/h/web", "master");
    expect(calls.map((c) => [c.method, c.url, c.body && JSON.parse(c.body)])).toEqual([
      ["PUT", "http://x/api/projects/ledger/repos/my%20api/default", { branch: "trunk" }],
      [
        "POST",
        "http://x/api/projects/ledger/repos",
        { name: "web", path: "/h/web", defaultBranch: "master" },
      ],
    ]);
  });

  it("comments on and decides a review", async () => {
    const calls = stubFetch(204, undefined);
    const c = httpClient("http://x");
    const general = {
      repo: "",
      path: "",
      side: "" as const,
      line: 0,
      commit: "",
      snippet: "",
      body: "Why?",
    };
    await c.reviews.addComment("rev-1", general);
    await c.reviews.editComment("rev-1", "com-2", "Why not?");
    await c.reviews.deleteComment("rev-1", "com-2");
    await c.reviews.requestChanges("rev-1", "see the comments");
    await c.reviews.merge("rev-1", { api: "a1b2" });
    await c.reviews.merge("rev-1", { api: "a1b2" }, "f/csv");
    expect(calls.map((c) => [c.method, c.url, c.body && JSON.parse(c.body)])).toEqual([
      ["POST", "http://x/api/reviews/rev-1/comments", general],
      ["PUT", "http://x/api/reviews/rev-1/comments/com-2", { body: "Why not?" }],
      ["DELETE", "http://x/api/reviews/rev-1/comments/com-2", undefined],
      ["POST", "http://x/api/reviews/rev-1/changes", { note: "see the comments" }],
      ["POST", "http://x/api/reviews/rev-1/merge", { heads: { api: "a1b2" } }],
      ["POST", "http://x/api/reviews/rev-1/merge", { heads: { api: "a1b2" }, landing: "f/csv" }],
    ]);
  });

  it("dismisses a review, answering with the branches kept", async () => {
    const kept = [{ repo: "api", branch: "lead/atlas/x", why: "changes in the way" }];
    const calls = stubFetch(200, { kept });
    const c = httpClient("http://x");
    const dismiss = { note: "", dropBranches: true, stopTeam: true };
    expect(await c.reviews.dismiss("rev-1", dismiss)).toEqual({ kept });
    expect(calls.map((c) => [c.method, c.url, c.body && JSON.parse(c.body)])).toEqual([
      ["POST", "http://x/api/reviews/rev-1/dismiss", dismiss],
    ]);
  });

  it("reads the reviews and a review's change", async () => {
    const calls = stubFetch(200, []);
    const c = httpClient("http://x");
    await c.reviews.list("ledger");
    await c.reviews.get("rev-1");
    await c.reviews.diff("rev-1", false);
    await c.reviews.diff("rev-1", true);
    expect(calls.map((c) => c.url)).toEqual([
      "http://x/api/reviews?project=ledger",
      "http://x/api/reviews/rev-1",
      "http://x/api/reviews/rev-1/diff",
      "http://x/api/reviews/rev-1/diff?since=1",
    ]);
  });

  it("reads a project's pipeline", async () => {
    const calls = stubFetch(200, { workItems: [], mergeRequests: [] });
    await httpClient("http://x").pipeline.list("my project");
    expect(calls[0].url).toBe("http://x/api/pipeline?project=my%20project");
  });

  it("cleans up a project, saying whether uncommitted work goes too", async () => {
    const calls = stubFetch(200, { removed: [], kept: [] });
    const c = httpClient("http://x");
    await c.pipeline.cleanUp("my project", false);
    await c.pipeline.cleanUp("my project", true);
    expect(calls.map((c) => [c.method, c.url, JSON.parse(c.body ?? "")])).toEqual([
      ["POST", "http://x/api/projects/my%20project/cleanup", { uncommitted: false }],
      ["POST", "http://x/api/projects/my%20project/cleanup", { uncommitted: true }],
    ]);
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

  // A header the library can't edit safely is refused with a 422 whose text says
  // why: that text is the error the board's toast shows, not a status code.
  it("carries a refused wiki write's text", async () => {
    const refusal = "frontmatter of backlog/x.md can't be edited safely: line 2: flow mapping";
    stubFetch(422, { error: refusal });
    const c = httpClient("http://x");
    for (const write of [
      () => c.wiki.moveCard("ledger", "sprint 7", "backlog/x.md", "done", "", "v2"),
      () => c.wiki.setCheckbox("ledger", "backlog/x.md", 12, true, "v1"),
    ]) {
      const e = await write().catch((e: unknown) => e);
      expect(e).toBeInstanceOf(ApiError);
      expect((e as Error).message).toBe(refusal);
    }
  });

  it("answers a request with the CEO's text", async () => {
    const calls = stubFetch(200, {});
    await httpClient("http://x").requests.resolve("req/1", "Approve, but pin 2.x");
    expect(calls[0]).toMatchObject({
      url: "http://x/api/requests/req%2F1/resolve",
      method: "POST",
      body: JSON.stringify({ answer: "Approve, but pin 2.x" }),
    });
  });

  it("dismisses a request, with nothing to send", async () => {
    const calls = stubFetch(200, {});
    await httpClient("http://x").requests.dismiss("req/1");
    expect(calls[0]).toMatchObject({
      url: "http://x/api/requests/req%2F1/dismiss",
      method: "POST",
    });
  });
});
