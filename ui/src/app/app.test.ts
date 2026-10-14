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
    expect(staleKeys({ type: "something.else", data: {} })).toEqual([]);
  });
});
