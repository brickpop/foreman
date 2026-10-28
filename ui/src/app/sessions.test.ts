import { describe, expect, it } from "vitest";
import type { Instance, Request, Team } from "../api/client";
import {
  activeDeployments,
  applyMention,
  byProject,
  countdown,
  deploymentStatus,
  isLineShortcut,
  linePlaceholder,
  lineShortcut,
  mentionAt,
  mentionMatches,
  mostRecentAgent,
  rosterOrder,
  type SessionsView,
  sessionsView,
  stoppedLine,
  withDoing,
} from "./sessions";

const inst = (id: string, deploymentId: string) =>
  ({ id, deploymentId, agentId: id.split("@")[0] }) as Instance;
const atlas = {
  id: "atlas",
  config: {
    name: "Atlas",
    lead: "mara",
    agents: { ilya: {}, mara: {}, tove: {} },
    order: ["tove", "mara", "ilya"],
  },
} as unknown as Team;

const all = [
  inst("tove@ledger", "atlas@ledger"),
  inst("mara@ledger", "atlas@ledger"),
  inst("ilya@ledger", "atlas@ledger"),
  inst("lumen@relay", "orion@relay"),
];

describe("Sessions", () => {
  it("lists the active deployments by project, then team name, with their needs-you dot", () => {
    const open = { status: "open", deploymentId: "orion@relay" } as Request;
    const vesper = { id: "vesper", config: { name: "Vesper" } } as unknown as Team;
    const list = [...all, inst("kit@ledger", "vesper@ledger"), inst("ana@harbor", "zed@harbor")];
    expect(activeDeployments(list, [atlas, vesper], [open])).toEqual([
      { id: "zed@harbor", team: "zed", project: "harbor", needsYou: false, status: "asleep" },
      { id: "atlas@ledger", team: "Atlas", project: "ledger", needsYou: false, status: "asleep" },
      { id: "vesper@ledger", team: "Vesper", project: "ledger", needsYou: false, status: "asleep" },
      { id: "orion@relay", team: "orion", project: "relay", needsYou: true, status: "asleep" },
    ]);
    // A resolved request, or one of another deployment, is no dot.
    const resolved = { status: "resolved", deploymentId: "atlas@ledger" } as Request;
    expect(activeDeployments(all, [atlas], [resolved])[0].needsYou).toBe(false);
  });

  it("shows a team's liveliest agent as its status", () => {
    const with_ = (...statuses: string[]) => statuses.map((status) => ({ status }) as Instance);
    expect(deploymentStatus(with_("asleep", "running", "idle"))).toBe("running");
    expect(deploymentStatus(with_("running", "awaiting"))).toBe("awaiting"); // waiting on the CEO first
    expect(deploymentStatus(with_("idle", "done", "asleep"))).toBe("idle");
    expect(deploymentStatus(with_("asleep", "stopped"))).toBe("asleep");
    expect(deploymentStatus(with_("weird"))).toBe("asleep");
    expect(deploymentStatus([])).toBe("asleep");
  });

  it("finds the agent that did something last", () => {
    const at = (id: string, deploymentId: string, doing?: string) =>
      ({ ...inst(id, deploymentId), doing: doing && { line: "Started", at: doing } }) as Instance;
    const list = [
      at("tove@ledger", "atlas@ledger", "2026-10-17T10:00:00Z"),
      at("mara@ledger", "atlas@ledger", "2026-10-17T11:30:00Z"),
      at("ilya@ledger", "atlas@ledger"),
      at("lumen@relay", "orion@relay", "2026-10-17T12:00:00Z"),
    ];
    expect(mostRecentAgent(list, "atlas@ledger")).toBe("mara");
    expect(mostRecentAgent([at("ilya@ledger", "atlas@ledger")], "atlas@ledger")).toBeUndefined();
    expect(mostRecentAgent(list, "nope@x")).toBeUndefined();
  });

  it("replaces one agent's activity line, leaving the rest", () => {
    const doing = { line: "Bash: go test ./...", at: "2026-10-18T10:00:00Z" };
    const list = [inst("tove@ledger", "atlas@ledger"), inst("mara@ledger", "atlas@ledger")];
    const got = withDoing(list, { id: "mara@ledger", doing });
    expect(got?.map((i) => i.doing)).toEqual([undefined, doing]);
    expect(list[1].doing).toBeUndefined(); // the cached list itself is never changed
    expect(withDoing(undefined, { id: "mara@ledger", doing })).toBeUndefined(); // nothing fetched yet
    expect(withDoing(list, { id: "nobody@x", doing })).toEqual(list);
  });

  it("puts the lead first, then the team file's order", () => {
    expect(rosterOrder("atlas@ledger", all, [atlas]).map((i) => i.agentId)).toEqual([
      "mara",
      "tove",
      "ilya",
    ]);
  });

  it("words the line input for each case", () => {
    expect(linePlaceholder("Mara", "awaiting", true)).toBe("Reply to Mara…");
    expect(linePlaceholder("Mara", "running", true)).toBe(
      "Tell Mara what you need. They’ll delegate.",
    );
    expect(linePlaceholder("Tove", "idle", false)).toBe("Direct instruction to Tove…");
  });
});

describe("countdown", () => {
  it("shows m:ss, rounded up", () => {
    expect(countdown(60_000)).toBe("1:00");
    expect(countdown(41_200)).toBe("0:42");
    expect(countdown(1)).toBe("0:01");
    expect(countdown(0)).toBe("0:00");
    expect(countdown(-5)).toBe("0:00");
  });
});

describe("the line input's shortcut", () => {
  const none = { code: "KeyL", ctrlKey: false, shiftKey: true, altKey: false, metaKey: false };
  const ctrl = { ...none, ctrlKey: true };
  const cmd = { ...none, metaKey: true };

  it("is Ctrl+Shift+L on a PC and Cmd+Shift+L on a Mac, by key position (any layout)", () => {
    expect(isLineShortcut(ctrl, false)).toBe(true);
    expect(isLineShortcut(cmd, true)).toBe(true);
    expect(lineShortcut(false)).toBe("Ctrl+Shift+L");
    expect(lineShortcut(true)).toBe("⌘⇧L");
  });

  it("leaves the agent its own keys", () => {
    // The other platform's combination, and Ctrl+Shift+L on a Mac (a terminal key there).
    expect(isLineShortcut(cmd, false)).toBe(false);
    expect(isLineShortcut(ctrl, true)).toBe(false);
    for (const mac of [false, true]) {
      const mod = mac ? cmd : ctrl;
      expect(isLineShortcut({ ...mod, shiftKey: false }, mac)).toBe(false); // Ctrl+L / Cmd+L
      expect(isLineShortcut({ ...mod, altKey: true }, mac)).toBe(false);
      expect(isLineShortcut({ ...mod, code: "KeyK" }, mac)).toBe(false);
      expect(isLineShortcut({ ...mod, ctrlKey: true, metaKey: true }, mac)).toBe(false);
    }
  });
});

describe("mentions in the line input", () => {
  it("finds the mention being typed before the caret", () => {
    expect(mentionAt("ask @Ta", 7)).toEqual({ start: 4, query: "Ta" });
    expect(mentionAt("@", 1)).toEqual({ start: 0, query: "" });
    expect(mentionAt("ask @Tac to", 11)).toBeNull(); // past it
    expect(mentionAt("mail me@host", 12)).toBeNull(); // not after a space
    expect(mentionAt("ask @Tö", 7)).toEqual({ start: 4, query: "Tö" });
  });
  it("offers the names that start with it, any case", () => {
    expect(mentionMatches(["Tic", "Tac", "Toe"], "t")).toEqual(["Tic", "Tac", "Toe"]);
    expect(mentionMatches(["Tic", "Tac", "Toe"], "TA")).toEqual(["Tac"]);
    expect(mentionMatches(["Tic"], "x")).toEqual([]);
  });
  it("puts the plain name in, then a space, keeping what follows", () => {
    expect(applyMention("ask @Ta now", 7, 4, "Tac")).toEqual({ text: "ask Tac  now", caret: 8 });
    expect(applyMention("@", 1, 0, "Toe")).toEqual({ text: "Toe ", caret: 4 });
  });
});

describe("the Sessions URL", () => {
  const inst = (agent: string, project = "ledger") =>
    ({ id: `${agent}@${project}`, agentId: agent }) as Instance;
  const rosters: Record<string, Instance[]> = {
    "atlas@ledger": [inst("tic"), inst("tac"), inst("toe")],
    "orion@relay": [inst("lumen", "relay")],
  };
  const deployments = Object.keys(rosters);
  const rosterOf = (d: string) => rosters[d] ?? [];
  const view = (search: object, last: Record<string, SessionsView> = {}) =>
    sessionsView(search, deployments, rosterOf, (d) => last[d]);

  it("is the list when it names no deployment, or one not deployed (stopped since)", () => {
    expect(view({})).toBe(undefined);
    expect(view({ agent: "tic", view: "activity" })).toBe(undefined);
    expect(view({ deployment: "gone@ledger", agent: "tac" })).toBe(undefined);
    // Not the first deployment by default: the one named, its lead.
    expect(view({ deployment: "orion@relay" })).toEqual({
      deployment: "orion@relay",
      agent: "lumen",
      view: undefined,
    });
  });

  it("reads the agent by id or full instance id, and an unknown one is the lead", () => {
    expect(view({ deployment: "atlas@ledger", agent: "toe" })?.agent).toBe("toe");
    expect(view({ deployment: "atlas@ledger", agent: "toe@ledger" })?.agent).toBe("toe");
    expect(view({ deployment: "atlas@ledger", agent: "gone" })?.agent).toBe("tic");
    // Another deployment's agent isn't this one's.
    expect(view({ deployment: "atlas@ledger", agent: "lumen" })?.agent).toBe("tic");
  });

  it("keeps Activity open as the URL says", () => {
    expect(view({ deployment: "atlas@ledger", agent: "tac", view: "activity" })?.view).toBe(
      "activity",
    );
  });

  it("comes back to a deployment's last view when the URL names only the deployment", () => {
    const last: Record<string, SessionsView> = {
      "atlas@ledger": { deployment: "atlas@ledger", agent: "toe", view: "activity" },
    };
    expect(view({ deployment: "atlas@ledger" }, last)).toEqual(last["atlas@ledger"]);
    // A link naming the agent (a notification) means that agent, as it says.
    expect(view({ deployment: "atlas@ledger", agent: "tac" }, last)).toEqual({
      deployment: "atlas@ledger",
      agent: "tac",
      view: undefined,
    });
    // A last agent gone since (a team change) is the lead.
    const gone = { "atlas@ledger": { deployment: "atlas@ledger", agent: "left" } };
    expect(view({ deployment: "atlas@ledger" }, gone)?.agent).toBe("tic");
  });

  it("is nothing when nothing is deployed", () => {
    expect(sessionsView({ deployment: "atlas@ledger" }, [], rosterOf, () => undefined)).toBe(
      undefined,
    );
  });
});

describe("stoppedLine", () => {
  it("says why Foreman stopped the agent", () => {
    expect(stoppedLine("Leady", `couldn't start: poc has no branch "main"`)).toBe(
      `Leady stopped: couldn't start: poc has no branch "main". Messages for them wait until you start them again; they pick up their conversation.`,
    );
  });
  it("says only that it stopped when there's no reason (from before reasons were kept)", () => {
    expect(stoppedLine("Leady", "")).toBe(
      "Leady stopped. Messages for them wait until you start them again; they pick up their conversation.",
    );
  });
});

describe("byProject", () => {
  it("groups items under their project, keeping their order", () => {
    const items = [
      { project: "harbor", id: 1 },
      { project: "ledger", id: 2 },
      { project: "ledger", id: 3 },
      { project: "relay", id: 4 },
    ];
    expect(byProject(items).map((g) => [g.project, g.items.map((i) => i.id)])).toEqual([
      ["harbor", [1]],
      ["ledger", [2, 3]],
      ["relay", [4]],
    ]);
    expect(byProject([])).toEqual([]);
  });
});
