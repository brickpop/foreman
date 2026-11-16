// The setup checklist on the Dashboard (wiki/product/features/first-run.md): which
// steps show, and what each row says.
import type { MachineSize, SetupStep } from "../api/client";

/** Whether the checklist shows: some step isn't done. */
export function setupPending(steps: readonly SetupStep[]): boolean {
  return steps.some((s) => s.state !== "done");
}

/** A step's row title. */
export function stepTitle(s: SetupStep): string {
  switch (s.kind) {
    case "hq":
      return "Foreman's HQ";
    case "podman":
      return "Podman installed";
    case "machine":
      return "Podman machine";
    case "running":
      return "Podman running";
    case "init":
      return "Podman can start agents";
    case "image":
      return "Agent image";
    case "profile":
      return `The ${s.subject} profile connected`;
    case "team":
      return "A team";
    case "project":
      return "A project";
    default:
      return s.kind;
  }
}

/** The memory choices for a new machine, in GiB: none when the Mac is too small. */
export function memoryChoices(size: MachineSize): number[] {
  const out: number[] = [];
  for (let gib = size.minMemory; gib <= size.maxMemory; gib++) out.push(gib);
  return out;
}

/** The CPU choices for a new machine. */
export function cpuChoices(size: MachineSize): number[] {
  return Array.from({ length: size.maxCpus }, (_, i) => i + 1);
}

/** The last lines of a log, for a row that shows its progress. */
export function logTail(log: string, lines = 6): string {
  return log.trimEnd().split("\n").slice(-lines).join("\n");
}
