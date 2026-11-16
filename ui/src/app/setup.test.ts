import { describe, expect, it } from "vitest";
import type { SetupStep } from "../api/client";
import { cpuChoices, logTail, memoryChoices, setupPending, stepTitle } from "./setup";

const step = (kind: string, state: string, subject = ""): SetupStep => ({
  kind,
  subject,
  state,
  detail: "",
  log: "",
});

describe("setup", () => {
  it("shows while any step is left, failed or working", () => {
    expect(setupPending([step("podman", "done"), step("team", "done")])).toBe(false);
    expect(setupPending([])).toBe(false);
    for (const state of ["todo", "working", "failed"]) {
      expect(setupPending([step("podman", "done"), step("image", state)])).toBe(true);
    }
  });

  it("titles name the profile", () => {
    expect(stepTitle(step("profile", "todo", "work"))).toBe("The work profile connected");
    expect(stepTitle(step("init", "failed"))).toBe("Podman can start agents");
    expect(stepTitle(step("hq", "todo"))).toBe("Foreman's HQ");
  });

  it("offers memory in whole GiB within the bounds, and none on a Mac too small", () => {
    const size = { minMemory: 4, maxMemory: 9, memory: 8, maxCpus: 8, cpus: 4 };
    expect(memoryChoices(size)).toEqual([4, 5, 6, 7, 8, 9]);
    expect(memoryChoices({ ...size, maxMemory: 3 })).toEqual([]);
    expect(cpuChoices(size)).toEqual([1, 2, 3, 4, 5, 6, 7, 8]);
  });

  it("keeps a log's last lines", () => {
    expect(logTail("a\nb\nc\nd\n", 2)).toBe("c\nd");
    expect(logTail("")).toBe("");
  });
});
