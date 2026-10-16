import { useMutation, useQueryClient } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import { useState } from "react";
import { type MachineSize, type SetupStep, useClient } from "../api/client";
import { useOpenChiefOfStaff } from "../app/chiefOfStaff";
import { keys } from "../app/queries";
import { cpuChoices, logTail, memoryChoices, stepTitle } from "../app/setup";
import { AddProject } from "./AddProject";
import { ConnectDialog } from "./ConnectDialog";
import { CheckIcon } from "./icons";
import { Button, Panel, SectionLabel } from "./ui";

// The setup checklist (wiki/product/features/first-run.md): Home shows it
// while any step is left, in order, each row with what to do and its button.
export function SetupChecklist({ steps }: { steps: SetupStep[] }) {
  const done = steps.filter((s) => s.state === "done").length;
  const machineStopped =
    steps.find((s) => s.kind === "machine")?.state === "done" &&
    steps.find((s) => s.kind === "running")?.state === "todo";
  return (
    <section className="flex flex-col gap-2.5">
      <SectionLabel>
        Setup · {done} of {steps.length} done
      </SectionLabel>
      <Panel>
        {steps.map((s, i) => (
          <StepRow
            key={`${s.kind}:${s.subject}`}
            step={s}
            first={i === 0}
            startable={s.kind === "running" && machineStopped}
          />
        ))}
      </Panel>
    </section>
  );
}

const marks: Record<string, string> = {
  done: "border-ok bg-ok text-accInk",
  todo: "border-line3",
  working: "border-acc border-t-transparent animate-spin",
  failed: "border-err bg-err",
};

function StepRow({
  step: s,
  first,
  startable,
}: {
  step: SetupStep;
  first: boolean;
  startable: boolean;
}) {
  const done = s.state === "done";
  return (
    <div className={`flex flex-col gap-2 px-4 py-3 ${first ? "" : "border-t border-line"}`}>
      <div className="flex items-center gap-3">
        <span
          role="img"
          aria-label={s.state}
          className={`flex size-[18px] shrink-0 items-center justify-center rounded-full border-2 ${marks[s.state] ?? marks.todo}`}
        >
          {done && <CheckIcon />}
        </span>
        <span className="flex min-w-0 flex-1 flex-col gap-0.5">
          <span className={`text-[14px] font-medium ${done ? "text-tx3" : "text-tx"}`}>
            {stepTitle(s)}
          </span>
          {!done && s.detail && (
            <span
              className={`text-[13px] ${s.state === "failed" ? "text-err" : "text-tx3"} break-words`}
            >
              {s.detail}
            </span>
          )}
        </span>
        {!done && <StepAction step={s} startable={startable} />}
      </div>
      {s.state === "todo" && s.machine && <MachineForm size={s.machine} />}
      {(s.state === "working" || s.state === "failed") && s.log && (
        <pre className="m-0 ml-[30px] max-h-36 overflow-auto rounded-lg bg-term px-3 py-2 font-mono text-[12px] whitespace-pre-wrap text-tx3">
          {logTail(s.log)}
        </pre>
      )}
    </div>
  );
}

function StepAction({ step: s, startable }: { step: SetupStep; startable: boolean }) {
  const client = useClient();
  const queries = useQueryClient();
  const openChiefOfStaff = useOpenChiefOfStaff();
  const [dialog, setDialog] = useState(false);
  const start = useMutation({
    mutationFn: () => client.setup.startPodman(),
    onSettled: () => queries.invalidateQueries({ queryKey: keys.setup }),
  });
  if (startable) {
    return (
      <Button variant="small" disabled={start.isPending} onClick={() => start.mutate()}>
        Start
      </Button>
    );
  }
  switch (s.kind) {
    case "image":
      return s.state === "failed" ? (
        <Link to="/settings" className="text-[13px] text-accTx hover:underline">
          Maintenance
        </Link>
      ) : null;
    case "harness":
      return (
        <>
          <Button variant="small" onClick={() => setDialog(true)}>
            Connect
          </Button>
          {dialog && <ConnectDialog harness={s.subject} onClose={() => setDialog(false)} />}
        </>
      );
    case "team":
      return (
        <Button variant="small" onClick={() => openChiefOfStaff({ flow: "create-team" })}>
          Create a team
        </Button>
      );
    case "project":
      return (
        <>
          <Button variant="small" onClick={() => setDialog(true)}>
            Add project
          </Button>
          {dialog && <AddProject open onClose={() => setDialog(false)} />}
        </>
      );
    case "repo":
      return (
        <Link
          to="/projects/$project"
          params={{ project: s.subject }}
          search={{ tab: "settings" }}
          className="text-[13px] text-accTx hover:underline"
        >
          New repo
        </Link>
      );
    default:
      return null;
  }
}

const select = "rounded-[7px] border border-line2 bg-transparent px-2 py-1 text-[13px] text-tx";

/** The machine's size, prefilled with half the Mac within the CEO's bounds. */
function MachineForm({ size }: { size: MachineSize }) {
  const client = useClient();
  const queries = useQueryClient();
  const [memory, setMemory] = useState(size.memory);
  const [cpus, setCpus] = useState(size.cpus);
  const create = useMutation({
    mutationFn: () => client.setup.createMachine({ cpus, memory }),
    onSettled: () => queries.invalidateQueries({ queryKey: keys.setup }),
  });
  const memories = memoryChoices(size);
  if (memories.length === 0) {
    return (
      <p className="m-0 ml-[30px] text-[13px] text-err">
        This Mac has too little memory for a Podman machine: Foreman needs {size.minMemory} GB, and
        uses at most 60% of the Mac's.
      </p>
    );
  }
  return (
    <div className="ml-[30px] flex flex-wrap items-center gap-3 text-[13px] text-tx2">
      <label className="flex items-center gap-1.5">
        Memory
        <select
          className={select}
          value={memory}
          onChange={(e) => setMemory(Number(e.target.value))}
        >
          {memories.map((m) => (
            <option key={m} value={m}>
              {m} GB
            </option>
          ))}
        </select>
      </label>
      <label className="flex items-center gap-1.5">
        CPUs
        <select className={select} value={cpus} onChange={(e) => setCpus(Number(e.target.value))}>
          {cpuChoices(size).map((c) => (
            <option key={c} value={c}>
              {c}
            </option>
          ))}
        </select>
      </label>
      <Button variant="small" disabled={create.isPending} onClick={() => create.mutate()}>
        Create machine
      </Button>
      <span className="basis-full text-[12.5px] text-tx4">
        Downloads Podman's machine image (about 1 GB) once. Each agent needs a few hundred MB;
        <code className="font-mono">podman machine set</code> resizes it later.
      </span>
      {create.error && <span className="basis-full text-err">{create.error.message}</span>}
    </div>
  );
}
