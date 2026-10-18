import { describe, expect, it } from "vitest";
import type { Instance, Project, Request, Team } from "../api/client";
import { keys, staleKeys } from "./queries";
import { sidebarModel } from "./Shell";

const project = (id: string) => ({ id }) as Project;
const team = (id: string, name: string) => ({ id, config: { name } }) as Team;
const instance = (deploymentId: string) => ({ deploymentId }) as Instance;
const request = (projectId: string, status = "open") => ({ projectId, status }) as Request;

describe("sidebarModel", () => {
  it("counts open requests and marks the projects that need the CEO", () => {
    const m = sidebarModel(
      [project("ledger"), project("relay")],
      [],
      [],
      [request("ledger"), request("ledger"), request("relay", "resolved")],
    );
    expect(m.openRequests).toBe(2);
    expect(m.projects).toEqual([
      { id: "ledger", needsYou: true },
      { id: "relay", needsYou: false },
    ]);
  });

  it("says where each team is deployed", () => {
    const m = sidebarModel(
      [],
      [team("atlas", "Atlas"), team("vesper", "Vesper"), team("orion", "")],
      [
        instance("atlas@ledger"),
        instance("atlas@ledger"),
        instance("vesper@ledger"),
        instance("vesper@relay"),
      ],
      [],
    );
    expect(m.teams).toEqual([
      { id: "atlas", name: "Atlas", deployedOn: "ledger" },
      { id: "vesper", name: "Vesper", deployedOn: "2 projects" },
      { id: "orion", name: "orion", deployedOn: "—" },
    ]);
  });
});

describe("staleKeys", () => {
  it("invalidates what each event touches", () => {
    expect(staleKeys({ type: "config.changed", data: { object: "team" } })).toEqual([
      keys.teams,
      ["prompt"],
      keys.setup,
    ]);
    expect(staleKeys({ type: "config.changed", data: { object: "settings" } })).toEqual([
      keys.settings,
      keys.setup,
    ]);
    expect(staleKeys({ type: "config.changed", data: { object: "project" } })).toEqual([
      keys.projects,
      keys.setup,
    ]);
    expect(staleKeys({ type: "workspace.changed", data: {} })).toEqual([
      keys.projects,
      keys.setup,
      keys.repos,
    ]);
    expect(staleKeys({ type: "setup.changed", data: {} })).toEqual([keys.setup, keys.maintenance]);
    expect(staleKeys({ type: "instance.status", data: {} })).toEqual([keys.instances]);
    expect(staleKeys({ type: "request.created", data: {} })).toEqual([keys.requests]);
    expect(staleKeys({ type: "team.message", data: {} })).toEqual([keys.messages, keys.activity]);
    expect(staleKeys({ type: "team.action", data: {} })).toEqual([
      keys.actions,
      keys.activity,
      keys.repos,
    ]);
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

import { checksSentence, homeSubtitle, homeTitle, ordered, projectRows } from "./model";

const req = (id: string, kind: string, createdAt: string, projectId = "ledger") =>
  ({ id, kind, createdAt, projectId, status: "open" }) as Request;
const inst = (id: string, status: string, deploymentId: string) =>
  ({ id, status, deploymentId }) as Instance;

describe("Home's model", () => {
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

  it("titles and subtitles", () => {
    expect([homeTitle(0), homeTitle(1), homeTitle(3)]).toEqual([
      "All clear",
      "1 thing needs you",
      "3 things need you",
    ]);
    expect(
      homeSubtitle([
        inst("mara@ledger", "running", "atlas@ledger"),
        inst("tove@ledger", "idle", "atlas@ledger"),
        inst("lumen@relay", "running", "orion@relay"),
      ]),
    ).toBe("2 sessions working across 2 deployments.");
    expect(homeSubtitle([inst("mara@ledger", "running", "atlas@ledger")])).toBe(
      "1 session working across 1 deployment.",
    );
  });

  it("gives each project its line and state", () => {
    const rows = projectRows(
      [project("ledger"), project("relay"), project("harbor")],
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

  it("says how checks went", () => {
    expect(["pass", "fail", "running", ""].map(checksSentence)).toEqual([
      " Checks pass.",
      " Checks fail.",
      " Checks running.",
      "",
    ]);
  });
});

import { palettes, randomLook, resolveWho, roleLabel, roster } from "./model";

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

  it("labels roles", () => {
    expect(["team_lead", "qa", "security_auditor"].map(roleLabel)).toEqual([
      "Team Lead",
      "QA / Tester",
      "Security auditor",
    ]);
  });

  it("draws a random look from the palettes", () => {
    expect(randomLook(() => 0)).toEqual({
      color: palettes.color[0],
      skin: palettes.skin[0],
      hair: "short",
      hairColor: palettes.hairColor[0],
      eyes: "dots",
      beard: "none",
    });
    const look = randomLook();
    expect(palettes.hair).toContain(look.hair);
    expect(palettes.color).toContain(look.color);
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

import { when } from "./model";

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

import type { Settings } from "../api/client";
import { profileOf } from "./model";

describe("profileOf", () => {
  it("is the project's own profile, else the default", () => {
    const s = { projectProfiles: { ledger: "work" } } as unknown as Settings;
    expect(profileOf(s, "ledger")).toBe("work");
    expect(profileOf(s, "relay")).toBe("default");
    // A workspace whose settings name none: Go sends null.
    expect(profileOf({ projectProfiles: null } as unknown as Settings, "ledger")).toBe("default");
    expect(profileOf(undefined, "ledger")).toBe("default");
  });
});

import { asTheme } from "./theme";

describe("asTheme", () => {
  it("reads a stored or served theme, System unless Dark or Light", () => {
    expect(asTheme("dark")).toBe("dark");
    expect(asTheme("light")).toBe("light");
    for (const other of ["system", "neon", "", null, undefined])
      expect(asTheme(other)).toBe("system");
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
