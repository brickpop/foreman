import { describe, expect, it } from "vitest";
import type { Deployment, Profile, Project, Team } from "../api/client";
import { hasTeam, profileChoices, projectChoices, teamChoices } from "./deploy";

const project = (id: string, name = "") => ({ id, name }) as Project;
const on = (teamId: string, projectId: string) => ({ teamId, projectId }) as Deployment;
const team = (id: string, name: string) => ({ id, config: { name } }) as Team;
const profile = (name: string, harness: string, connected: boolean): Profile => ({
  name,
  harness,
  connected,
});
const teamName = (id: string) => ({ fullstack: "FullStack", writers: "Writers" })[id] ?? id;

describe("the Deploy dialog's choices", () => {
  const projects = [
    project("website"),
    project("productivity"),
    project("blog"),
    project("app-10"),
    project("app-2"),
  ];
  const deployments = [on("fullstack", "productivity"), on("writers", "blog")];

  it("lists the projects by name, a project another team is on still available", () => {
    expect(projectChoices(projects, deployments, "fullstack", teamName)).toEqual([
      { value: "app-2", label: "App 2" },
      { value: "app-10", label: "App 10" },
      { value: "blog", label: "Blog", detail: "Writers is on it" },
      { value: "website", label: "Website" },
      // The team is already here: last, greyed, with why.
      {
        value: "productivity",
        label: "Productivity",
        detail: "already has this team",
        disabled: true,
      },
    ]);
  });

  it("disables no project before a team is picked", () => {
    const choices = projectChoices(projects, deployments, undefined, teamName);
    expect(choices.filter((c) => c.disabled)).toEqual([]);
    expect(choices.find((c) => c.value === "productivity")?.detail).toBe("FullStack is on it");
  });

  it("knows a team on a project already, never before a team is picked", () => {
    expect(hasTeam(deployments, "productivity", "fullstack")).toBe(true);
    expect(hasTeam(deployments, "blog", "fullstack")).toBe(false);
    expect(hasTeam(deployments, "website", undefined)).toBe(false);
  });

  it("lists the teams by name", () => {
    expect(teamChoices([team("writers", "Writers"), team("fullstack", "FullStack")])).toEqual([
      { value: "fullstack", label: "FullStack" },
      { value: "writers", label: "Writers" },
    ]);
  });

  it("lists the connected profiles first, then the others, each with its harness", () => {
    const label = (h: string) => ({ "claude-code": "Claude Code", codex: "Codex" })[h] ?? h;
    expect(
      profileChoices(
        [
          profile("work", "codex", false),
          profile("personal", "claude-code", true),
          profile("alt", "claude-code", false),
          profile("main", "codex", true),
        ],
        label,
      ),
    ).toEqual([
      { value: "main", label: "main", detail: "Codex" },
      { value: "personal", label: "personal", detail: "Claude Code" },
      { value: "alt", label: "alt", detail: "Claude Code · not connected" },
      { value: "work", label: "work", detail: "Codex · not connected" },
    ]);
  });
});
