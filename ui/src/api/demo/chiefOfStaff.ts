import { randomLook, roleLabel } from "../../app/model";
import type { ChiefOfStaffArea, CosConversation, CosEntry, CosPanel, Request } from "../client";
import type { Seed } from "./seed";

// The Chief of Staff on the demo client: the flows of
// wiki/product/features/chief-of-staff.md#flows, scripted. The real one is a
// harness session whose tool calls (say, present_choices, show_panel) feed the
// same conversation. Its changes come back as review requests; merging one
// applies it here, as the file watcher would after a real merge.

type Hire = {
  team?: string;
  role?: string;
  focus?: string;
  style?: string;
  harness?: string;
  command?: string;
  name?: string;
  changes: string[];
};
type Draft = {
  purpose?: string;
  name?: string;
  changes: string[];
  /** [role, name], the lead first. */
  roster?: [string, string][];
};

type Flow =
  | { flow: "none" }
  | {
      flow: "hire";
      step: "team" | "role" | "focus" | "style" | "harness" | "command" | "confirm";
      hire: Hire;
    }
  | { flow: "create-team"; step: "purpose" | "name" | "confirm" | "change"; draft: Draft }
  | { flow: "refine"; team: string; agent: string }
  | { flow: "project-config"; project: string };

const greeting =
  "I set up teams, interview you for new roles, and pick up anything the UI doesn’t cover.";
const startChips = ["Hire for a role", "Create a team", "What needs me today?"];
const roles = [
  {
    id: "engineer",
    label: "Engineer",
    focus: ["Backend APIs and migrations", "Frontend and UX polish"],
  },
  { id: "qa", label: "QA", focus: ["Property tests and fuzzing", "Load and concurrency"] },
  { id: "architect", label: "Architect", focus: ["Data model and storage", "Distributed systems"] },
  { id: "reviewer", label: "Reviewer", focus: ["Code review against the ADRs", "Security review"] },
];
const styles = [
  "Heads-down, reports when done",
  "Pushes back, challenges decisions",
  "Collaborative, checks in often",
];
const harnesses = [
  { id: "claude-code", label: "Claude Code" },
  { id: "codex", label: "Codex CLI" },
  { id: "pi", label: "Pi" },
  { id: "hermes", label: "Hermes Agent" },
  { id: "custom", label: "Custom" },
];
// spec (chief-of-staff.md, "Default instructions per role").
const instructions: Record<string, string> = {
  team_lead:
    "Break the goal into cards on the board. Assign cards and review every handoff before integrating. You are the only member who requests merges to main, with request_merge and the card that explains the work.",
  engineer:
    "Work only in your own worktree. Write the test first. Hand off to the lead with a summary and test results.",
  qa: "Try to break whatever is handed to you: property tests, fuzzing, races. File every finding with a repro. Never fix the code yourself.",
  architect:
    "Propose designs as ADRs with contribute (kind proposal). Back every claim with a spike and numbers.",
  reviewer:
    "Review diffs and ADRs for correctness and clarity. Approve or request changes. Never push code.",
};
// A drafted team's roles and playbook (the real Chief of Staff drafts them
// from the CEO's description and the example teams).
const draftRoles = ["team_lead", "engineer", "qa", "reviewer"];
const draftPlaybook: Record<string, string>[] = [
  { lead: "Break the goal into cards" },
  { engineer: "Build and hand off" },
  { qa: "Try to break it" },
  { reviewer: "Approve or request changes" },
  { lead: "Request the merge" },
];

// invented: the pools names are suggested from.
const agentNames = ["Nova", "Juno", "Rafa", "Ines", "Otto", "Lena", "Pim", "Sora", "Theo", "Vera"];
const teamNames = ["Orion", "Vega", "Lyra", "Polaris", "Draco", "Cygnus"];

export function demoChiefOfStaff(
  state: Seed,
  emit: (type: string, data: unknown) => void,
  options: { iso: () => string; replyMs: number },
): { area: ChiefOfStaffArea; resolved: (r: Request) => void } {
  const talk: CosConversation = {
    session: `Claude Code · ${state.workspace.path}/.foreman/cos`,
    items: [],
    choices: [],
    working: false,
  };
  let flow: Flow = { flow: "none" };
  /** What merging each Chief of Staff review applies. */
  const pending = new Map<string, () => void>();
  let configBranch = 0;

  const newVersion = () => Math.random().toString(16).slice(2, 10);
  const changed = () => emit("cos.changed", {});
  const say = (text: string, choices: string[] = []) => {
    talk.items.push({ id: talk.items.length + 1, from: "cos", text });
    talk.choices = choices;
  };
  const panel = (p: CosPanel, choices: string[]) => {
    talk.items.push({ id: talk.items.length + 1, from: "cos", text: "", panel: p });
    talk.choices = choices;
  };
  /** The CoS works for a moment, then answers. */
  const reply = (work: () => void) => {
    talk.working = true;
    changed();
    setTimeout(() => {
      talk.working = false;
      work();
      changed();
    }, options.replyMs);
  };

  const usedNames = () =>
    new Set(
      state.teams
        .flatMap((t) => [t.config.name, ...Object.values(t.config.agents).map((s) => s.name)])
        .map((n) => n.toLowerCase()),
    );
  const freeName = (pool: string[]) => pool.find((n) => !usedNames().has(n.toLowerCase()));
  const teamByName = (text: string) =>
    state.teams.find((t) =>
      [t.id, t.config.name].some((n) => n.toLowerCase() === text.trim().toLowerCase()),
    );
  const deploymentsOf = (team: string) =>
    state.deployments.filter((d) => d.teamId === team && d.status === "active").length;

  const raiseReview = (summary: string, projectId: string, apply: () => void) => {
    const r: Request = {
      id: `cos-${state.requests.length + 1}`,
      kind: "review",
      projectId,
      deploymentId: "",
      instanceId: "",
      summary,
      options: ["Merge to main", "Reject"],
      source: "cos",
      status: "open",
      answer: "",
      createdAt: options.iso(),
      resolvedAt: "0001-01-01T00:00:00Z",
      mergeRequestId: "",
      links: [],
    };
    state.requests.push(r);
    pending.set(r.id, apply);
    emit("request.created", r);
  };

  // Hire for a role.
  const askHire = (h: Hire) => {
    if (flow.flow !== "hire") return;
    switch (flow.step) {
      case "team":
        return say(
          "Which team is this hire for?",
          state.teams.map((t) => t.config.name),
        );
      case "role":
        return say(
          "What role?",
          roles.map((r) => r.label),
        );
      case "focus":
        return say(
          "What will they spend most of their time on? A sentence is enough.",
          roles.find((r) => r.id === h.role)?.focus ?? ["General work"],
        );
      case "style":
        return say("How should they work with the rest of the team?", styles);
      case "harness":
        return say(
          "Which harness should run them?",
          harnesses.map((x) => x.label),
        );
      case "command":
        return say("What command starts it?");
      case "confirm":
        return showHire(h);
    }
  };
  const hireJob = (h: Hire) =>
    [
      instructions[h.role ?? ""] ?? "",
      `Focus: ${h.focus}.`,
      ...h.changes.map((c) => `Adjustment: ${c}.`),
    ]
      .join(" ")
      .trim();
  const showHire = (h: Hire) => {
    const team = state.teams.find((t) => t.id === h.team);
    if (!team) return;
    if (!h.name) h.name = freeName(agentNames);
    if (!h.name) return say("I’ve run out of names to suggest. What should we call them?");
    const n = deploymentsOf(team.id);
    say(
      n > 0
        ? `Here’s the job description. They’ll start on all ${n} of ${team.config.name}’s deployments.`
        : `Here’s the job description. ${team.config.name} isn’t deployed yet, so they’ll start when it is.`,
    );
    const role = roles.find((r) => r.id === h.role)?.label ?? h.role ?? "";
    panel(
      {
        title: `${h.name} · ${role} · ${team.config.name}`,
        rows: [
          ["Focus", h.focus ?? ""],
          ["Works", h.style ?? ""],
          [
            "Harness",
            h.harness === "custom"
              ? `Custom · ${h.command}`
              : (harnesses.find((x) => x.id === h.harness)?.label ?? ""),
          ],
          ["Job", hireJob(h)],
        ],
      },
      [`Hire ${h.name}`, "Start over"],
    );
  };
  const answerHire = (text: string, h: Hire) => {
    if (flow.flow !== "hire") return;
    const t = text.trim();
    switch (flow.step) {
      case "team": {
        const team = teamByName(t);
        if (!team)
          return say(
            `I don’t know a team called “${t}”. Which team is this hire for?`,
            state.teams.map((x) => x.config.name),
          );
        h.team = team.id;
        flow.step = "role";
        break;
      }
      case "role":
        h.role =
          roles.find((r) => r.label.toLowerCase() === t.toLowerCase())?.id ??
          t.toLowerCase().replaceAll(" ", "_");
        if (h.role === "team_lead")
          return say(
            "A team has one lead, and it already has one. Which other role?",
            roles.map((r) => r.label),
          );
        flow.step = "focus";
        break;
      case "focus":
        h.focus = t;
        flow.step = "style";
        break;
      case "style":
        h.style = t;
        flow.step = "harness";
        break;
      case "harness": {
        const harness = harnesses.find((x) => x.label.toLowerCase() === t.toLowerCase());
        if (!harness)
          return say(
            `I don’t know the harness “${t}”. Which harness should run them?`,
            harnesses.map((x) => x.label),
          );
        h.harness = harness.id;
        flow.step = harness.id === "custom" ? "command" : "confirm";
        break;
      }
      case "command":
        h.command = t;
        flow.step = "confirm";
        break;
      case "confirm":
        if (t === "Start over") {
          flow = { flow: "hire", step: "team", hire: { changes: [] } };
          return askHire(flow.hire);
        }
        if (t === `Hire ${h.name}`) return hire(h);
        // A single word is a name; anything else changes the job description.
        if (/^\p{L}+$/u.test(t)) {
          if (usedNames().has(t.toLowerCase()))
            return say(`${t} is already taken in this workspace. What else should we call them?`);
          h.name = t.charAt(0).toUpperCase() + t.slice(1);
        } else {
          h.changes.push(t);
        }
        break;
    }
    askHire(h);
  };
  const hire = (h: Hire) => {
    const team = state.teams.find((t) => t.id === h.team);
    const name = h.name;
    if (!team || !name) return;
    raiseReview(
      `Chief of Staff changed team ${team.config.name}: hire ${name}, ${h.role?.replaceAll("_", " ")}.`,
      "",
      () => {
        const id = name.toLowerCase();
        team.config.agents[id] = {
          name,
          role: h.role ?? "engineer",
          harness: h.harness ?? "claude-code",
          command: h.command ?? "",
          personality: h.style ?? "",
          skills: [],
          instruction: hireJob(h),
          look: randomLook(),
        };
        team.version = newVersion();
        emit("config.changed", { object: "team", id: team.id, version: team.version });
      },
    );
    flow = { flow: "none" };
    say(`${name} is ready to join ${team.config.name}: it’s waiting for you as a review.`, [
      "Open the review",
      "Hire for a role",
    ]);
  };

  // Create a team.
  /** Names the draft's roster, once: the panel and the merged file agree. */
  const roster = (d: Draft): [string, string][] => {
    if (d.roster) return d.roster;
    const taken = new Set<string>();
    const next = () => {
      const n = agentNames.find(
        (x) => !usedNames().has(x.toLowerCase()) && !taken.has(x.toLowerCase()),
      );
      if (n) taken.add(n.toLowerCase());
      return n ?? "";
    };
    d.roster = draftRoles
      .map((role): [string, string] => [role, next()])
      .filter(([, n]) => n !== "");
    return d.roster;
  };
  const teamPanel = (d: Draft) => {
    say(
      "Here’s the team I’d start with. I’ll draft a job description for each role, and you can refine any of them later.",
    );
    panel(
      {
        title: d.name ?? "",
        rows: [
          ["Purpose", d.purpose ?? ""],
          ...roster(d).map(([role, n]): [string, string] => [roleLabel(role), n]),
          ["Playbook", draftPlaybook.map((step) => Object.values(step)[0]).join(" → ")],
          ...d.changes.map((c): [string, string] => ["Change", c]),
        ],
      },
      [`Create ${d.name}`, "Change something", "Start over"],
    );
  };
  const answerCreate = (text: string, d: Draft) => {
    if (flow.flow !== "create-team") return;
    const t = text.trim();
    switch (flow.step) {
      case "purpose":
        d.purpose = t;
        flow.step = "name";
        return say(
          "What should we call it?",
          teamNames.filter((n) => !usedNames().has(n.toLowerCase())).slice(0, 3),
        );
      case "name":
        if (usedNames().has(t.toLowerCase()))
          return say(`${t} is already taken in this workspace. What should we call it?`);
        d.name = t;
        flow.step = "confirm";
        return teamPanel(d);
      case "change":
        d.changes.push(t);
        flow.step = "confirm";
        return teamPanel(d);
      case "confirm":
        if (t === "Start over") {
          flow = { flow: "create-team", step: "purpose", draft: { changes: [] } };
          return say("What is the new team for? A sentence or two is enough.");
        }
        if (t === "Change something") {
          flow.step = "change";
          return say("What should change?");
        }
        if (t === `Create ${d.name}`) return create(d);
        d.changes.push(t);
        return teamPanel(d);
    }
  };
  const create = (d: Draft) => {
    const name = d.name ?? "";
    const id = name.toLowerCase().replaceAll(/[^a-z0-9]+/g, "-");
    const members = roster(d);
    raiseReview(`Chief of Staff changed team ${name}: a new team, “${d.purpose}”.`, "", () => {
      const agents = Object.fromEntries(
        members.map(([role, n]) => [
          n.toLowerCase(),
          {
            name: n,
            role,
            harness: "claude-code",
            command: "",
            personality: "",
            skills: [],
            instruction: [instructions[role], ...d.changes.map((c) => `Adjustment: ${c}.`)].join(
              " ",
            ),
            look: randomLook(),
          },
        ]),
      );
      const team = {
        id,
        config: {
          team: id,
          name,
          purpose: d.purpose ?? "",
          lead: members[0]?.[1].toLowerCase() ?? "",
          playbook: draftPlaybook,
          escalate: [{ when: "a gate in project.toml", to: "ceo" }],
          agents,
        },
        version: newVersion(),
        problems: [],
        source: "",
      };
      state.teams.push(team);
      emit("config.changed", { object: "team", id, version: team.version });
    });
    flow = { flow: "none" };
    say(`${name} is drafted. Once you merge it, deploy it from its page.`, ["Open the review"]);
  };

  const needsMe = () => {
    const open = state.requests.filter((r) => r.status === "open");
    if (open.length === 0) return "Nothing needs you right now.";
    const kinds = new Map<string, number>();
    for (const r of open) kinds.set(r.kind, (kinds.get(r.kind) ?? 0) + 1);
    const parts = [...kinds].map(([k, n]) => `${n} ${k}${n === 1 ? "" : "s"}`);
    return `${open.length} thing${open.length === 1 ? "" : "s"} need${open.length === 1 ? "s" : ""} you: ${parts.join(", ")}. They’re on Home.`;
  };

  const answer = (text: string) => {
    const t = text.trim();
    switch (flow.flow) {
      case "hire":
        return answerHire(t, flow.hire);
      case "create-team":
        return answerCreate(t, flow.draft);
      case "refine": {
        const team = state.teams.find((x) => x.id === (flow as { team: string }).team);
        const agent = (flow as { agent: string }).agent;
        const soul = team?.config.agents[agent];
        flow = { flow: "none" };
        if (!team || !soul) return say("I can’t find that agent any more.");
        raiseReview(
          `Chief of Staff changed team ${team.config.name}: ${soul.name}, “${t}”.`,
          "",
          () => {
            soul.instruction = `${soul.instruction} Adjustment: ${t}.`.trim();
            team.version = newVersion();
            emit("config.changed", { object: "team", id: team.id, version: team.version });
          },
        );
        return say(
          `Updated ${soul.name}’s job description on a branch. Once you merge it, every running session picks it up on its next turn.`,
        );
      }
      case "project-config": {
        const project = flow.project;
        flow = { flow: "none" };
        configBranch++;
        raiseReview(
          `Chief of Staff changed ${project}/project/project.toml: ${t}.`,
          project,
          () => {},
        );
        return say(
          `Done: “${t}”. It’s on branch foreman/config-${configBranch} and waiting for you in ${project} → Changes.`,
        );
      }
      case "none":
        if (t === "Hire for a role") {
          flow = { flow: "hire", step: "team", hire: { changes: [] } };
          return askHire(flow.hire);
        }
        if (t === "Create a team") {
          flow = { flow: "create-team", step: "purpose", draft: { changes: [] } };
          return say("What is the new team for? A sentence or two is enough.");
        }
        if (t === "What needs me today?") return say(needsMe());
        if (t === "Open the review")
          return say("It’s at the top of Home, with Merge to main and Reject.");
        return say(
          `On it. I’ll work on “${t}” and report back on Home when it’s done or if I need you.`,
        );
    }
  };

  const start = (entry: CosEntry) => {
    switch (entry.flow) {
      case "open":
        if (talk.items.length === 0) say(greeting, startChips);
        return;
      case "hire": {
        const team = state.teams.find((t) => t.id === entry.team);
        flow = {
          flow: "hire",
          step: team ? "role" : "team",
          hire: { team: team?.id, changes: [] },
        };
        if (team)
          return say(
            `Hiring for ${team.config.name}. What role?`,
            roles.map((r) => r.label),
          );
        return askHire(flow.hire);
      }
      case "create-team":
        flow = { flow: "create-team", step: "purpose", draft: { changes: [] } };
        return say("What is the new team for? A sentence or two is enough.");
      case "refine": {
        const soul = state.teams.find((t) => t.id === entry.team)?.config.agents[entry.agent];
        flow = { flow: "refine", team: entry.team, agent: entry.agent };
        return say(
          `What should change about how ${soul?.name ?? entry.agent} works? This applies on every project they’re deployed to.`,
          ["Push back more", "Communicate more", "Narrower focus"],
        );
      }
      case "project-config":
        flow = { flow: "project-config", project: entry.project };
        return say(
          `What should change in ${entry.project}/project/project.toml? I’ll edit it on a branch and send it to you as a review.`,
          ["Use GitHub as the forge", "Add a gate", "Change the checks"],
        );
    }
  };

  return {
    area: {
      demo: true,
      conversation: () => Promise.resolve(structuredClone(talk)),
      start: async (entry) => {
        start(entry);
        changed();
      },
      send: async (text) => {
        if (text.trim() === "") return;
        talk.items.push({ id: talk.items.length + 1, from: "ceo", text: text.trim() });
        talk.choices = [];
        reply(() => answer(text));
      },
    },
    resolved: (r) => {
      const apply = pending.get(r.id);
      pending.delete(r.id);
      if (apply && r.answer === "Merge to main") apply();
    },
  };
}
