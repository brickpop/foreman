import { describe, expect, it } from "vitest";
import type { Instance, Project, Request, Team } from "../api/client";
import { keys, staleKeys } from "./queries";
import { sidebarModel } from "./Shell";

const project = (id: string) => ({ id }) as Project;
const team = (id: string, name: string) => ({ id, config: { name } }) as Team;
const instance = (deploymentId: string, status: string) => ({ deploymentId, status }) as Instance;
const request = (projectId: string, status = "open", kind = "question") =>
  ({ projectId, status, kind }) as Request;

describe("sidebarModel", () => {
  it("counts each project's Inbox: its open requests, reviews included", () => {
    const m = sidebarModel(
      [project("ledger"), project("relay"), project("harbor")],
      [],
      [
        request("ledger"),
        request("ledger", "open", "review"),
        request("relay", "resolved", "review"),
        request("elsewhere"),
      ],
    );
    expect(m.projects).toEqual([
      { id: "ledger", inbox: 2 },
      { id: "relay", inbox: 0 },
      { id: "harbor", inbox: 0 },
    ]);
  });

  it("puts the archived projects in their own group, still marked when they need the CEO", () => {
    const m = sidebarModel(
      [
        project("ledger"),
        { ...project("relay"), archived: true },
        { ...project("harbor"), archived: true },
      ],
      [],
      [request("harbor")],
    );
    expect(m.projects).toEqual([{ id: "ledger", inbox: 0 }]);
    expect(m.archived).toEqual([
      { id: "relay", inbox: 0 },
      { id: "harbor", inbox: 1 },
    ]);
  });

  it("marks a project's Session while its team is busy, not at rest", () => {
    const m = sidebarModel(
      [project("ledger"), project("relay"), project("harbor"), project("big-ledger")],
      [
        instance("atlas@ledger", "idle"),
        instance("atlas@ledger", "running"),
        instance("orion@relay", "idle"),
        instance("orion@relay", "asleep"),
        instance("vesper@big-ledger", "awaiting"),
      ],
      [],
    );
    expect(m.projects.map((p) => p.busy)).toEqual(["running", undefined, undefined, "awaiting"]);
  });
});

describe("staleKeys", () => {
  it("invalidates what each event touches", () => {
    expect(staleKeys({ type: "config.changed", data: { object: "team" } })).toEqual([
      keys.teams,
      ["prompt"],
      keys.setup,
      keys.instances,
    ]);
    expect(staleKeys({ type: "config.changed", data: { object: "settings" } })).toEqual([
      keys.settings,
      keys.setup,
      ["prompt"],
      ["workspace"],
      keys.instances,
    ]);
    expect(staleKeys({ type: "config.changed", data: { object: "project" } })).toEqual([
      keys.projects,
      keys.setup,
      keys.instances,
    ]);
    expect(staleKeys({ type: "projects.changed", data: {} })).toEqual([
      keys.projects,
      keys.setup,
      keys.repos,
      keys.instances,
    ]);
    expect(staleKeys({ type: "setup.changed", data: {} })).toEqual([keys.setup, keys.maintenance]);
    expect(staleKeys({ type: "instance.status", data: {} })).toEqual([keys.instances]);
    // A review request opening or closing is its review's status changing.
    expect(staleKeys({ type: "request.created", data: {} })).toEqual([
      keys.requests,
      ["pipeline", "reviews"],
    ]);
    expect(staleKeys({ type: "team.message", data: {} })).toEqual([keys.messages, keys.activity]);
    expect(staleKeys({ type: "team.action", data: {} })).toEqual([keys.activity, keys.repos]);
    expect(staleKeys({ type: "deployment.created", data: {} })).toEqual([
      keys.instances,
      keys.deployments,
    ]);
    expect(staleKeys({ type: "wiki.changed", data: { project: "ledger" } })).toEqual([
      ["wiki", "ledger"],
    ]);
    expect(staleKeys({ type: "pipeline.changed", data: {} })).toEqual([["pipeline"], keys.repos]);
    expect(staleKeys({ type: "something.else", data: {} })).toEqual([]);
  });
});

import { greeting, inbox, ordered, projectRows, teamCards, workingSummary } from "./model";

const req = (id: string, kind: string, createdAt: string, projectId = "ledger") =>
  ({ id, kind, createdAt, projectId, status: "open" }) as Request;
const inst = (id: string, status: string, deploymentId: string) =>
  ({ id, status, deploymentId }) as Instance;

describe("the Dashboard's model", () => {
  it("orders requests by kind, oldest first within one", () => {
    const list = ordered([
      req("q", "question", "2026-10-14T09:00:00Z"),
      req("f", "finding", "2026-10-14T08:00:00Z"),
      req("a2", "approval", "2026-10-14T09:30:00Z"),
      req("b", "blocked", "2026-10-14T09:40:00Z"),
      req("a1", "approval", "2026-10-14T09:10:00Z"),
      req("r", "review", "2026-10-14T07:00:00Z"),
    ]);
    expect(list.map((r) => r.id)).toEqual(["b", "a1", "a2", "r", "q", "f"]);
  });

  it("keeps reviews out of the requests answered in place", () => {
    const list = inbox([
      req("r", "review", "2026-10-14T07:00:00Z"),
      req("q", "question", "2026-10-14T09:00:00Z"),
      { ...req("old", "approval", "2026-10-14T06:00:00Z"), status: "resolved" },
      req("b", "blocked", "2026-10-14T09:40:00Z"),
    ]);
    expect(list.map((r) => r.id)).toEqual(["b", "q"]);
  });

  it("greets by the time of day", () => {
    const at = (h: number) => greeting(new Date(2026, 9, 21, h, 30));
    expect([at(3), at(5), at(11), at(12), at(17), at(18), at(23)]).toEqual([
      "Good night",
      "Good morning",
      "Good morning",
      "Good afternoon",
      "Good afternoon",
      "Good evening",
      "Good evening",
    ]);
  });

  it("titles and subtitles", () => {
    expect(
      workingSummary([
        inst("mara@ledger", "running", "atlas@ledger"),
        inst("tove@ledger", "idle", "atlas@ledger"),
        inst("lumen@relay", "running", "orion@relay"),
      ]),
    ).toBe("2 sessions working across 2 projects.");
    expect(workingSummary([inst("mara@ledger", "running", "atlas@ledger")])).toBe(
      "1 session working across 1 project.",
    );
  });

  it("gives each team its card: members lead first, the projects it's on", () => {
    const atlas = {
      id: "atlas",
      config: {
        name: "",
        purpose: "Builds the ledger",
        lead: "mara",
        order: ["tove"],
        agents: { mara: { name: "Mara" }, tove: { name: "Tove" } },
      },
    } as unknown as Team;
    const [card] = teamCards(
      [atlas],
      [
        inst("tove@relay", "running", "atlas@relay"),
        inst("mara@ledger", "idle", "atlas@ledger"),
        inst("tove@ledger", "idle", "atlas@ledger"),
        inst("lumen@ledger", "idle", "atlantis@ledger"),
      ],
    );
    expect(card.name).toBe("atlas");
    expect(card.purpose).toBe("Builds the ledger");
    expect(card.members.map((s) => s.name)).toEqual(["Mara", "Tove"]);
    expect(card.on).toEqual(["ledger", "relay"]);
  });

  it("gives each project not archived its line and state", () => {
    const rows = projectRows(
      [
        project("ledger"),
        project("relay"),
        project("harbor"),
        { ...project("old"), archived: true },
      ],
      [team("atlas", "Atlas"), team("orion", "Orion")],
      [
        inst("mara@ledger", "running", "atlas@ledger"),
        inst("tove@ledger", "running", "atlas@ledger"),
        inst("lumen@relay", "idle", "orion@relay"),
      ],
      [req("x", "question", "2026-10-14T09:00:00Z", "ledger")],
    );
    expect(rows).toEqual([
      { id: "ledger", line: "Atlas · 2 agents working", state: "needs you" },
      { id: "relay", line: "Orion · 0 agents working", state: "idle" },
      { id: "harbor", line: "No team deployed", state: "idle" },
    ]);
  });
});

import { agentOf, resolveWho, roleLabel, roster } from "./model";

describe("teams", () => {
  const atlas = {
    id: "atlas",
    config: {
      lead: "mara",
      agents: {
        mara: { name: "Mara", role: "team_lead" },
        tove: { name: "Tove", role: "engineer" },
        ilya: { name: "Ilya", role: "engineer" },
      },
    },
  } as unknown as Team;

  it("resolves who a step names", () => {
    expect(resolveWho(atlas.config, "lead")).toEqual({ text: "Mara", known: true });
    expect(resolveWho(atlas.config, "members")).toEqual({ text: "Members", known: true });
    expect(resolveWho(atlas.config, "ceo")).toEqual({ text: "You", known: true });
    expect(resolveWho(atlas.config, "tove")).toEqual({ text: "Tove", known: true });
    expect(resolveWho(atlas.config, "engineer")).toEqual({ text: "Tove, Ilya", known: true });
    expect(resolveWho(atlas.config, "designer")).toEqual({ text: "designer", known: false });
  });

  it("finds an agent through its deployment's team: ids are only unique within a team", () => {
    const orion = {
      id: "orion",
      config: { lead: "mara", agents: { mara: { name: "Auditor" } } },
    } as unknown as Team;
    const teams = [atlas, orion];
    expect(agentOf(teams, "atlas@ledger", "mara@ledger")?.soul.name).toBe("Mara");
    expect(agentOf(teams, "orion@ledger", "mara@ledger")?.soul.name).toBe("Auditor");
    expect(agentOf(teams, "orion@ledger", "mara@ledger")?.team.id).toBe("orion");
    // Another team having the id doesn't make it this team's.
    expect(agentOf(teams, "orion@ledger", "tove@ledger")).toBeUndefined();
    // A team file since deleted, or no deployment at all (Foreman's own requests).
    expect(agentOf(teams, "vesper@ledger", "mara@ledger")).toBeUndefined();
    expect(agentOf(teams, "", "mara@ledger")).toBeUndefined();
  });

  it("labels roles", () => {
    expect(["team_lead", "qa", "security_auditor"].map(roleLabel)).toEqual([
      "Team Lead",
      "QA / Tester",
      "Security auditor",
    ]);
  });
});

describe("roster", () => {
  it("puts the lead first, then the file's order, then any it doesn't name, sorted", () => {
    const c = {
      lead: "mara",
      agents: { mara: {}, tove: {}, ivo: {}, ada: {}, zed: {} },
      order: ["tove", "mara", "ivo", "gone"],
    } as unknown as Team["config"];
    expect(roster(c)).toEqual(["mara", "tove", "ivo", "ada", "zed"]);
  });
});

import { ago, when } from "./model";

describe("when", () => {
  // Local times, whatever the machine's zone: built from local parts.
  const at = (y: number, mo: number, d: number, h: number, mi: number) =>
    new Date(y, mo, d, h, mi).toISOString();
  const now = new Date(2026, 9, 17, 16, 0);
  it("shows the time today, the date before, the year before this one", () => {
    expect(when(at(2026, 9, 17, 9, 5), now)).toBe("09:05");
    expect(when(at(2026, 9, 15, 14, 30), now)).toBe("Oct 15 14:30");
    expect(when(at(2025, 11, 31, 23, 59), now)).toBe("Dec 31 2025 23:59");
  });
});

describe("ago", () => {
  const now = new Date("2026-10-29T12:00:00Z");
  const earlier = (seconds: number) => new Date(now.getTime() - seconds * 1000).toISOString();
  it("counts down to the largest whole unit, rounding down", () => {
    expect(ago(earlier(0), now)).toBe("just now");
    expect(ago(earlier(59), now)).toBe("just now");
    expect(ago(earlier(60), now)).toBe("1m ago");
    expect(ago(earlier(59 * 60 + 59), now)).toBe("59m ago");
    expect(ago(earlier(3600), now)).toBe("1h ago");
    expect(ago(earlier(24 * 3600 - 1), now)).toBe("23h ago");
    expect(ago(earlier(24 * 3600), now)).toBe("1d ago");
    expect(ago(earlier(400 * 24 * 3600), now)).toBe("400d ago");
  });
  it("calls a time ahead of now (another clock) just now", () => {
    expect(ago(earlier(-300), now)).toBe("just now");
  });
});

import { profileName } from "./model";

describe("profileName", () => {
  it("makes a typed name into a profile's, as settings.toml accepts it", () => {
    expect(profileName("work")).toBe("work");
    expect(profileName("  My Work!! ")).toBe("my-work");
    expect(profileName("Side__Projects 2026")).toBe("side-projects-2026");
    expect(profileName("../../etc")).toBe("etc");
    expect(profileName("¡¿")).toBe("");
    // Cut at 40, never ending on a dash.
    expect(profileName(`${"a".repeat(39)} b`)).toBe("a".repeat(39));
    expect(profileName("x".repeat(50))).toHaveLength(40);
  });
});

import { asAccent, asTheme } from "./theme";

describe("asTheme", () => {
  it("reads a stored or served theme, System unless Dark or Light", () => {
    expect(asTheme("dark")).toBe("dark");
    expect(asTheme("light")).toBe("light");
    for (const other of ["system", "neon", "", null, undefined])
      expect(asTheme(other)).toBe("system");
  });
});

describe("the accent", () => {
  it("reads a stored or served accent, blue unless one of them", () => {
    expect(asAccent("teal")).toBe("teal");
    for (const other of ["Teal", "neon", "", null, undefined]) expect(asAccent(other)).toBe("blue");
  });
});

import { waitingReason } from "./model";

describe("why a message waits", () => {
  const inst = (id: string, status: string) => ({ id, status }) as Instance;
  const all = [
    inst("a@p", "idle"),
    inst("b@p", "running"),
    inst("c@p", "queued"),
    inst("d@p", "asleep"),
  ];
  it("names what the recipient is doing", () => {
    expect(waitingReason(inst("c@p", "queued"), "Lumen", all, 2, false)).toBe(
      "Lumen hasn't started: 2 of 2 session slots are taken, idle agents included",
    );
    expect(waitingReason(inst("b@p", "running"), "Forge", all, 2, false)).toBe(
      "Forge is busy: it gets it when its turn ends",
    );
    expect(waitingReason(inst("a@p", "awaiting"), "Vale", all, 2, false)).toMatch(
      /waiting for you/,
    );
    expect(waitingReason(inst("d@p", "asleep"), "Nyx", all, 2, false)).toMatch(/asleep/);
    expect(waitingReason(inst("a@p", "idle"), "Vale", all, 2, true)).toMatch(/unsent line/);
    expect(waitingReason(inst("a@p", "idle"), "Vale", all, 2, false)).toBe("Being delivered");
    expect(waitingReason(undefined, "Ghost", all, 2, false)).toBe("Ghost isn't running");
  });
});

import { onOtherProfile } from "./model";

describe("onOtherProfile", () => {
  const inst = (id: string, status: string, profile: string, harness = "claude-code") =>
    ({ id, status, profile, harness }) as Instance;
  it("lists the project's running agents on another profile", () => {
    const list = [
      inst("mara@ledger", "running", "default"),
      inst("tove@ledger", "idle", "local-pi", "pi"),
      inst("ivo@ledger", "asleep", "default"), // starts on the new one anyway
      inst("nora@relay", "running", "default"), // another project
      inst("lia@ledger", "awaiting", ""), // from before profiles were kept: by harness
      inst("zed@ledger", "starting", "", "pi"),
    ];
    const ids = (p: { name: string; harness: string }) =>
      onOtherProfile(list, "ledger", p).map((i) => i.id);
    expect(ids({ name: "local-pi", harness: "pi" })).toEqual(["mara@ledger", "lia@ledger"]);
    expect(ids({ name: "default", harness: "claude-code" })).toEqual(["tove@ledger", "zed@ledger"]);
  });
  it("doesn't take a project for another that ends the same", () => {
    const list = [inst("mara@my-ledger", "running", "default")];
    expect(onOtherProfile(list, "ledger", { name: "pi", harness: "pi" })).toEqual([]);
  });
});

import { reloadLine, reloadOf } from "./model";

describe("reloadOf", () => {
  const inst = (id: string, status: string, behind = false) =>
    ({ id, agentId: id.split("@")[0], deploymentId: "atlas@ledger", status, behind }) as Instance;
  const teams = [
    {
      id: "atlas",
      config: { name: "Atlas", agents: { mara: { name: "Mara" }, tove: { name: "Tove" } } },
    },
  ] as unknown as Team[];

  it("offers it for the agents behind once nobody is busy", () => {
    const r = reloadOf([inst("mara@ledger", "idle", true), inst("tove@ledger", "asleep")]);
    expect(r.behind.map((i) => i.id)).toEqual(["mara@ledger"]);
    expect(r.busy).toBeUndefined();
    expect(reloadLine(r, teams)).toBe(
      "1 agent runs on an older prompt: Reload restarts it on the new one, continuing its conversation.",
    );
  });

  it("holds it while any agent is on its way up, working or waiting on the CEO", () => {
    for (const status of ["queued", "starting", "running", "awaiting"]) {
      const r = reloadOf([inst("mara@ledger", "idle", true), inst("tove@ledger", status)]);
      expect(r.busy?.id).toBe("tove@ledger");
    }
    // Asleep or stopped hold nothing back: they start on the new prompt anyway.
    for (const status of ["asleep", "stopped", "idle"]) {
      expect(reloadOf([inst("tove@ledger", status)]).busy).toBeUndefined();
    }
  });

  it("says who holds it back, and why", () => {
    const behind = [inst("mara@ledger", "idle", true), inst("ivo@ledger", "idle", true)];
    expect(reloadLine(reloadOf([...behind, inst("tove@ledger", "running")]), teams)).toBe(
      "2 agents run on an older prompt. Tove is busy: the change applies at each agent's next start. Reload once they're idle.",
    );
    expect(reloadLine(reloadOf([...behind, inst("tove@ledger", "awaiting")]), teams)).toMatch(
      /^2 agents run on an older prompt\. Tove is waiting on you:/,
    );
    // An agent no team file names any more goes by its id.
    expect(reloadLine(reloadOf([...behind, inst("zed@ledger", "running")]), teams)).toMatch(
      /\. zed is busy:/,
    );
  });

  it("offers nothing when nobody is behind", () => {
    expect(reloadOf([inst("mara@ledger", "running")]).behind).toEqual([]);
  });
});

import { teamTab } from "./model";

describe("teamTab", () => {
  it("opens a deployed team on Deployments, one not deployed on Members", () => {
    expect(teamTab(undefined, true)).toBe("deployments");
    expect(teamTab(undefined, false)).toBe("members");
  });

  it("keeps the tab the URL asks for, even Deployments with none", () => {
    expect(teamTab("deployments", false)).toBe("deployments");
    expect(teamTab("file", true)).toBe("file");
  });

  it("falls back on an unknown tab", () => {
    expect(teamTab("nope", false)).toBe("members");
    expect(teamTab(3, true)).toBe("deployments");
  });
});

import { statusColor } from "../components/ui";

describe("statusColor", () => {
  it("maps every status to its color", () => {
    expect(
      ["running", "awaiting", "idle", "done", "starting", "stopped", "queued"].map(statusColor),
    ).toEqual([
      "var(--ok)",
      "var(--attn)",
      "var(--tx4)",
      "var(--info)",
      "var(--team)",
      "var(--err)",
      "var(--tx4)",
    ]);
  });
});

import { readable } from "./model";

describe("readable", () => {
  it("reads a slug as words, only the first capital", () => {
    expect(readable("my-portfolio")).toBe("My portfolio");
    expect(readable("api")).toBe("Api");
    expect(readable("my__data--set_")).toBe("My data set");
    // A name from before slugs: only its separators change.
    expect(readable("Ledger.v2")).toBe("Ledger.v2");
  });
});

import { saysLead } from "./model";

describe("saysLead", () => {
  it("hears lead as a word or a word's start, any case", () => {
    for (const name of ["Lead", "lead", "Team Lead", "Tech-lead", "Leader", "QA_lead"]) {
      expect(saysLead(name), name).toBe(true);
    }
  });
  it("not inside a word, nor in other names", () => {
    for (const name of ["Mara", "Pleadra", "Misleader", "", "Le ad"]) {
      expect(saysLead(name), name).toBe(false);
    }
  });
});
