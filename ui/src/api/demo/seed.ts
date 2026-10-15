// Demo data of the UI prototype (wiki/product/design/prototype/foreman-prototype.dc.html),
// in the shapes of the generated API types. Plain data: the demo client serves it as is.
// Where the prototype and the specs (wiki/product/) disagree, the specs win; each such
// place carries a "spec:" comment. "invented:" marks data the prototype doesn't have.
// "Now" in the demo is 2026-10-14T09:45:00Z.

import type { Status } from "../types/harness";
import type {
  Deployment,
  Instance,
  MergeRequest,
  Request,
  TeamMessage,
  WorkItem,
} from "../types/store";
import type { Board, Entry, Node } from "../types/wiki";
import type { Candidate, Project, SettingsFile, Team } from "../types/workspace";

export interface Seed {
  workspace: { path: string };
  projects: Project[];
  teams: Team[];
  settings: SettingsFile;
  harnesses: Status[];
  deployments: Deployment[];
  instances: Instance[];
  requests: Request[];
  messages: TeamMessage[];
  workItems: WorkItem[];
  mergeRequests: MergeRequest[];
  /** Folders that aren't projects yet (Add a project). */
  candidates: Candidate[];
  /** Per project id. */
  wiki: Record<string, { tree: Node[]; entries: Record<string, Entry>; boards: Board[] }>;
}

const UNSET = "0001-01-01T00:00:00Z";

// spec (agent-soul.md, "Default job descriptions"): the lead requests merges, it doesn't
// open PRs as the prototype's INSTR said.
const TEAM_LEAD =
  "Break the goal into cards on the board.\n" +
  "Assign cards and review every handoff before integrating.\n" +
  "You are the only member who requests merges to main " +
  "(request_merge, with the card that explains the work).";
const ENGINEER =
  "Work only in your own worktree.\n" +
  "Write the test first. Hand off to the lead with a summary and test results.";
const QA =
  "Try to break whatever is handed to you: property tests, fuzzing, races.\n" +
  "File every finding with a repro. Never fix the code yourself.";
const ARCHITECT =
  "Propose designs as ADRs in wiki/adr/.\nBack every claim with a spike and numbers.";
const REVIEWER =
  "Review diffs and ADRs for correctness and clarity.\n" +
  "Approve or request changes. Never push code.";
// The CEO dropped the prototype's Scrum Master role (only the lead writes the wiki):
// its Quill and Pike are reviewers here.

export const seed: Seed = {
  // invented: two folders with repos cloned but no project yet.
  candidates: [
    { folder: "billing", repos: ["billing-api", "billing-web"] },
    { folder: "scratch", repos: [] },
  ],
  workspace: { path: "~/agents-ws" },

  // Prototype: `projects` (lines 914-918).
  projects: [
    {
      id: "ledger",
      config: {
        project: "ledger",
        description: "Settlement and reconciliation service.",
        repos: ["ledger-api", "ledger-web"],
        main: "main",
        checks: "npm test",
        mergeToMain: "ceo",
        alwaysAskCeo: [
          "new dependency",
          "touches shared staging",
          "scope change vs an accepted ADR",
        ],
        // URLs from the project-config.md example.
        forge: {
          kind: "forgejo",
          url: "https://localhost:3000/ledger",
          projectRepo: "https://localhost:3000/ledger/ledger-project",
        },
      },
      repos: ["ledger-api", "ledger-web"],
      version: "84862373137c6b55",
      problems: [],
      source: `project = "ledger"
description = "Settlement and reconciliation service."
repos = ["ledger-api", "ledger-web"]
checks = "npm test"
always_ask_ceo = [
  "new dependency",
  "touches shared staging",
  "scope change vs an accepted ADR",
]

[forge]
kind = "forgejo"
url = "https://localhost:3000/ledger"
project_repo = "https://localhost:3000/ledger/ledger-project"
`,
    },
    {
      id: "relay",
      config: {
        project: "relay",
        description: "Event routing between internal services.",
        repos: ["relay-core"],
        main: "main",
        checks: "cargo test",
        mergeToMain: "ceo",
        alwaysAskCeo: ["adds a paid service"],
        // invented: the GitHub owner and project repo URL.
        forge: {
          kind: "github",
          url: "https://github.com/acme",
          projectRepo: "https://github.com/acme/relay-project",
        },
      },
      repos: ["relay-core"],
      version: "6a674502691ad312",
      problems: [],
      source: `project = "relay"
description = "Event routing between internal services."
repos = ["relay-core"]
checks = "cargo test"
always_ask_ceo = ["adds a paid service"]

[forge]
kind = "github"
url = "https://github.com/acme"
project_repo = "https://github.com/acme/relay-project"
`,
    },
    {
      id: "harbor",
      config: {
        project: "harbor",
        description: "Infrastructure as code for the fleet.",
        repos: ["harbor-infra", "harbor-modules"],
        main: "main",
        checks: "terraform validate",
        mergeToMain: "ceo",
        alwaysAskCeo: ["any apply outside sandbox accounts"],
        forge: { kind: "none", url: "", projectRepo: "" },
      },
      repos: ["harbor-infra", "harbor-modules"],
      version: "d1610f71061f5470",
      problems: [],
      source: `project = "harbor"
description = "Infrastructure as code for the fleet."
repos = ["harbor-infra", "harbor-modules"]
checks = "terraform validate"
always_ask_ceo = ["any apply outside sandbox accounts"]
`,
    },
  ],

  // Prototype: `agents`, `faces`, `cols` and `teams` in seed() (lines 932-963).
  // spec (team-definition.md): teams have a free-text `purpose`, not a kind (build,
  // challenge, architect); the purposes are invented from the spec's example teams.
  // Playbook `who`s are snake_case roles.
  teams: [
    {
      id: "atlas",
      config: {
        team: "atlas",
        name: "Atlas",
        purpose: "Builds features end to end, tested and reviewed.",
        lead: "mara",
        // spec: the team-definition.md Atlas example. The prototype had a first Scrum
        // Master step and "opens the PR to main" as the last one.
        playbook: [
          {
            lead:
              "Turns the goal into cards on the board and assigns them. " +
              "Members work in their own worktrees",
          },
          { members: "Build, then hand off to the lead with tests" },
          { qa: "Challenges every handoff. Failures go back to the author" },
          { lead: "Integrates, then requests the merge to main" },
        ],
        escalate: [{ when: "Blocked for more than 30 minutes", to: "lead" }],
        agents: {
          mara: {
            name: "Mara",
            role: "team_lead",
            harness: "claude-code",
            command: "",
            personality: "Calm, decisive, terse. Prefers small PRs.",
            skills: ["decomposition", "code review"],
            instruction: TEAM_LEAD,
            look: {
              color: "#D9825B",
              skin: "#E2AE85",
              hair: "bun",
              hairColor: "#2B2420",
              eyes: "dots",
              beard: "none",
            },
          },
          ilya: {
            name: "Ilya",
            role: "engineer",
            harness: "codex",
            command: "",
            personality: "Methodical. Writes the test first, distrusts floating point.",
            skills: ["TypeScript", "numerics"],
            instruction: ENGINEER,
            look: {
              color: "#6FA8DC",
              skin: "#F1C9A5",
              hair: "short",
              hairColor: "#6B4A2F",
              eyes: "glasses",
              beard: "stubble",
            },
          },
          tove: {
            name: "Tove",
            role: "engineer",
            harness: "claude-code",
            command: "",
            personality: "Fast and pragmatic. Ships small, then hardens.",
            skills: ["API design", "migrations"],
            instruction: ENGINEER,
            look: {
              color: "#8DB86B",
              skin: "#C38A62",
              hair: "long",
              hairColor: "#D8B46A",
              eyes: "dots",
              beard: "none",
            },
          },
          rhea: {
            name: "Rhea",
            role: "qa",
            harness: "pi",
            command: "",
            personality: "Skeptical by default.",
            skills: ["property testing"],
            instruction: QA,
            look: {
              color: "#E07A9A",
              skin: "#8D5A3B",
              hair: "curly",
              hairColor: "#2B2420",
              eyes: "sleepy",
              beard: "none",
            },
          },
          quill: {
            name: "Quill",
            role: "reviewer",
            harness: "claude-code",
            command: "",
            personality: "Tidy and persistent.",
            skills: ["code review"],
            instruction: REVIEWER,
            look: {
              color: "#8E9AAF",
              skin: "#F1C9A5",
              hair: "short",
              hairColor: "#A8A8A8",
              eyes: "glasses",
              beard: "mustache",
            },
          },
        },
      },
      version: "85c8f830451a06bf",
      problems: [],
    },
    {
      id: "vesper",
      config: {
        team: "vesper",
        name: "Vesper",
        purpose: "Tries to break what others built.",
        lead: "kestrel",
        playbook: [
          { lead: "Picks targets from what other teams hand off" },
          { qa: "Attack in their own sandbox. Every finding needs a repro" },
          // spec (requests.md, "Finding routing"): a finding goes to the CEO only; the
          // prototype had the lead send it to the owning team's lead.
          { lead: "Escalates high-severity findings to the CEO" },
        ],
        escalate: [{ when: "Finding of high severity", to: "lead" }],
        agents: {
          kestrel: {
            name: "Kestrel",
            role: "team_lead",
            harness: "claude-code",
            command: "",
            personality: "Adversarial but disciplined.",
            skills: ["threat modeling"],
            instruction: TEAM_LEAD,
            look: {
              color: "#B48BD6",
              skin: "#5E3B26",
              hair: "bald",
              hairColor: "#2B2420",
              eyes: "shades",
              beard: "full",
            },
          },
          nyx: {
            name: "Nyx",
            role: "qa",
            harness: "hermes",
            command: "",
            personality: "Chaotic on purpose.",
            skills: ["fuzzing", "Unicode"],
            instruction: QA,
            look: {
              color: "#4FB3A9",
              skin: "#E2AE85",
              hair: "long",
              hairColor: "#A4482C",
              eyes: "shades",
              beard: "none",
            },
          },
          bram: {
            name: "Bram",
            role: "qa",
            harness: "custom",
            // spec (team-definition.md example): "./bin/loadbot"; the prototype's
            // "--acp" flag belongs to the dropped ACP transport.
            command: "./bin/loadbot",
            personality: "Patient and stubborn.",
            skills: ["concurrency"],
            instruction: QA,
            look: {
              color: "#CDB46B",
              skin: "#C38A62",
              hair: "short",
              hairColor: "#6B4A2F",
              eyes: "dots",
              beard: "full",
            },
          },
        },
      },
      version: "58d401c99355845b",
      problems: [],
    },
    {
      id: "orion",
      config: {
        team: "orion",
        name: "Orion",
        purpose: "Designs: ADRs, spikes, reviews of designs.",
        lead: "lumen",
        // Only the lead writes the wiki, and ADRs live in it, not behind a PR (the
        // prototype had a Scrum Master write requirements and the lead open a PR).
        playbook: [
          { lead: "Drafts the ADR" },
          { architect: "Runs spikes to back or refute it" },
          { reviewer: "Reviews the draft for correctness and clarity" },
          { lead: "Records the accepted ADR in the wiki" },
        ],
        escalate: [{ when: "Spike contradicts the draft", to: "lead" }],
        agents: {
          lumen: {
            name: "Lumen",
            role: "team_lead",
            harness: "claude-code",
            command: "",
            personality: "Socratic. Argues both sides first.",
            skills: ["distributed systems"],
            instruction: TEAM_LEAD,
            look: {
              color: "#6FA8DC",
              skin: "#8D5A3B",
              hair: "curly",
              hairColor: "#A8A8A8",
              eyes: "glasses",
              beard: "none",
            },
          },
          arden: {
            name: "Arden",
            role: "architect",
            harness: "codex",
            command: "",
            personality: "Prefers a benchmark to an opinion.",
            skills: ["benchmarks"],
            instruction: ARCHITECT,
            look: {
              color: "#D9825B",
              skin: "#F1C9A5",
              hair: "short",
              hairColor: "#D8B46A",
              eyes: "dots",
              beard: "stubble",
            },
          },
          pike: {
            name: "Pike",
            role: "reviewer",
            harness: "pi",
            command: "",
            personality: "Turns debates into one-page decisions.",
            skills: ["ADR review"],
            instruction: REVIEWER,
            look: {
              color: "#E07A9A",
              skin: "#E2AE85",
              hair: "bun",
              hairColor: "#A4482C",
              eyes: "dots",
              beard: "none",
            },
          },
        },
      },
      version: "6e64c84cb3ea1bc5",
      problems: [],
    },
    {
      id: "sentinel",
      config: {
        team: "sentinel",
        name: "Sentinel",
        purpose: "Reviews changes for security and correctness, then tries to break them.",
        lead: "hale",
        // The prototype's "Scopes the review with you" reads wrong in a session prompt.
        playbook: [
          { lead: "Scopes the review with the CEO" },
          { reviewer: "Reads every change against the wiki" },
          { qa: "Tries to break what was reviewed" },
          { lead: "Writes the findings report in the wiki" },
        ],
        escalate: [{ when: "Anything that looks like a security issue", to: "ceo" }],
        agents: {
          hale: {
            name: "Hale",
            role: "team_lead",
            harness: "hermes",
            command: "",
            personality: "Careful and thorough.",
            skills: ["security review"],
            instruction: TEAM_LEAD,
            look: {
              color: "#CDB46B",
              skin: "#5E3B26",
              hair: "short",
              hairColor: "#A8A8A8",
              eyes: "sleepy",
              beard: "full",
            },
          },
          petra: {
            name: "Petra",
            role: "reviewer",
            harness: "pi",
            command: "",
            personality: "Exacting about wording and assumptions.",
            skills: ["review"],
            instruction: REVIEWER,
            look: {
              color: "#B48BD6",
              skin: "#F1C9A5",
              hair: "long",
              hairColor: "#6B4A2F",
              eyes: "glasses",
              beard: "none",
            },
          },
          odo: {
            name: "Odo",
            role: "qa",
            harness: "codex",
            command: "",
            personality: "Relentless about edge cases.",
            skills: ["fuzzing"],
            instruction: QA,
            look: {
              color: "#8DB86B",
              skin: "#C38A62",
              hair: "bald",
              hairColor: "#2B2420",
              eyes: "dots",
              beard: "mustache",
            },
          },
        },
      },
      version: "9e0b35d50b78d0c1",
      problems: [],
    },
  ],

  // The defaults (workspace-settings.md); version "" while settings.toml doesn't exist.
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

  // invented: the prototype has no login state; every harness is connected.
  harnesses: [
    { id: "claude-code", connected: true },
    { id: "codex", connected: true },
    { id: "pi", connected: true },
    { id: "hermes", connected: true },
  ],

  // Prototype: `deps` (lines 967-972). invented: the times.
  deployments: [
    {
      id: "atlas@ledger",
      teamId: "atlas",
      projectId: "ledger",
      goal: "Ship settlement engine v2 as specified in ADR-014, behind a feature flag.",
      status: "active",
      createdAt: "2026-10-12T08:00:00Z",
      stoppedAt: UNSET,
    },
    {
      id: "vesper@ledger",
      teamId: "vesper",
      projectId: "ledger",
      goal: "Break settlement v2 before users do.",
      status: "active",
      createdAt: "2026-10-13T10:00:00Z",
      stoppedAt: UNSET,
    },
    {
      id: "orion@relay",
      teamId: "orion",
      projectId: "relay",
      goal: "Propose the event-bus topology for relay v1.",
      status: "active",
      createdAt: "2026-10-13T14:00:00Z",
      stoppedAt: UNSET,
    },
    {
      id: "vesper@relay",
      teamId: "vesper",
      projectId: "relay",
      goal: "Stress the JetStream spike: oversized messages, partitions, slow consumers.",
      status: "active",
      createdAt: "2026-10-14T08:30:00Z",
      stoppedAt: UNSET,
    },
  ],

  // Prototype: `inst` (lines 973-978). invented: the times and Arden's restarts.
  // Sandboxes per spec (instance.md): <project>/.sandboxes/<agent>.
  instances: [
    {
      id: "mara@ledger",
      deploymentId: "atlas@ledger",
      agentId: "mara",
      status: "running",
      task: "Reviewing Ilya’s handoff",
      sandboxPath: "~/agents-ws/ledger/.sandboxes/mara",
      harness: "claude-code",
      startedAt: "2026-10-12T08:01:00Z",
      lastOutputAt: "2026-10-14T09:44:00Z",
      restarts: 0,
    },
    {
      id: "ilya@ledger",
      deploymentId: "atlas@ledger",
      agentId: "ilya",
      status: "awaiting",
      task: "Waiting on you: new dependency",
      sandboxPath: "~/agents-ws/ledger/.sandboxes/ilya",
      harness: "codex",
      startedAt: "2026-10-12T08:01:00Z",
      lastOutputAt: "2026-10-14T09:38:00Z",
      restarts: 0,
    },
    {
      id: "tove@ledger",
      deploymentId: "atlas@ledger",
      agentId: "tove",
      status: "running",
      task: "Idempotency keys for POST /settle",
      sandboxPath: "~/agents-ws/ledger/.sandboxes/tove",
      harness: "claude-code",
      startedAt: "2026-10-12T08:01:00Z",
      lastOutputAt: "2026-10-14T09:44:00Z",
      restarts: 0,
    },
    {
      id: "rhea@ledger",
      deploymentId: "atlas@ledger",
      agentId: "rhea",
      status: "running",
      task: "Property tests for netting",
      sandboxPath: "~/agents-ws/ledger/.sandboxes/rhea",
      harness: "pi",
      startedAt: "2026-10-12T08:01:00Z",
      lastOutputAt: "2026-10-14T09:39:00Z",
      restarts: 0,
    },
    {
      id: "quill@ledger",
      deploymentId: "atlas@ledger",
      agentId: "quill",
      status: "idle",
      task: "Wiki is up to date",
      sandboxPath: "~/agents-ws/ledger/.sandboxes/quill",
      harness: "claude-code",
      startedAt: "2026-10-12T08:01:00Z",
      lastOutputAt: "2026-10-14T09:05:00Z",
      restarts: 0,
    },
    {
      id: "kestrel@ledger",
      deploymentId: "vesper@ledger",
      agentId: "kestrel",
      status: "awaiting",
      task: "Waiting on you: load test on staging",
      sandboxPath: "~/agents-ws/ledger/.sandboxes/kestrel",
      harness: "claude-code",
      startedAt: "2026-10-13T10:01:00Z",
      lastOutputAt: "2026-10-14T09:41:00Z",
      restarts: 0,
    },
    {
      id: "nyx@ledger",
      deploymentId: "vesper@ledger",
      agentId: "nyx",
      status: "running",
      task: "Fuzzing /settle currency codes",
      sandboxPath: "~/agents-ws/ledger/.sandboxes/nyx",
      harness: "hermes",
      startedAt: "2026-10-13T10:01:00Z",
      lastOutputAt: "2026-10-14T09:43:00Z",
      restarts: 0,
    },
    {
      id: "bram@ledger",
      deploymentId: "vesper@ledger",
      agentId: "bram",
      status: "done",
      task: "Handed VSP-9 to Kestrel",
      sandboxPath: "~/agents-ws/ledger/.sandboxes/bram",
      harness: "custom",
      startedAt: "2026-10-13T10:01:00Z",
      lastOutputAt: "2026-10-14T09:12:00Z",
      restarts: 0,
    },
    {
      id: "lumen@relay",
      deploymentId: "orion@relay",
      agentId: "lumen",
      status: "running",
      task: "Drafting ADR-003",
      sandboxPath: "~/agents-ws/relay/.sandboxes/lumen",
      harness: "claude-code",
      startedAt: "2026-10-13T14:01:00Z",
      lastOutputAt: "2026-10-14T09:44:00Z",
      restarts: 0,
    },
    {
      id: "arden@relay",
      deploymentId: "orion@relay",
      agentId: "arden",
      status: "stopped",
      task: "Session exited: rate limit",
      sandboxPath: "~/agents-ws/relay/.sandboxes/arden",
      harness: "codex",
      startedAt: "2026-10-14T09:38:00Z",
      lastOutputAt: "2026-10-14T09:43:00Z",
      restarts: 2,
    },
    {
      id: "pike@relay",
      deploymentId: "orion@relay",
      agentId: "pike",
      status: "idle",
      task: "Waiting for the draft",
      sandboxPath: "~/agents-ws/relay/.sandboxes/pike",
      harness: "pi",
      startedAt: "2026-10-13T14:01:00Z",
      lastOutputAt: "2026-10-14T09:10:00Z",
      restarts: 0,
    },
    {
      id: "kestrel@relay",
      deploymentId: "vesper@relay",
      agentId: "kestrel",
      status: "running",
      task: "Planning attacks on the spike",
      sandboxPath: "~/agents-ws/relay/.sandboxes/kestrel",
      harness: "claude-code",
      startedAt: "2026-10-14T08:31:00Z",
      lastOutputAt: "2026-10-14T09:42:00Z",
      restarts: 0,
    },
    {
      id: "nyx@relay",
      deploymentId: "vesper@relay",
      agentId: "nyx",
      status: "running",
      task: "Oversized message headers",
      sandboxPath: "~/agents-ws/relay/.sandboxes/nyx",
      harness: "hermes",
      startedAt: "2026-10-14T08:31:00Z",
      lastOutputAt: "2026-10-14T09:44:00Z",
      restarts: 0,
    },
    {
      id: "bram@relay",
      deploymentId: "vesper@relay",
      agentId: "bram",
      status: "idle",
      task: "Waiting for a target",
      sandboxPath: "~/agents-ws/relay/.sandboxes/bram",
      harness: "custom",
      startedAt: "2026-10-14T08:31:00Z",
      lastOutputAt: "2026-10-14T08:32:00Z",
      restarts: 0,
    },
  ],

  // Prototype: `actions` (line 1014) and `asks` (line 1016); the reviews come from the
  // pipeline's stage-4 entry p1 (line 1004). invented: ids, times, links, the resolved
  // review of the merged mr-1. Link hrefs: "wiki:<path>" points at the project's wiki
  // (no URL scheme is specified yet).
  requests: [
    {
      id: "r-1",
      kind: "review",
      projectId: "ledger",
      deploymentId: "atlas@ledger",
      instanceId: "mara@ledger",
      summary: "Mara wants to merge LED-39: Schema migration 0016 into main.",
      options: ["Merge to main", "Create PR on Forgejo", "Reject"],
      source: "agent",
      status: "resolved",
      answer: "Merge to main",
      createdAt: "2026-10-13T15:50:00Z",
      resolvedAt: "2026-10-13T16:20:00Z",
      mergeRequestId: "mr-1",
      links: [],
    },
    {
      id: "r-2",
      kind: "review",
      projectId: "ledger",
      deploymentId: "atlas@ledger",
      instanceId: "mara@ledger",
      summary: "Mara wants to merge LED-40: /settle endpoint skeleton into main.",
      options: ["Merge to main", "Create PR on Forgejo", "Reject"],
      source: "agent",
      status: "open",
      answer: "",
      createdAt: "2026-10-14T09:30:00Z",
      resolvedAt: UNSET,
      mergeRequestId: "mr-2",
      links: [{ label: "backlog/LED-40.md", href: "wiki:backlog/LED-40.md" }],
    },
    {
      id: "r-3",
      kind: "approval",
      projectId: "ledger",
      deploymentId: "atlas@ledger",
      instanceId: "ilya@ledger",
      summary: "Ilya wants to add decimal.js as a new dependency for exact money arithmetic.",
      options: ["Approve", "Deny"],
      source: "agent",
      status: "open",
      answer: "",
      createdAt: "2026-10-14T09:38:00Z",
      resolvedAt: UNSET,
      mergeRequestId: "",
      links: [{ label: "backlog/LED-41.md", href: "wiki:backlog/LED-41.md" }],
    },
    {
      id: "r-4",
      kind: "approval",
      projectId: "ledger",
      deploymentId: "vesper@ledger",
      instanceId: "kestrel@ledger",
      summary: "Kestrel wants to run a 10-minute load test (500 rps) against shared staging.",
      options: ["Go ahead", "Hold off"],
      source: "agent",
      status: "open",
      answer: "",
      createdAt: "2026-10-14T09:41:00Z",
      resolvedAt: UNSET,
      mergeRequestId: "",
      links: [],
    },
    {
      id: "r-5",
      kind: "finding",
      projectId: "ledger",
      deploymentId: "vesper@ledger",
      instanceId: "kestrel@ledger",
      summary:
        "Kestrel rates VSP-9 (race in batch close) as high severity: " +
        "a late settle can be counted twice.",
      // The spec's "Owning team fixes now", with the owning team named.
      options: ["Atlas fixes it now", "Add to backlog", "Not an issue"],
      source: "agent",
      status: "open",
      answer: "",
      createdAt: "2026-10-14T09:36:00Z",
      resolvedAt: UNSET,
      mergeRequestId: "",
      links: [{ label: "backlog/VSP-9.md", href: "wiki:backlog/VSP-9.md" }],
    },
    {
      id: "r-6",
      kind: "question",
      projectId: "relay",
      deploymentId: "orion@relay",
      instanceId: "lumen@relay",
      summary:
        "Lumen asks: is a managed broker acceptable, " + "or must the event bus be self-hosted?",
      options: ["Self-hosted only", "Managed is fine"],
      source: "agent",
      status: "open",
      answer: "",
      createdAt: "2026-10-14T09:26:00Z",
      resolvedAt: UNSET,
      mergeRequestId: "",
      links: [{ label: "adr/003-event-bus.md", href: "wiki:adr/003-event-bus.md" }],
    },
    {
      id: "r-7",
      kind: "blocked",
      projectId: "relay",
      deploymentId: "orion@relay",
      instanceId: "arden@relay",
      summary:
        "Arden’s session exited: " + "Codex CLI hit its rate limit three times in ten minutes.",
      // spec (requests.md, "Blocked actions"): Retry now · Retry later · Switch harness;
      // the prototype said "Retry in 30 min" and "Switch to Claude Code".
      options: ["Retry now", "Retry later", "Switch harness"],
      source: "foreman",
      status: "open",
      answer: "",
      createdAt: "2026-10-14T09:43:00Z",
      resolvedAt: UNSET,
      mergeRequestId: "",
      links: [],
    },
  ],

  // Prototype: `acp` (lines 997-1002) and the ⇄ lines of `lines` (lines 980-995).
  // invented: the handoffs behind work items wi-3 and wi-6, Mara's review ask to Ilya,
  // and the delivery times. spec (team-message.md): req-N counts per deployment.
  messages: [
    {
      id: 1,
      deploymentId: "atlas@ledger",
      fromInstance: "ilya@ledger",
      toInstance: "mara@ledger",
      kind: "handoff",
      requestRef: "",
      body: "LED-41 ready: netting in exact decimals, 31 tests.",
      artifactRef: "wt/ilya/LED-41",
      createdAt: "2026-10-14T08:50:00Z",
      deliveredAt: "2026-10-14T08:51:00Z",
    },
    {
      // Queued: Lumen has been busy since; it moves wi-6 to stage 2 once delivered.
      id: 2,
      deploymentId: "orion@relay",
      fromInstance: "arden@relay",
      toInstance: "lumen@relay",
      kind: "handoff",
      requestRef: "",
      body: "JetStream spike: 20k msg/s at p99 7.8ms, 40k at p99 31ms.",
      artifactRef: "wt/arden/RLY-4",
      createdAt: "2026-10-14T09:02:00Z",
      deliveredAt: UNSET,
    },
    {
      id: 3,
      deploymentId: "vesper@ledger",
      fromInstance: "bram@ledger",
      toInstance: "kestrel@ledger",
      kind: "handoff",
      requestRef: "",
      body: "VSP-9 race in batch close, repro attached.",
      artifactRef: "wt/bram/VSP-9",
      createdAt: "2026-10-14T09:12:00Z",
      deliveredAt: "2026-10-14T09:12:00Z",
    },
    {
      id: 4,
      deploymentId: "atlas@ledger",
      fromInstance: "mara@ledger",
      toInstance: "rhea@ledger",
      kind: "ask",
      requestRef: "req-41",
      body: "Property test for netting rounding across EUR, USD, JPY.",
      artifactRef: "",
      createdAt: "2026-10-14T09:20:00Z",
      deliveredAt: "2026-10-14T09:20:00Z",
    },
    {
      id: 5,
      deploymentId: "atlas@ledger",
      fromInstance: "rhea@ledger",
      toInstance: "mara@ledger",
      kind: "reply",
      requestRef: "req-41",
      body: "2 failing seeds.",
      artifactRef: "",
      createdAt: "2026-10-14T09:31:00Z",
      deliveredAt: "2026-10-14T09:31:00Z",
    },
    {
      // spec (team-message.md): a lead's review comments are an ask to the author.
      id: 6,
      deploymentId: "atlas@ledger",
      fromInstance: "mara@ledger",
      toInstance: "ilya@ledger",
      kind: "ask",
      requestRef: "req-42",
      body:
        "2 review comments on LED-41: netting.ts:88 rounds before FX conversion; " +
        "Rhea found 2 failing seeds.",
      artifactRef: "wt/ilya/LED-41",
      createdAt: "2026-10-14T09:34:00Z",
      deliveredAt: "2026-10-14T09:34:00Z",
    },
    {
      id: 7,
      deploymentId: "orion@relay",
      fromInstance: "lumen@relay",
      toInstance: "arden@relay",
      kind: "ask",
      requestRef: "req-7",
      body: "p99 publish latency at 20k msg/s?",
      artifactRef: "",
      createdAt: "2026-10-14T09:40:00Z",
      deliveredAt: "2026-10-14T09:40:00Z",
    },
  ],

  // Prototype: `pipeline` (lines 1003-1009). Its stage 4 (p1, "PR #128") is a merge
  // request here (mr-2), not a work item. invented: wi-1 and wi-2 (the work items the
  // merge requests carry) and the checks results. spec (sandboxes-and-git.md): lead
  // branches are lead/<team>/<topic>, member branches wt/<agent>/<card>.
  workItems: [
    {
      id: "wi-1",
      deploymentId: "atlas@ledger",
      authorInstance: "tove@ledger",
      branch: "wt/tove/LED-39",
      cardRef: "backlog/LED-39.md",
      stage: 3,
      note: "Merged to main",
      checksStatus: "pass",
      mergeRequestId: "mr-1",
    },
    {
      id: "wi-2",
      deploymentId: "atlas@ledger",
      authorInstance: "ilya@ledger",
      branch: "wt/ilya/LED-40",
      cardRef: "backlog/LED-40.md",
      stage: 3,
      note: "Integrated into lead/atlas/integration",
      checksStatus: "pass",
      mergeRequestId: "mr-2",
    },
    {
      id: "wi-3",
      deploymentId: "atlas@ledger",
      authorInstance: "ilya@ledger",
      branch: "wt/ilya/LED-41",
      cardRef: "backlog/LED-41.md",
      stage: 2,
      note: "Mara requested changes",
      checksStatus: "fail",
      mergeRequestId: "",
    },
    {
      id: "wi-4",
      deploymentId: "atlas@ledger",
      authorInstance: "tove@ledger",
      branch: "wt/tove/LED-44",
      cardRef: "backlog/LED-44.md",
      stage: 0,
      note: "In progress",
      checksStatus: "",
      mergeRequestId: "",
    },
    {
      id: "wi-5",
      deploymentId: "vesper@ledger",
      authorInstance: "bram@ledger",
      branch: "wt/bram/VSP-9",
      cardRef: "backlog/VSP-9.md",
      stage: 2,
      note: "With Kestrel",
      checksStatus: "",
      mergeRequestId: "",
    },
    {
      // The prototype's branch was wt/arden/spike, which isn't wt/<agent>/<card>.
      id: "wi-6",
      deploymentId: "orion@relay",
      authorInstance: "arden@relay",
      branch: "wt/arden/RLY-4",
      cardRef: "backlog/RLY-4.md",
      stage: 1,
      note: "Handed off to Lumen",
      checksStatus: "pass",
      mergeRequestId: "",
    },
  ],

  // mr-2 is the prototype's p1 ("PR #128: settlement v2 groundwork"). spec
  // (merge-request.md): nothing reaches the forge until the CEO picks Create PR, so it
  // has no PR yet, and its title is its entry's. invented: mr-1 (merged), the entries.
  mergeRequests: [
    {
      id: "mr-1",
      projectId: "ledger",
      deploymentId: "atlas@ledger",
      authorInstance: "mara@ledger",
      repo: "ledger-api",
      branch: "lead/atlas/schema-0016",
      entry: "backlog/LED-39.md",
      state: "merged",
      checksStatus: "pass",
      conflicting: false,
      forgePrUrl: "",
      forgePrNumber: 0,
      note: "Merged to main",
      createdAt: "2026-10-13T15:50:00Z",
      mergedAt: "2026-10-13T16:20:00Z",
      rejectedAt: UNSET,
    },
    {
      id: "mr-2",
      projectId: "ledger",
      deploymentId: "atlas@ledger",
      authorInstance: "mara@ledger",
      repo: "ledger-api",
      branch: "lead/atlas/integration",
      entry: "backlog/LED-40.md",
      state: "open",
      checksStatus: "pass",
      conflicting: false,
      forgePrUrl: "",
      forgePrNumber: 0,
      note: "Checks pass. Ready for you.",
      createdAt: "2026-10-14T09:30:00Z",
      mergedAt: UNSET,
      rejectedAt: UNSET,
    },
  ],

  // Prototype: wikiFor() (lines 1154-1197): TK (tasks), DESC, docs, FM (frontmatter),
  // CH (changed entries) and boards. Rendered as the server would: headings carry ids,
  // checkboxes carry their source line (data-line), callouts are blockquotes.
  wiki: {
    ledger: {
      tree: [
        {
          path: "index.md",
          title: "ledger",
          folder: false,
          hasIndex: false,
          changed: false,
          children: [],
        },
        {
          path: "adr",
          title: "adr",
          folder: true,
          hasIndex: false,
          changed: false,
          children: [
            {
              path: "adr/013-ledger-ids.md",
              title: "ADR-013: ULIDs for ledger entries",
              folder: false,
              hasIndex: false,
              changed: false,
              children: [],
            },
            {
              path: "adr/014-settlement-v2.md",
              title: "ADR-014: Settlement engine v2",
              folder: false,
              hasIndex: false,
              changed: true,
              children: [],
            },
          ],
        },
        {
          path: "backlog",
          title: "Backlog",
          folder: true,
          hasIndex: true,
          changed: false,
          children: [
            {
              path: "backlog/LED-39.md",
              title: "LED-39: Schema migration 0016",
              folder: false,
              hasIndex: false,
              changed: false,
              children: [],
            },
            {
              path: "backlog/LED-40.md",
              title: "LED-40: /settle endpoint skeleton",
              folder: false,
              hasIndex: false,
              changed: false,
              children: [],
            },
            {
              path: "backlog/LED-41.md",
              title: "LED-41: FX-aware netting",
              folder: false,
              hasIndex: false,
              changed: false,
              children: [],
            },
            {
              path: "backlog/LED-44.md",
              title: "LED-44: Idempotency keys for POST /settle",
              folder: false,
              hasIndex: false,
              changed: true,
              children: [],
            },
            {
              path: "backlog/LED-45.md",
              title: "LED-45: Property tests for netting",
              folder: false,
              hasIndex: false,
              changed: false,
              children: [],
            },
            {
              path: "backlog/LED-47.md",
              title: "LED-47: Settlement report CSV export",
              folder: false,
              hasIndex: false,
              changed: false,
              children: [],
            },
            {
              path: "backlog/LED-48.md",
              title: "LED-48: Multi-currency rounding policy",
              folder: false,
              hasIndex: false,
              changed: false,
              children: [],
            },
            {
              path: "backlog/VSP-11.md",
              title: "VSP-11: 500 on non-ASCII currency code",
              folder: false,
              hasIndex: false,
              changed: false,
              children: [],
            },
            {
              path: "backlog/VSP-12.md",
              title: "VSP-12: Authz bypass via merchant_id swap",
              folder: false,
              hasIndex: false,
              changed: false,
              children: [],
            },
            {
              path: "backlog/VSP-9.md",
              title: "VSP-9: Race: batch close vs late settle",
              folder: false,
              hasIndex: false,
              changed: false,
              children: [],
            },
          ],
        },
        {
          path: "standups",
          title: "standups",
          folder: true,
          hasIndex: false,
          changed: false,
          children: [
            {
              path: "standups/2026-10-14.md",
              title: "Standup, 14 Oct",
              folder: false,
              hasIndex: false,
              changed: true,
              children: [],
            },
          ],
        },
      ],
      entries: {
        // The prototype's team lines read "Atlas (build): …"; spec: teams have no kind.
        // The prototype said "Only leads open PRs to main"; spec: leads request merges.
        "index.md": {
          path: "index.md",
          title: "ledger",
          version: "987a1e995f40d3b0",
          frontmatter: {},
          html:
            "<h1>ledger</h1>\n" +
            "<p>Settlement and reconciliation service. " +
            "This wiki is the shared memory for every team deployed here.</p>\n" +
            '<h2 id="start-here">Start here</h2>\n' +
            "<ul>\n" +
            '<li><a href="adr/014-settlement-v2.md">ADR-014: Settlement engine v2</a></li>\n' +
            '<li><a href="backlog/index.md">Backlog</a></li>\n' +
            '<li><a href="standups/2026-10-14.md">Today’s standup</a></li>\n' +
            "</ul>\n" +
            '<h2 id="teams">Teams</h2>\n' +
            "<ul>\n" +
            "<li>Atlas: Ship settlement engine v2 as specified in ADR-014, " +
            "behind a feature flag.</li>\n" +
            "<li>Vesper: Break settlement v2 before users do.</li>\n" +
            "</ul>\n" +
            '<h2 id="conventions">Conventions</h2>\n' +
            "<ul>\n" +
            "<li>Members work in <code>wt/&lt;agent&gt;/&lt;card&gt;</code>. " +
            "Only leads request merges to main.</li>\n" +
            "<li>Every decision gets an ADR. Every finding gets a task with a repro.</li>\n" +
            "</ul>\n",
          headings: [
            { level: 2, text: "Start here", anchor: "start-here" },
            { level: 2, text: "Teams", anchor: "teams" },
            { level: 2, text: "Conventions", anchor: "conventions" },
          ],
          checkboxes: [],
          backlinks: [],
        },
        "adr/013-ledger-ids.md": {
          path: "adr/013-ledger-ids.md",
          title: "ADR-013: ULIDs for ledger entries",
          version: "17b9cd4c692fb845",
          frontmatter: { type: "adr", status: "accepted" },
          html:
            "<h1>ADR-013: ULIDs for ledger entries</h1>\n" +
            "<p>Entries get sortable ULIDs instead of sequential integers, " +
            "so sandboxes can create them without coordinating.</p>\n",
          headings: [],
          checkboxes: [],
          backlinks: [],
        },
        "adr/014-settlement-v2.md": {
          path: "adr/014-settlement-v2.md",
          title: "ADR-014: Settlement engine v2",
          version: "730cb1eaadddf981",
          frontmatter: { type: "adr", status: "accepted", owner: "mara" },
          html:
            "<h1>ADR-014: Settlement engine v2</h1>\n" +
            '<h2 id="context">Context</h2>\n' +
            "<p>v1 nets in floating point and converts currency after rounding. " +
            "It drifts by up to one minor unit per batch.</p>\n" +
            '<h2 id="decision">Decision</h2>\n' +
            "<ul>\n" +
            "<li>Exact decimal arithmetic on every money path.</li>\n" +
            "<li>Convert currency before rounding. Round once, at the end.</li>\n" +
            "<li>Idempotency keys on <code>POST /settle</code>.</li>\n" +
            "</ul>\n" +
            '<blockquote class="callout" data-callout="warning">\n' +
            "<p><strong>Open question</strong></p>\n" +
            "<p>The rounding policy for zero-decimal currencies like JPY is undecided.</p>\n" +
            "</blockquote>\n" +
            '<p><a href="../backlog/LED-48.md">LED-48: Multi-currency rounding policy</a></p>\n' +
            '<h2 id="consequences">Consequences</h2>\n' +
            "<p>Adds a decimals dependency, which needs your approval. " +
            "Settlement runs about 15% slower in benchmarks.</p>\n" +
            '<h2 id="related">Related</h2>\n' +
            "<ul>\n" +
            '<li><a href="../backlog/LED-41.md">LED-41: FX-aware netting</a></li>\n' +
            '<li><a href="../backlog/VSP-9.md">VSP-9: Race in batch close</a></li>\n' +
            "</ul>\n",
          headings: [
            { level: 2, text: "Context", anchor: "context" },
            { level: 2, text: "Decision", anchor: "decision" },
            { level: 2, text: "Consequences", anchor: "consequences" },
            { level: 2, text: "Related", anchor: "related" },
          ],
          checkboxes: [],
          backlinks: ["index.md"],
        },
        // The prototype linked "Open the board" to "#board", not a wiki entry: dropped.
        "backlog/index.md": {
          path: "backlog/index.md",
          title: "Backlog",
          version: "b72d29baf3e969e3",
          frontmatter: {},
          html:
            "<h1>Backlog</h1>\n" +
            "<p>One file per task. " +
            "The boards declared in wiki.toml are drawn from their frontmatter.</p>\n",
          headings: [],
          checkboxes: [],
          backlinks: ["index.md"],
        },
        // The prototype's standup is dated 12 Oct; moved to the demo's "today".
        "standups/2026-10-14.md": {
          path: "standups/2026-10-14.md",
          title: "Standup, 14 Oct",
          version: "055b10b3c512ba0e",
          frontmatter: { type: "standup", author: "quill" },
          html:
            "<h1>Standup, 14 Oct</h1>\n" +
            '<h2 id="atlas">Atlas</h2>\n' +
            "<ul>\n" +
            "<li>Tove is close on idempotency keys (LED-44).</li>\n" +
            "<li>Mara sent LED-41 back to Ilya over rounding order.</li>\n" +
            "</ul>\n" +
            '<h2 id="vesper">Vesper</h2>\n' +
            "<ul>\n" +
            "<li>Bram reproduced the batch-close race (VSP-9).</li>\n" +
            "<li>Nyx is fuzzing currency codes.</li>\n" +
            "</ul>\n" +
            '<h2 id="waiting-on-the-ceo">Waiting on the CEO</h2>\n' +
            "<ul>\n" +
            '<li><input type="checkbox" disabled data-line="20"> ' +
            "decimal.js dependency for LED-41</li>\n" +
            '<li><input type="checkbox" disabled data-line="21"> ' +
            "Load test on shared staging</li>\n" +
            "</ul>\n",
          headings: [
            { level: 2, text: "Atlas", anchor: "atlas" },
            { level: 2, text: "Vesper", anchor: "vesper" },
            { level: 2, text: "Waiting on the CEO", anchor: "waiting-on-the-ceo" },
          ],
          checkboxes: [
            { line: 20, done: false, text: "decimal.js dependency for LED-41" },
            { line: 21, done: false, text: "Load test on shared staging" },
          ],
          backlinks: ["index.md"],
        },
        // Task entries: 7 frontmatter lines, the H1 on line 9, an optional description on
        // line 11, then "## Checklist" and its three boxes.
        "backlog/LED-39.md": {
          path: "backlog/LED-39.md",
          title: "LED-39: Schema migration 0016",
          version: "69251729c42251bb",
          frontmatter: {
            type: "task",
            kind: "feature",
            status: "done",
            priority: "medium",
            assignee: "tove",
          },
          html:
            "<h1>LED-39: Schema migration 0016</h1>\n" +
            '<h2 id="checklist">Checklist</h2>\n' +
            "<ul>\n" +
            '<li><input type="checkbox" checked disabled data-line="13"> Worktree created</li>\n' +
            '<li><input type="checkbox" checked disabled data-line="14"> Checks pass</li>\n' +
            '<li><input type="checkbox" checked disabled data-line="15"> ' +
            "Reviewed by the lead</li>\n" +
            "</ul>\n",
          headings: [{ level: 2, text: "Checklist", anchor: "checklist" }],
          checkboxes: [
            { line: 13, done: true, text: "Worktree created" },
            { line: 14, done: true, text: "Checks pass" },
            { line: 15, done: true, text: "Reviewed by the lead" },
          ],
          backlinks: [],
        },
        "backlog/LED-40.md": {
          path: "backlog/LED-40.md",
          title: "LED-40: /settle endpoint skeleton",
          version: "969e93ba80d19a3b",
          frontmatter: {
            type: "task",
            kind: "feature",
            status: "done",
            priority: "high",
            assignee: "ilya",
          },
          html:
            "<h1>LED-40: /settle endpoint skeleton</h1>\n" +
            '<h2 id="checklist">Checklist</h2>\n' +
            "<ul>\n" +
            '<li><input type="checkbox" checked disabled data-line="13"> Worktree created</li>\n' +
            '<li><input type="checkbox" checked disabled data-line="14"> Checks pass</li>\n' +
            '<li><input type="checkbox" checked disabled data-line="15"> ' +
            "Reviewed by the lead</li>\n" +
            "</ul>\n",
          headings: [{ level: 2, text: "Checklist", anchor: "checklist" }],
          checkboxes: [
            { line: 13, done: true, text: "Worktree created" },
            { line: 14, done: true, text: "Checks pass" },
            { line: 15, done: true, text: "Reviewed by the lead" },
          ],
          backlinks: [],
        },
        "backlog/LED-41.md": {
          path: "backlog/LED-41.md",
          title: "LED-41: FX-aware netting",
          version: "d65d36ef029067bb",
          frontmatter: {
            type: "task",
            kind: "feature",
            status: "review",
            priority: "high",
            assignee: "ilya",
          },
          html:
            "<h1>LED-41: FX-aware netting</h1>\n" +
            "<p>Net positions in exact decimals and convert currency before rounding. " +
            "Mara requested changes: rounding still happens before FX conversion " +
            "in netting.ts.</p>\n" +
            '<h2 id="checklist">Checklist</h2>\n' +
            "<ul>\n" +
            '<li><input type="checkbox" checked disabled data-line="15"> Worktree created</li>\n' +
            '<li><input type="checkbox" checked disabled data-line="16"> Checks pass</li>\n' +
            '<li><input type="checkbox" disabled data-line="17"> Reviewed by the lead</li>\n' +
            "</ul>\n",
          headings: [{ level: 2, text: "Checklist", anchor: "checklist" }],
          checkboxes: [
            { line: 15, done: true, text: "Worktree created" },
            { line: 16, done: true, text: "Checks pass" },
            { line: 17, done: false, text: "Reviewed by the lead" },
          ],
          backlinks: ["adr/014-settlement-v2.md"],
        },
        "backlog/LED-44.md": {
          path: "backlog/LED-44.md",
          title: "LED-44: Idempotency keys for POST /settle",
          version: "6cf593277bb9be5a",
          frontmatter: {
            type: "task",
            kind: "feature",
            status: "in-progress",
            priority: "high",
            assignee: "tove",
          },
          html:
            "<h1>LED-44: Idempotency keys for POST /settle</h1>\n" +
            "<p>Accept an Idempotency-Key header on POST /settle. " +
            "Keys are kept 24 hours, scoped per merchant.</p>\n" +
            '<h2 id="checklist">Checklist</h2>\n' +
            "<ul>\n" +
            '<li><input type="checkbox" checked disabled data-line="15"> Worktree created</li>\n' +
            '<li><input type="checkbox" disabled data-line="16"> Checks pass</li>\n' +
            '<li><input type="checkbox" disabled data-line="17"> Reviewed by the lead</li>\n' +
            "</ul>\n",
          headings: [{ level: 2, text: "Checklist", anchor: "checklist" }],
          checkboxes: [
            { line: 15, done: true, text: "Worktree created" },
            { line: 16, done: false, text: "Checks pass" },
            { line: 17, done: false, text: "Reviewed by the lead" },
          ],
          backlinks: [],
        },
        "backlog/LED-45.md": {
          path: "backlog/LED-45.md",
          title: "LED-45: Property tests for netting",
          version: "31f9698dbdae5191",
          frontmatter: {
            type: "task",
            kind: "feature",
            status: "in-progress",
            priority: "medium",
            assignee: "rhea",
          },
          html:
            "<h1>LED-45: Property tests for netting</h1>\n" +
            '<h2 id="checklist">Checklist</h2>\n' +
            "<ul>\n" +
            '<li><input type="checkbox" checked disabled data-line="13"> Worktree created</li>\n' +
            '<li><input type="checkbox" disabled data-line="14"> Checks pass</li>\n' +
            '<li><input type="checkbox" disabled data-line="15"> Reviewed by the lead</li>\n' +
            "</ul>\n",
          headings: [{ level: 2, text: "Checklist", anchor: "checklist" }],
          checkboxes: [
            { line: 13, done: true, text: "Worktree created" },
            { line: 14, done: false, text: "Checks pass" },
            { line: 15, done: false, text: "Reviewed by the lead" },
          ],
          backlinks: [],
        },
        "backlog/LED-47.md": {
          path: "backlog/LED-47.md",
          title: "LED-47: Settlement report CSV export",
          version: "90ff503c2cff9489",
          frontmatter: {
            type: "task",
            kind: "feature",
            status: "todo",
            priority: "low",
            assignee: "tove",
          },
          html:
            "<h1>LED-47: Settlement report CSV export</h1>\n" +
            '<h2 id="checklist">Checklist</h2>\n' +
            "<ul>\n" +
            '<li><input type="checkbox" disabled data-line="13"> Worktree created</li>\n' +
            '<li><input type="checkbox" disabled data-line="14"> Checks pass</li>\n' +
            '<li><input type="checkbox" disabled data-line="15"> Reviewed by the lead</li>\n' +
            "</ul>\n",
          headings: [{ level: 2, text: "Checklist", anchor: "checklist" }],
          checkboxes: [
            { line: 13, done: false, text: "Worktree created" },
            { line: 14, done: false, text: "Checks pass" },
            { line: 15, done: false, text: "Reviewed by the lead" },
          ],
          backlinks: [],
        },
        "backlog/LED-48.md": {
          path: "backlog/LED-48.md",
          title: "LED-48: Multi-currency rounding policy",
          version: "28813a0853da2b8c",
          frontmatter: {
            type: "task",
            kind: "feature",
            status: "todo",
            priority: "medium",
            assignee: "quill",
          },
          html:
            "<h1>LED-48: Multi-currency rounding policy</h1>\n" +
            '<h2 id="checklist">Checklist</h2>\n' +
            "<ul>\n" +
            '<li><input type="checkbox" disabled data-line="13"> Worktree created</li>\n' +
            '<li><input type="checkbox" disabled data-line="14"> Checks pass</li>\n' +
            '<li><input type="checkbox" disabled data-line="15"> Reviewed by the lead</li>\n' +
            "</ul>\n",
          headings: [{ level: 2, text: "Checklist", anchor: "checklist" }],
          checkboxes: [
            { line: 13, done: false, text: "Worktree created" },
            { line: 14, done: false, text: "Checks pass" },
            { line: 15, done: false, text: "Reviewed by the lead" },
          ],
          backlinks: ["adr/014-settlement-v2.md"],
        },
        "backlog/VSP-9.md": {
          path: "backlog/VSP-9.md",
          title: "VSP-9: Race: batch close vs late settle",
          version: "eba334b27801a783",
          frontmatter: {
            type: "task",
            kind: "finding",
            status: "review",
            priority: "high",
            assignee: "mara",
          },
          html:
            "<h1>VSP-9: Race: batch close vs late settle</h1>\n" +
            "<p>A late settle can land after the batch closes and be counted twice. " +
            "Bram reproduced it 37 times in 200 runs.</p>\n" +
            '<h2 id="checklist">Checklist</h2>\n' +
            "<ul>\n" +
            '<li><input type="checkbox" checked disabled data-line="15"> Worktree created</li>\n' +
            '<li><input type="checkbox" checked disabled data-line="16"> Checks pass</li>\n' +
            '<li><input type="checkbox" disabled data-line="17"> Reviewed by the lead</li>\n' +
            "</ul>\n",
          headings: [{ level: 2, text: "Checklist", anchor: "checklist" }],
          checkboxes: [
            { line: 15, done: true, text: "Worktree created" },
            { line: 16, done: true, text: "Checks pass" },
            { line: 17, done: false, text: "Reviewed by the lead" },
          ],
          backlinks: ["adr/014-settlement-v2.md"],
        },
        "backlog/VSP-11.md": {
          path: "backlog/VSP-11.md",
          title: "VSP-11: 500 on non-ASCII currency code",
          version: "5bf86b2d04cdd62a",
          frontmatter: {
            type: "task",
            kind: "finding",
            status: "in-progress",
            priority: "medium",
            assignee: "nyx",
          },
          html:
            "<h1>VSP-11: 500 on non-ASCII currency code</h1>\n" +
            '<h2 id="checklist">Checklist</h2>\n' +
            "<ul>\n" +
            '<li><input type="checkbox" checked disabled data-line="13"> Worktree created</li>\n' +
            '<li><input type="checkbox" disabled data-line="14"> Checks pass</li>\n' +
            '<li><input type="checkbox" disabled data-line="15"> Reviewed by the lead</li>\n' +
            "</ul>\n",
          headings: [{ level: 2, text: "Checklist", anchor: "checklist" }],
          checkboxes: [
            { line: 13, done: true, text: "Worktree created" },
            { line: 14, done: false, text: "Checks pass" },
            { line: 15, done: false, text: "Reviewed by the lead" },
          ],
          backlinks: [],
        },
        "backlog/VSP-12.md": {
          path: "backlog/VSP-12.md",
          title: "VSP-12: Authz bypass via merchant_id swap",
          version: "a4049bd7828039ef",
          frontmatter: {
            type: "task",
            kind: "finding",
            status: "todo",
            priority: "high",
            assignee: "nyx",
          },
          html:
            "<h1>VSP-12: Authz bypass via merchant_id swap</h1>\n" +
            "<p>Swapping merchant_id in the body may let one merchant settle against " +
            "another’s balance. Not confirmed yet.</p>\n" +
            '<h2 id="checklist">Checklist</h2>\n' +
            "<ul>\n" +
            '<li><input type="checkbox" disabled data-line="15"> Worktree created</li>\n' +
            '<li><input type="checkbox" disabled data-line="16"> Checks pass</li>\n' +
            '<li><input type="checkbox" disabled data-line="17"> Reviewed by the lead</li>\n' +
            "</ul>\n",
          headings: [{ level: 2, text: "Checklist", anchor: "checklist" }],
          checkboxes: [
            { line: 15, done: false, text: "Worktree created" },
            { line: 16, done: false, text: "Checks pass" },
            { line: 17, done: false, text: "Reviewed by the lead" },
          ],
          backlinks: [],
        },
      },
      // Card titles are the task titles without the id, which the card shows apart.
      boards: [
        {
          id: "sprint",
          name: "Sprint 07",
          columns: ["todo", "in-progress", "review", "done"],
          laneField: "priority",
          lanes: ["high", "medium", "low"],
          cards: [
            {
              path: "backlog/LED-39.md",
              id: "LED-39",
              title: "Schema migration 0016",
              column: "done",
              lane: "medium",
              priority: "medium",
              assignee: "tove",
              version: "69251729c42251bb",
            },
            {
              path: "backlog/LED-40.md",
              id: "LED-40",
              title: "/settle endpoint skeleton",
              column: "done",
              lane: "high",
              priority: "high",
              assignee: "ilya",
              version: "969e93ba80d19a3b",
            },
            {
              path: "backlog/LED-41.md",
              id: "LED-41",
              title: "FX-aware netting",
              column: "review",
              lane: "high",
              priority: "high",
              assignee: "ilya",
              version: "d65d36ef029067bb",
            },
            {
              path: "backlog/LED-44.md",
              id: "LED-44",
              title: "Idempotency keys for POST /settle",
              column: "in-progress",
              lane: "high",
              priority: "high",
              assignee: "tove",
              version: "6cf593277bb9be5a",
            },
            {
              path: "backlog/LED-45.md",
              id: "LED-45",
              title: "Property tests for netting",
              column: "in-progress",
              lane: "medium",
              priority: "medium",
              assignee: "rhea",
              version: "31f9698dbdae5191",
            },
            {
              path: "backlog/LED-47.md",
              id: "LED-47",
              title: "Settlement report CSV export",
              column: "todo",
              lane: "low",
              priority: "low",
              assignee: "tove",
              version: "90ff503c2cff9489",
            },
            {
              path: "backlog/LED-48.md",
              id: "LED-48",
              title: "Multi-currency rounding policy",
              column: "todo",
              lane: "medium",
              priority: "medium",
              assignee: "quill",
              version: "28813a0853da2b8c",
            },
          ],
          // spec (board.md): a board shows exactly what its `where` selects, so the sprint
          // board filters kind=feature (the prototype's had no filter and still showed
          // features only). Columns and lanes declared, as in the board.md example.
          declaration: `[[tool.wikiview.board]]
id      = "sprint"
path    = "/backlog"
name    = "Sprint 07"
where   = ["type=task", "kind=feature"]
columns = ["todo", "in-progress", "review", "done"]
lane    = "priority"
lanes   = ["high", "medium", "low"]`,
          problems: [],
        },
        {
          id: "findings",
          name: "Findings",
          columns: ["todo", "in-progress", "review", "done"],
          laneField: "",
          lanes: [],
          cards: [
            {
              path: "backlog/VSP-9.md",
              id: "VSP-9",
              title: "Race: batch close vs late settle",
              column: "review",
              lane: "",
              priority: "high",
              assignee: "mara",
              version: "eba334b27801a783",
            },
            {
              path: "backlog/VSP-11.md",
              id: "VSP-11",
              title: "500 on non-ASCII currency code",
              column: "in-progress",
              lane: "",
              priority: "medium",
              assignee: "nyx",
              version: "5bf86b2d04cdd62a",
            },
            {
              path: "backlog/VSP-12.md",
              id: "VSP-12",
              title: "Authz bypass via merchant_id swap",
              column: "todo",
              lane: "",
              priority: "high",
              assignee: "nyx",
              version: "a4049bd7828039ef",
            },
          ],
          declaration: `[[tool.wikiview.board]]
id      = "findings"
path    = "/backlog"
name    = "Findings"
where   = ["type=task", "kind=finding"]
columns = ["todo", "in-progress", "review", "done"]`,
          problems: [],
        },
      ],
    },

    relay: {
      tree: [
        {
          path: "index.md",
          title: "relay",
          folder: false,
          hasIndex: false,
          changed: false,
          children: [],
        },
        {
          path: "adr",
          title: "adr",
          folder: true,
          hasIndex: false,
          changed: false,
          children: [
            {
              path: "adr/003-event-bus.md",
              title: "ADR-003: Event bus topology",
              folder: false,
              hasIndex: false,
              changed: true,
              children: [],
            },
          ],
        },
        {
          path: "backlog",
          title: "Backlog",
          folder: true,
          hasIndex: true,
          changed: false,
          children: [
            {
              path: "backlog/RLY-1.md",
              title: "RLY-1: Requirements doc",
              folder: false,
              hasIndex: false,
              changed: false,
              children: [],
            },
            {
              path: "backlog/RLY-3.md",
              title: "RLY-3: ADR-003: event bus topology",
              folder: false,
              hasIndex: false,
              changed: false,
              children: [],
            },
            {
              path: "backlog/RLY-4.md",
              title: "RLY-4: JetStream throughput spike",
              folder: false,
              hasIndex: false,
              changed: false,
              children: [],
            },
            {
              path: "backlog/RLY-6.md",
              title: "RLY-6: Dead-letter strategy",
              folder: false,
              hasIndex: false,
              changed: false,
              children: [],
            },
            {
              path: "backlog/RLY-8.md",
              title: "RLY-8: Broker crash on 1MB header",
              folder: false,
              hasIndex: false,
              changed: true,
              children: [],
            },
          ],
        },
      ],
      entries: {
        "index.md": {
          path: "index.md",
          title: "relay",
          version: "4be15aa3c5554ad9",
          frontmatter: {},
          html:
            "<h1>relay</h1>\n" +
            "<p>Event routing between internal services.</p>\n" +
            '<h2 id="start-here">Start here</h2>\n' +
            "<ul>\n" +
            '<li><a href="adr/003-event-bus.md">ADR-003: Event bus topology</a></li>\n' +
            '<li><a href="backlog/index.md">Backlog</a></li>\n' +
            "</ul>\n" +
            '<h2 id="teams">Teams</h2>\n' +
            "<ul>\n" +
            "<li>Orion: Propose the event-bus topology for relay v1.</li>\n" +
            "<li>Vesper: Stress the JetStream spike: oversized messages, partitions, " +
            "slow consumers.</li>\n" +
            "</ul>\n",
          headings: [
            { level: 2, text: "Start here", anchor: "start-here" },
            { level: 2, text: "Teams", anchor: "teams" },
          ],
          checkboxes: [],
          backlinks: [],
        },
        // The callout promises a "Consequences" section the prototype's draft doesn't
        // have yet; kept, since the ADR is a draft.
        "adr/003-event-bus.md": {
          path: "adr/003-event-bus.md",
          title: "ADR-003: Event bus topology",
          version: "a113da785fa0f8c7",
          frontmatter: { type: "adr", status: "draft", owner: "lumen" },
          html:
            "<h1>ADR-003: Event bus topology</h1>\n" +
            '<blockquote class="callout" data-callout="info">\n' +
            "<p><strong>Draft</strong></p>\n" +
            "<p>Lumen is writing this. Arden’s spike numbers go under Consequences.</p>\n" +
            "</blockquote>\n" +
            '<h2 id="context">Context</h2>\n' +
            "<p>Ordering must hold per account, not globally. " +
            "Peak load is 20k messages per second.</p>\n" +
            '<h2 id="options">Options</h2>\n' +
            "<ul>\n" +
            "<li>Kafka</li>\n" +
            "<li>NATS JetStream</li>\n" +
            "<li>Redpanda</li>\n" +
            "</ul>\n",
          headings: [
            { level: 2, text: "Context", anchor: "context" },
            { level: 2, text: "Options", anchor: "options" },
          ],
          checkboxes: [],
          backlinks: ["index.md"],
        },
        "backlog/index.md": {
          path: "backlog/index.md",
          title: "Backlog",
          version: "0a1c6f738158a607",
          frontmatter: {},
          html: "<h1>Backlog</h1>\n<p>One file per task.</p>\n",
          headings: [],
          checkboxes: [],
          backlinks: ["index.md"],
        },
        "backlog/RLY-1.md": {
          path: "backlog/RLY-1.md",
          title: "RLY-1: Requirements doc",
          version: "fb97b95b842a6a3a",
          frontmatter: {
            type: "task",
            kind: "feature",
            status: "done",
            priority: "medium",
            assignee: "pike",
          },
          html:
            "<h1>RLY-1: Requirements doc</h1>\n" +
            '<h2 id="checklist">Checklist</h2>\n' +
            "<ul>\n" +
            '<li><input type="checkbox" checked disabled data-line="13"> Worktree created</li>\n' +
            '<li><input type="checkbox" checked disabled data-line="14"> Checks pass</li>\n' +
            '<li><input type="checkbox" checked disabled data-line="15"> ' +
            "Reviewed by the lead</li>\n" +
            "</ul>\n",
          headings: [{ level: 2, text: "Checklist", anchor: "checklist" }],
          checkboxes: [
            { line: 13, done: true, text: "Worktree created" },
            { line: 14, done: true, text: "Checks pass" },
            { line: 15, done: true, text: "Reviewed by the lead" },
          ],
          backlinks: [],
        },
        "backlog/RLY-3.md": {
          path: "backlog/RLY-3.md",
          title: "RLY-3: ADR-003: event bus topology",
          version: "f48fd34fbdab9dc3",
          frontmatter: {
            type: "task",
            kind: "feature",
            status: "in-progress",
            priority: "high",
            assignee: "lumen",
          },
          html:
            "<h1>RLY-3: ADR-003: event bus topology</h1>\n" +
            '<h2 id="checklist">Checklist</h2>\n' +
            "<ul>\n" +
            '<li><input type="checkbox" checked disabled data-line="13"> Worktree created</li>\n' +
            '<li><input type="checkbox" disabled data-line="14"> Checks pass</li>\n' +
            '<li><input type="checkbox" disabled data-line="15"> Reviewed by the lead</li>\n' +
            "</ul>\n",
          headings: [{ level: 2, text: "Checklist", anchor: "checklist" }],
          checkboxes: [
            { line: 13, done: true, text: "Worktree created" },
            { line: 14, done: false, text: "Checks pass" },
            { line: 15, done: false, text: "Reviewed by the lead" },
          ],
          backlinks: [],
        },
        "backlog/RLY-4.md": {
          path: "backlog/RLY-4.md",
          title: "RLY-4: JetStream throughput spike",
          version: "cccf25380ad408be",
          frontmatter: {
            type: "task",
            kind: "feature",
            status: "in-progress",
            priority: "high",
            assignee: "arden",
          },
          html:
            "<h1>RLY-4: JetStream throughput spike</h1>\n" +
            '<h2 id="checklist">Checklist</h2>\n' +
            "<ul>\n" +
            '<li><input type="checkbox" checked disabled data-line="13"> Worktree created</li>\n' +
            '<li><input type="checkbox" disabled data-line="14"> Checks pass</li>\n' +
            '<li><input type="checkbox" disabled data-line="15"> Reviewed by the lead</li>\n' +
            "</ul>\n",
          headings: [{ level: 2, text: "Checklist", anchor: "checklist" }],
          checkboxes: [
            { line: 13, done: true, text: "Worktree created" },
            { line: 14, done: false, text: "Checks pass" },
            { line: 15, done: false, text: "Reviewed by the lead" },
          ],
          backlinks: [],
        },
        "backlog/RLY-6.md": {
          path: "backlog/RLY-6.md",
          title: "RLY-6: Dead-letter strategy",
          version: "c93b928e1dc768ed",
          frontmatter: {
            type: "task",
            kind: "feature",
            status: "todo",
            priority: "low",
            assignee: "lumen",
          },
          html:
            "<h1>RLY-6: Dead-letter strategy</h1>\n" +
            '<h2 id="checklist">Checklist</h2>\n' +
            "<ul>\n" +
            '<li><input type="checkbox" disabled data-line="13"> Worktree created</li>\n' +
            '<li><input type="checkbox" disabled data-line="14"> Checks pass</li>\n' +
            '<li><input type="checkbox" disabled data-line="15"> Reviewed by the lead</li>\n' +
            "</ul>\n",
          headings: [{ level: 2, text: "Checklist", anchor: "checklist" }],
          checkboxes: [
            { line: 13, done: false, text: "Worktree created" },
            { line: 14, done: false, text: "Checks pass" },
            { line: 15, done: false, text: "Reviewed by the lead" },
          ],
          backlinks: [],
        },
        "backlog/RLY-8.md": {
          path: "backlog/RLY-8.md",
          title: "RLY-8: Broker crash on 1MB header",
          version: "a85119de12a63734",
          frontmatter: {
            type: "task",
            kind: "finding",
            status: "todo",
            priority: "high",
            assignee: "nyx",
          },
          html:
            "<h1>RLY-8: Broker crash on 1MB header</h1>\n" +
            "<p>A message with a 1MB header crashes the broker. " +
            "Waiting on your call about reporting upstream.</p>\n" +
            '<h2 id="checklist">Checklist</h2>\n' +
            "<ul>\n" +
            '<li><input type="checkbox" disabled data-line="15"> Worktree created</li>\n' +
            '<li><input type="checkbox" disabled data-line="16"> Checks pass</li>\n' +
            '<li><input type="checkbox" disabled data-line="17"> Reviewed by the lead</li>\n' +
            "</ul>\n",
          headings: [{ level: 2, text: "Checklist", anchor: "checklist" }],
          checkboxes: [
            { line: 15, done: false, text: "Worktree created" },
            { line: 16, done: false, text: "Checks pass" },
            { line: 17, done: false, text: "Reviewed by the lead" },
          ],
          backlinks: [],
        },
      },
      boards: [
        {
          id: "adr",
          name: "ADR track",
          columns: ["todo", "in-progress", "review", "done"],
          laneField: "",
          lanes: [],
          cards: [
            {
              path: "backlog/RLY-1.md",
              id: "RLY-1",
              title: "Requirements doc",
              column: "done",
              lane: "",
              priority: "medium",
              assignee: "pike",
              version: "fb97b95b842a6a3a",
            },
            {
              path: "backlog/RLY-3.md",
              id: "RLY-3",
              title: "ADR-003: event bus topology",
              column: "in-progress",
              lane: "",
              priority: "high",
              assignee: "lumen",
              version: "f48fd34fbdab9dc3",
            },
            {
              path: "backlog/RLY-4.md",
              id: "RLY-4",
              title: "JetStream throughput spike",
              column: "in-progress",
              lane: "",
              priority: "high",
              assignee: "arden",
              version: "cccf25380ad408be",
            },
            {
              path: "backlog/RLY-6.md",
              id: "RLY-6",
              title: "Dead-letter strategy",
              column: "todo",
              lane: "",
              priority: "low",
              assignee: "lumen",
              version: "c93b928e1dc768ed",
            },
            {
              path: "backlog/RLY-8.md",
              id: "RLY-8",
              title: "Broker crash on 1MB header",
              column: "todo",
              lane: "",
              priority: "high",
              assignee: "nyx",
              version: "a85119de12a63734",
            },
          ],
          declaration: `[[tool.wikiview.board]]
id      = "adr"
path    = "/backlog"
name    = "ADR track"
columns = ["todo", "in-progress", "review", "done"]`,
          problems: [],
        },
      ],
    },

    harbor: {
      tree: [
        {
          path: "index.md",
          title: "harbor",
          folder: false,
          hasIndex: false,
          changed: false,
          children: [],
        },
        {
          path: "backlog",
          title: "Backlog",
          folder: true,
          hasIndex: true,
          changed: false,
          children: [],
        },
      ],
      entries: {
        // spec (deploy-team.md): Deploy opens from Project settings, the Team page or
        // the Chief of Staff; the prototype said "from the Sessions tab".
        "index.md": {
          path: "index.md",
          title: "harbor",
          version: "64f295a75b9bf885",
          frontmatter: {},
          html:
            "<h1>harbor</h1>\n" +
            "<p>Infrastructure as code for the fleet.</p>\n" +
            '<blockquote class="callout" data-callout="info">\n' +
            "<p><strong>Idle</strong></p>\n" +
            "<p>No team is deployed here yet. Deploy one from Project settings.</p>\n" +
            "</blockquote>\n" +
            '<h2 id="teams">Teams</h2>\n' +
            "<p>No team is deployed here yet.</p>\n",
          headings: [{ level: 2, text: "Teams", anchor: "teams" }],
          checkboxes: [],
          backlinks: [],
        },
        "backlog/index.md": {
          path: "backlog/index.md",
          title: "Backlog",
          version: "775f3b47c52db6a1",
          frontmatter: {},
          html: "<h1>Backlog</h1>\n<p>Empty for now.</p>\n",
          headings: [],
          checkboxes: [],
          backlinks: [],
        },
      },
      boards: [
        {
          id: "backlog",
          name: "Backlog",
          columns: ["todo", "in-progress", "review", "done"],
          laneField: "",
          lanes: [],
          cards: [],
          declaration: `[[tool.wikiview.board]]
id      = "backlog"
path    = "/backlog"
name    = "Backlog"
columns = ["todo", "in-progress", "review", "done"]`,
          problems: [],
        },
      ],
    },
  },
};
