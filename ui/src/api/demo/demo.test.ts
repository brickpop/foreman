import { afterEach, describe, expect, it, vi } from "vitest";
import type { ForemanClient } from "../client";
import type { Event } from "../types/events";
import { demoClient } from "./demo";
import type { Seed } from "./seed";

const zero = "0001-01-01T00:00:00Z";

// A tiny seed: one project, one team of two, one deployment.
const tiny: Seed = {
  workspace: { path: "~/agents-ws" },
  projects: [
    {
      id: "ledger",
      config: {
        project: "ledger",
        description: "",
        repos: [],
        main: "main",
        checks: "",
        mergeToMain: "ceo",
        alwaysAskCeo: [],
        forge: { kind: "none", url: "", projectRepo: "" },
      },
      repos: ["ledger-api"],
      version: "v1",
      problems: [],
      source: `project = "ledger"\n`,
    },
  ],
  candidates: [{ folder: "billing", repos: ["billing-api"] }],
  teams: [
    {
      id: "atlas",
      version: "v1",
      problems: [],
      source: "",
      config: {
        team: "atlas",
        name: "Atlas",
        purpose: "",
        lead: "mara",
        playbook: [],
        escalate: [],
        agents: {
          mara: soul("Mara", "team_lead"),
          tove: soul("Tove", "engineer"),
        },
      },
    },
  ],
  settings: {
    settings: {
      listen: "local",
      port: 8888,
      maxSessions: 12,
      engine: "podman",
      harnesses: {},
      stuckAfterMinutes: 20,
      teamMessageMarkers: true,
      logCapMb: 50,
      forges: {},
      workspaceForge: { kind: "none", url: "" },
      notifications: { needsMe: true, finishes: true, mergeRequested: true },
    },
    version: "",
    problems: [],
  },
  harnesses: [{ id: "claude-code", connected: true }],
  deployments: [],
  instances: [],
  requests: [
    {
      id: "r1",
      kind: "approval",
      projectId: "ledger",
      deploymentId: "atlas@ledger",
      instanceId: "mara@ledger",
      summary: "Add a dependency?",
      options: ["Approve", "Deny"],
      source: "agent",
      status: "open",
      answer: "",
      createdAt: zero,
      resolvedAt: zero,
      mergeRequestId: "",
      links: [],
    },
  ],
  messages: [],
  workItems: [],
  mergeRequests: [
    {
      id: "mr1",
      projectId: "ledger",
      deploymentId: "atlas@ledger",
      authorInstance: "mara@ledger",
      repo: "ledger-api",
      branch: "lead/atlas/integration",
      entry: "backlog/LED-44.md",
      state: "open",
      checksStatus: "pass",
      conflicting: false,
      forgePrUrl: "",
      forgePrNumber: 0,
      note: "",
      createdAt: zero,
      mergedAt: zero,
      rejectedAt: zero,
    },
  ],
  wiki: {
    ledger: {
      tree: [
        {
          path: "index.md",
          title: "Ledger",
          folder: false,
          hasIndex: false,
          changed: true,
          children: [],
        },
      ],
      entries: {
        "index.md": {
          path: "index.md",
          title: "Ledger",
          version: "e1",
          frontmatter: {},
          html: "<h1>Ledger</h1>",
          headings: [],
          checkboxes: [{ line: 3, done: false, text: "ship" }],
          backlinks: [],
        },
      },
      boards: [
        {
          id: "sprint",
          name: "Sprint",
          columns: ["todo", "done"],
          laneField: "priority",
          lanes: ["high", "low"],
          cards: [
            {
              path: "backlog/LED-44.md",
              id: "LED-44",
              title: "Idempotency keys",
              column: "todo",
              lane: "high",
              priority: "high",
              assignee: "tove",
              version: "c1",
            },
          ],
          declaration: "",
          problems: [],
        },
      ],
    },
  },
};

function soul(name: string, role: string) {
  return {
    name,
    role,
    harness: "claude-code",
    command: "",
    personality: "",
    skills: [],
    instruction: "",
    look: { color: "", skin: "", hair: "", hairColor: "", eyes: "", beard: "" },
  };
}

function client(): { c: ForemanClient; events: Event[] } {
  const c = demoClient(tiny, { tick: 0, now: () => new Date("2026-10-14T09:00:00Z") });
  const events: Event[] = [];
  c.events.subscribe({ event: (e) => events.push(e), reconnected: () => {} });
  return { c, events };
}

afterEach(() => vi.useRealTimers());

describe("demoClient", () => {
  it("marks every area as demo data", () => {
    const { c } = client();
    for (const area of Object.values(c)) expect(area.demo).toBe(true);
  });

  it("deploys: a deployment, its instances starting then running, and events", async () => {
    vi.useFakeTimers();
    const { c, events } = client();
    const dep = await c.agents.deploy("atlas", "ledger", "Ship it");
    expect(dep).toMatchObject({ id: "atlas@ledger", status: "active", goal: "Ship it" });
    expect((await c.agents.instances()).map((i) => [i.id, i.status])).toEqual([
      ["mara@ledger", "starting"],
      ["tove@ledger", "starting"],
    ]);
    vi.advanceTimersByTime(1500);
    expect((await c.agents.instances()).map((i) => i.status)).toEqual(["running", "idle"]);
    expect(events.map((e) => e.type)).toContain("deployment.created");
    await expect(c.agents.deploy("atlas", "ledger", "again")).rejects.toThrow("already deployed");
    await c.agents.stop("atlas@ledger");
    expect(await c.agents.instances()).toEqual([]);
    expect(events.at(-1)?.type).toBe("deployment.stopped");
  });

  it("resolves requests", async () => {
    const { c, events } = client();
    await c.requests.resolve("r1", "Approve");
    expect(await c.requests.list()).toEqual([]);
    expect(events).toEqual([expect.objectContaining({ type: "request.resolved" })]);
  });

  it("merges, opens PRs and rejects merge requests", async () => {
    const { c } = client();
    await c.pipeline.createPr("mr1");
    let [mr] = (await c.pipeline.list("ledger")).mergeRequests;
    expect(mr.forgePrNumber).toBeGreaterThan(0);
    await c.pipeline.reject("mr1", "Needs tests");
    [mr] = (await c.pipeline.list("ledger")).mergeRequests;
    expect(mr).toMatchObject({ state: "rejected", note: "Needs tests" });
  });

  it("guards wiki writes by version, like the server", async () => {
    const { c } = client();
    await expect(c.wiki.setCheckbox("ledger", "index.md", 3, true, "stale")).rejects.toThrow(
      "changed since",
    );
    await c.wiki.setCheckbox("ledger", "index.md", 3, true, "e1");
    const entry = await c.wiki.entry("ledger", "index.md");
    expect(entry.checkboxes[0].done).toBe(true);
    expect(entry.version).not.toBe("e1");

    await c.wiki.moveCard("ledger", "sprint", "backlog/LED-44.md", "done", "low", "c1");
    const card = (await c.wiki.boards("ledger"))[0].cards[0];
    expect(card).toMatchObject({ column: "done", lane: "low" });
    await expect(
      c.wiki.moveCard("ledger", "sprint", "backlog/LED-44.md", "todo", "high", "c1"),
    ).rejects.toThrow("changed since");
  });

  it("adds a project from a candidate, with a starter wiki", async () => {
    const { c, events } = client();
    expect(await c.workspace.candidates()).toHaveLength(1);
    const p = await c.workspace.addProject("billing");
    expect(p).toMatchObject({ id: "billing", repos: ["billing-api"] });
    expect(p.source).toContain(`project = "billing"`);
    expect(p.source).toContain("# always_ask_ceo");
    expect(await c.workspace.candidates()).toEqual([]);
    expect((await c.wiki.entry("billing", "index.md")).html).toContain("Settings tab");
    expect((await c.wiki.boards("billing"))[0].cards).toEqual([]);
    expect(events.map((e) => e.type)).toContain("workspace.changed");
    await expect(c.workspace.addProject("nope")).rejects.toThrow("no such folder");
  });

  it("marks an entry seen when it is read", async () => {
    const { c } = client();
    expect((await c.wiki.tree("ledger"))[0].changed).toBe(true);
    await c.wiki.entry("ledger", "index.md");
    expect((await c.wiki.tree("ledger"))[0].changed).toBe(false);
  });

  it("hands out copies: a caller can't change the demo's memory", async () => {
    const { c } = client();
    const [p] = await c.workspace.projects();
    p.id = "changed";
    expect((await c.workspace.projects())[0].id).toBe("ledger");
  });

  it("simulates the team at work on its timer", async () => {
    vi.useFakeTimers();
    const c = demoClient(tiny, { tick: 1000 });
    await c.agents.deploy("atlas", "ledger", "g");
    const events: Event[] = [];
    const stop = c.events.subscribe({ event: (e) => events.push(e), reconnected: () => {} });
    vi.advanceTimersByTime(1500 + 4000);
    stop();
    const types = events.map((e) => e.type);
    expect(types).toContain("team.message");
    expect(types).toContain("request.created");
    const count = events.length;
    vi.advanceTimersByTime(5000);
    expect(events.length).toBe(count); // no subscriber, no simulation
  });
});

describe("demoClient's teams", () => {
  it("saves a soul: version-guarded, the file and the event follow", async () => {
    const { c, events } = client();
    const [t] = await c.workspace.teams();
    expect(t.source).toContain('[agents.mara]\nname = "Mara"');
    const soul = { ...t.config.agents.tove, personality: "Careful." };
    await expect(c.workspace.saveSoul("atlas", "tove", soul, "stale")).rejects.toThrow(
      "changed since",
    );
    const saved = await c.workspace.saveSoul("atlas", "tove", soul, t.version);
    expect(saved.config.agents.tove.personality).toBe("Careful.");
    expect(saved.source).toContain('personality = "Careful."');
    expect(saved.version).not.toBe(t.version);
    expect(events.at(-1)).toMatchObject({
      type: "config.changed",
      data: { object: "team", id: "atlas" },
    });
  });

  it("gives prompts: per instance, and a template with placeholders", async () => {
    vi.useFakeTimers();
    const { c } = client();
    await c.agents.deploy("atlas", "ledger", "Ship it");
    expect(await c.agents.prompt("tove@ledger")).toContain("Goal: Ship it");
    expect(await c.workspace.promptTemplate("atlas", "tove")).toContain("Goal: {goal}");
  });
});
