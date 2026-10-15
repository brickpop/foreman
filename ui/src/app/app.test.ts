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
    expect(staleKeys({ type: "config.changed", data: { object: "team" } })).toEqual([keys.teams]);
    expect(staleKeys({ type: "config.changed", data: { object: "settings" } })).toEqual([
      keys.settings,
    ]);
    expect(staleKeys({ type: "config.changed", data: { object: "project" } })).toEqual([
      keys.projects,
    ]);
    expect(staleKeys({ type: "workspace.changed", data: {} })).toEqual([keys.projects]);
    expect(staleKeys({ type: "instance.status", data: {} })).toEqual([keys.instances]);
    expect(staleKeys({ type: "request.created", data: {} })).toEqual([keys.requests]);
    expect(staleKeys({ type: "team.message", data: {} })).toEqual([keys.messages]);
    expect(staleKeys({ type: "deployment.created", data: {} })).toEqual([
      keys.instances,
      keys.deployments,
    ]);
    expect(staleKeys({ type: "wiki.changed", data: { project: "ledger" } })).toEqual([
      ["wiki", "ledger"],
    ]);
    expect(staleKeys({ type: "pipeline.changed", data: {} })).toEqual([["pipeline"]]);
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

import { palettes, randomLook, resolveWho, roleLabel } from "./model";

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
    expect(resolveWho(atlas, "lead")).toEqual({ text: "Mara", known: true });
    expect(resolveWho(atlas, "members")).toEqual({ text: "Members", known: true });
    expect(resolveWho(atlas, "ceo")).toEqual({ text: "You", known: true });
    expect(resolveWho(atlas, "tove")).toEqual({ text: "Tove", known: true });
    expect(resolveWho(atlas, "engineer")).toEqual({ text: "Tove, Ilya", known: true });
    expect(resolveWho(atlas, "designer")).toEqual({ text: "designer", known: false });
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
