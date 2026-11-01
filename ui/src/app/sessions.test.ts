import { describe, expect, it } from "vitest";
import type { Instance, Request, Team } from "../api/client";
import {
  activeDeployments,
  agentStep,
  applyMention,
  countdown,
  deploymentStatus,
  draftNotice,
  isLineShortcut,
  linePlaceholder,
  lineShortcut,
  mentionAt,
  mentionMatches,
  rosterOrder,
  sessionAgent,
  stepAgent,
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

describe("draftNotice", () => {
  it("asks for the line while the messages wait on it", () => {
    expect(draftNotice(1, "Mara", true)).toBe("1 message waiting for your line: send or clear it.");
  });
  it("says they still come once the line went out", () => {
    expect(draftNotice(1, "Mara", false)).toBe(
      "1 message waiting: it goes in when Mara's turn ends.",
    );
    expect(draftNotice(3, "Mara", false)).toBe(
      "3 messages waiting: they go in when Mara's turn ends.",
    );
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

describe("the agent a Session shows", () => {
  const inst = (agent: string) => ({ id: `${agent}@ledger`, agentId: agent }) as Instance;
  const roster = [inst("tic"), inst("tac"), inst("toe")];

  it("is the one asked, by id or full instance id", () => {
    expect(sessionAgent("toe", roster)).toBe("toe");
    expect(sessionAgent("toe@ledger", roster)).toBe("toe");
  });

  it("comes back to the last one shown on the project when none is asked", () => {
    expect(sessionAgent(undefined, roster, "tac")).toBe("tac");
    // A link naming an agent (a notification) means that agent, as it says.
    expect(sessionAgent("toe", roster, "tac")).toBe("toe");
  });

  it("is the lead, first in the roster, when neither is on the team", () => {
    expect(sessionAgent(undefined, roster)).toBe("tic");
    // Another team's agent (the one replaced), or one gone since a team change.
    expect(sessionAgent("lumen", roster, "left")).toBe("tic");
    // An unknown one asked still falls back to the last one shown.
    expect(sessionAgent("gone", roster, "tac")).toBe("tac");
  });

  it("is nothing with no team on the project", () => {
    expect(sessionAgent("tic", [], "tac")).toBeUndefined();
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

describe("moving between agents by keyboard", () => {
  const key = (code: string, mod: "ctrl" | "cmd", more = {}) => ({
    code,
    ctrlKey: mod === "ctrl",
    metaKey: mod === "cmd",
    shiftKey: true,
    altKey: false,
    ...more,
  });

  it("is Cmd+Shift+← / → on a Mac and Ctrl+Shift elsewhere", () => {
    expect(agentStep(key("ArrowLeft", "cmd"), true)).toBe(-1);
    expect(agentStep(key("ArrowRight", "cmd"), true)).toBe(1);
    expect(agentStep(key("ArrowLeft", "ctrl"), false)).toBe(-1);
    expect(agentStep(key("ArrowRight", "ctrl"), false)).toBe(1);
  });

  it("leaves the agent every other key", () => {
    expect(agentStep(key("ArrowLeft", "ctrl"), true)).toBe(0); // Ctrl is the terminal's on a Mac
    expect(agentStep(key("ArrowRight", "cmd"), false)).toBe(0);
    expect(agentStep(key("ArrowRight", "cmd", { shiftKey: false }), true)).toBe(0); // Cmd+→
    expect(agentStep(key("ArrowRight", "cmd", { altKey: true }), true)).toBe(0);
    expect(agentStep(key("ArrowUp", "cmd"), true)).toBe(0);
    expect(agentStep(key("KeyL", "cmd"), true)).toBe(0); // the Line input's
  });

  it("steps through the tabs in order, round the ends", () => {
    const roster = ["lead@p", "a@p", "b@p"];
    expect(stepAgent(roster, "a@p", 1)).toBe("b@p");
    expect(stepAgent(roster, "a@p", -1)).toBe("lead@p");
    expect(stepAgent(roster, "b@p", 1)).toBe("lead@p");
    expect(stepAgent(roster, "lead@p", -1)).toBe("b@p");
    expect(stepAgent(["lead@p"], "lead@p", 1)).toBe("lead@p");
    expect(stepAgent(roster, "gone@p", 1)).toBe("lead@p");
    expect(stepAgent([], "x@p", 1)).toBeUndefined();
  });
});
