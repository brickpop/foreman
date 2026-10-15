import { afterEach, describe, expect, it, vi } from "vitest";
import type { ForemanClient } from "../client";
import { demoClient } from "./demo";
import { lineDiff } from "./files";
import { seed } from "./seed";

// The scripted Chief of Staff (wiki/product/features/chief-of-staff.md#flows).

afterEach(() => vi.useRealTimers());

function client() {
  vi.useFakeTimers();
  return demoClient(seed, { tick: 0, replyMs: 10 });
}

/** The CEO says something; the Chief of Staff answers. */
async function say(c: ForemanClient, text: string) {
  await c.chiefOfStaff.send(text);
  expect((await c.chiefOfStaff.conversation()).working).toBe(true);
  vi.advanceTimersByTime(10);
  return c.chiefOfStaff.conversation();
}

const last = async (c: ForemanClient) => (await c.chiefOfStaff.conversation()).items.at(-1);

describe("the demo Chief of Staff", () => {
  it("greets on the first open only", async () => {
    const c = client();
    await c.chiefOfStaff.start({ flow: "open" });
    await c.chiefOfStaff.start({ flow: "open" });
    const talk = await c.chiefOfStaff.conversation();
    expect(talk.items).toHaveLength(1);
    expect(talk.choices).toEqual(["Hire for a role", "Create a team", "What needs me today?"]);
  });

  it("hires from a team: a job panel, then a review; merging adds the agent", async () => {
    const c = client();
    await c.chiefOfStaff.start({ flow: "hire", team: "atlas" });
    expect((await last(c))?.text).toBe("Hiring for Atlas. What role?");
    await say(c, "QA");
    expect((await c.chiefOfStaff.conversation()).choices).toEqual([
      "Property tests and fuzzing",
      "Load and concurrency",
    ]);
    await say(c, "Property tests and fuzzing");
    await say(c, "Pushes back, challenges decisions");
    const talk = await say(c, "Claude Code");
    const panel = talk.items.at(-1)?.panel;
    expect(panel?.title).toMatch(/^\w+ · QA · Atlas$/);
    expect(panel?.rows.find(([k]) => k === "Job")?.[1]).toContain(
      "Focus: Property tests and fuzzing.",
    );
    const name = panel?.title.split(" · ")[0] ?? "";
    expect(talk.choices).toEqual([`Hire ${name}`, "Start over"]);

    const done = await say(c, `Hire ${name}`);
    expect(done.items.at(-1)?.text).toBe(
      `${name} is ready to join Atlas: it’s waiting for you as a review.`,
    );
    const review = (await c.requests.list()).find((r) => r.source === "cos");
    expect(review).toMatchObject({
      kind: "review",
      projectId: "",
      options: ["Merge to main", "Reject"],
    });

    await c.pipeline.merge(review?.mergeRequestId ?? "");
    const atlas = (await c.workspace.teams()).find((t) => t.id === "atlas");
    expect(atlas?.config.agents[name.toLowerCase()]).toMatchObject({ name, role: "qa" });
    expect(atlas?.source).toContain(`name = "${name}"`);
  });

  it("asks again about a team it doesn't know, and a harness it doesn't know", async () => {
    const c = client();
    await c.chiefOfStaff.start({ flow: "hire" });
    let talk = await say(c, "Nonexistent");
    expect(talk.items.at(-1)?.text).toBe(
      "I don’t know a team called “Nonexistent”. Which team is this hire for?",
    );
    await say(c, "atlas");
    await say(c, "Engineer");
    await say(c, "Backend APIs and migrations");
    await say(c, "Collaborative, checks in often");
    talk = await say(c, "Emacs");
    expect(talk.items.at(-1)?.text).toContain("I don’t know the harness “Emacs”");
    talk = await say(c, "Custom");
    expect(talk.items.at(-1)?.text).toBe("What command starts it?");
    talk = await say(c, "my-agent --fast");
    expect(talk.items.at(-1)?.panel?.rows).toContainEqual(["Harness", "Custom · my-agent --fast"]);
  });

  it("confirms only on the exact Hire: a taken name is refused, other text changes the job", async () => {
    const c = client();
    await c.chiefOfStaff.start({ flow: "hire", team: "atlas" });
    for (const line of [
      "Engineer",
      "Backend APIs and migrations",
      "Heads-down, reports when done",
      "Claude Code",
    ])
      await say(c, line);
    let talk = await say(c, "Mara");
    expect(talk.items.at(-1)?.text).toBe(
      "Mara is already taken in this workspace. What else should we call them?",
    );
    talk = await say(c, "Zed");
    expect(talk.items.at(-1)?.panel?.title).toBe("Zed · Engineer · Atlas");
    talk = await say(c, "Hire them");
    expect(talk.items.at(-1)?.panel?.rows.find(([k]) => k === "Job")?.[1]).toContain(
      "Adjustment: Hire them.",
    );
    expect((await c.requests.list()).some((r) => r.source === "cos")).toBe(false);
  });

  it("creates a team: a draft, changes, and the team once merged", async () => {
    const c = client();
    await c.chiefOfStaff.start({ flow: "create-team" });
    let talk = await say(c, "Build the payouts features, with tests and reviews");
    expect(talk.choices).not.toContain("Atlas");
    talk = await say(c, "Atlas");
    expect(talk.items.at(-1)?.text).toContain("Atlas is already taken");
    talk = await say(c, "Payouts");
    expect(talk.choices).toEqual(["Create Payouts", "Change something", "Start over"]);
    await say(c, "Change something");
    talk = await say(c, "Add an architect");
    expect(talk.items.at(-1)?.panel?.rows).toContainEqual(["Change", "Add an architect"]);
    await say(c, "Create Payouts");
    const review = (await c.requests.list()).find((r) => r.source === "cos");
    await c.pipeline.merge(review?.mergeRequestId ?? "");
    const payouts = (await c.workspace.teams()).find((t) => t.config.name === "Payouts");
    // The merged file has the roster the panel showed, the lead first.
    const shown =
      talk.items
        .at(-1)
        ?.panel?.rows.filter(([k]) =>
          ["Team Lead", "Engineer", "QA / Tester", "Reviewer"].includes(k),
        ) ?? [];
    expect(shown).toHaveLength(4);
    expect(Object.values(payouts?.config.agents ?? {}).map((s) => s.name)).toEqual(
      shown.map(([, n]) => n),
    );
    expect(payouts?.config.agents[payouts.config.lead]?.role).toBe("team_lead");
    expect(payouts?.source).toContain("Adjustment: Add an architect.");
  });

  it("drops a rejected change", async () => {
    const c = client();
    await c.chiefOfStaff.start({ flow: "refine", team: "atlas", agent: "tove" });
    await say(c, "Push back more");
    const review = (await c.requests.list()).find((r) => r.source === "cos");
    await c.pipeline.reject(review?.mergeRequestId ?? "", "");
    const tove = (await c.workspace.teams()).find((t) => t.id === "atlas")?.config.agents.tove;
    expect(tove?.instruction).not.toContain("Push back more");
  });

  it("refines an agent once merged", async () => {
    const c = client();
    await c.chiefOfStaff.start({ flow: "refine", team: "atlas", agent: "tove" });
    const talk = await say(c, "Push back more");
    expect(talk.items.at(-1)?.text).toContain("Updated Tove’s job description on a branch");
    const review = (await c.requests.list()).find((r) => r.source === "cos");
    await c.pipeline.merge(review?.mergeRequestId ?? "");
    const tove = (await c.workspace.teams()).find((t) => t.id === "atlas")?.config.agents.tove;
    expect(tove?.instruction).toContain("Adjustment: Push back more.");
  });

  it("changes project config as a review on that project", async () => {
    const c = client();
    await c.chiefOfStaff.start({ flow: "project-config", project: "ledger" });
    const talk = await say(c, "Add a gate");
    expect(talk.items.at(-1)?.text).toBe(
      "Done: “Add a gate”. It’s on branch foreman/config-1 and waiting for you in ledger → Changes.",
    );
    expect((await c.requests.list()).find((r) => r.source === "cos")).toMatchObject({
      projectId: "ledger",
      summary: "Chief of Staff proposes a change to ledger/project/project.toml: Add a gate.",
    });
  });

  it("counts what needs the CEO like Home, and takes anything else on", async () => {
    const c = client();
    const open = (await c.requests.list()).length;
    let talk = await say(c, "What needs me today?");
    expect(talk.items.at(-1)?.text).toMatch(new RegExp(`^${open} things need you`));
    talk = await say(c, "Tidy the backlog");
    expect(talk.items.at(-1)?.text).toBe(
      "On it. I’ll work on “Tidy the backlog” and report back on Home when it’s done or if I need you.",
    );
  });
});

describe("the demo Chief of Staff's names", () => {
  it("never proposes a name already drafted in a review that isn't merged yet", async () => {
    const c = client();
    const hire = async () => {
      await c.chiefOfStaff.start({ flow: "hire", team: "atlas" });
      for (const line of [
        "Engineer",
        "Backend APIs and migrations",
        "Heads-down, reports when done",
      ])
        await say(c, line);
      const talk = await say(c, "Claude Code");
      const name = talk.items.at(-1)?.panel?.title.split(" · ")[0] ?? "";
      await say(c, `Hire ${name}`);
      return name;
    };
    const first = await hire();
    const second = await hire();
    expect(second).not.toBe(first);
  });
});

describe("the demo Chief of Staff's merge requests", () => {
  it("carries the change as a diff of the file it edits", async () => {
    const c = client();
    await c.chiefOfStaff.start({ flow: "hire", team: "atlas" });
    for (const line of [
      "QA",
      "Load and concurrency",
      "Pushes back, challenges decisions",
      "Claude Code",
    ])
      await say(c, line);
    const name = (await last(c))?.panel?.title.split(" · ")[0] ?? "";
    await say(c, `Hire ${name}`);
    const review = (await c.requests.list()).find((r) => r.source === "cos");
    const mr = await c.pipeline.mergeRequest(review?.mergeRequestId ?? "");
    expect(mr).toMatchObject({
      repo: "workspace",
      projectId: "",
      branch: "foreman/config-1",
      state: "open",
    });
    const diff = await c.pipeline.diff(mr.id);
    expect(diff.split("\n").slice(0, 2)).toEqual([
      "--- a/teams/atlas.toml",
      "+++ b/teams/atlas.toml",
    ]);
    expect(diff).toContain(`+[agents.${name.toLowerCase()}]`);
    expect(diff).toContain(`+name = "${name}"`);
    // A hire only adds lines (below the two header lines).
    expect(
      diff
        .split("\n")
        .slice(2)
        .some((l) => l.startsWith("-")),
    ).toBe(false);
  });

  it("hears why a change was rejected, and offers to start again", async () => {
    const c = client();
    await c.chiefOfStaff.start({ flow: "hire", team: "atlas" });
    for (const line of [
      "QA",
      "Load and concurrency",
      "Pushes back, challenges decisions",
      "Claude Code",
    ])
      await say(c, line);
    const name = (await last(c))?.panel?.title.split(" · ")[0] ?? "";
    await say(c, `Hire ${name}`);
    const review = (await c.requests.list()).find((r) => r.source === "cos");
    await c.pipeline.reject(review?.mergeRequestId ?? "", "Make them a reviewer");
    const talk = await c.chiefOfStaff.conversation();
    expect(talk.items.at(-1)?.text).toContain("Make them a reviewer");
    expect(talk.choices).toEqual(["Hire for a role"]);
    expect((await c.requests.list()).some((r) => r.source === "cos")).toBe(false);
    const atlas = (await c.workspace.teams()).find((t) => t.id === "atlas");
    expect(atlas?.config.agents[name.toLowerCase()]).toBeUndefined();
  });
});

describe("lineDiff", () => {
  it("marks kept, removed and added lines", () => {
    expect(lineDiff("f", "a\nb\nc", "a\nc\nd")).toBe("--- a/f\n+++ b/f\n a\n-b\n c\n+d");
  });
});
