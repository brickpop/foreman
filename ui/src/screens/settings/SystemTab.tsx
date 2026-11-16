import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { type ImageStatus, useClient } from "../../api/client";
import { keys, useHomeFolder, useMaintenance } from "../../app/queries";
import { Button } from "../../components/ui";
import { onThisMachine, Row, Section } from "./parts";

/** Settings → System: Foreman's folder and home, and the agent images. */
export function SystemTab() {
  return (
    <>
      <HomeSection />
      <MaintenanceSection />
    </>
  );
}

function HomeSection() {
  const home = useHomeFolder();
  return (
    <Section label="Folders" kept={onThisMachine}>
      <Row
        title="Foreman’s folder"
        explanation="What is yours: Foreman’s HQ (teams, your instructions), each project’s HQ and files, and the agents’ worktrees. To move it, stop Foreman, move the folder, and set foreman_folder in settings.toml."
      >
        <span className="font-mono text-[13px] text-tx2">{home.data?.folder}</span>
      </Row>
      <Row
        title="Home"
        explanation="Foreman’s own machinery on this machine: settings.toml, its state, the logins, and the agents’ harness folders (their config and conversations)."
      >
        <span className="font-mono text-[13px] text-tx2">{home.data?.path}</span>
      </Row>
    </Section>
  );
}

const imageState: Record<string, { label: string; color: string }> = {
  ready: { label: "ready", color: "text-ok" },
  building: { label: "building…", color: "text-team" },
  failed: { label: "failed", color: "text-err" },
  missing: { label: "not built yet", color: "text-tx4" },
};

function MaintenanceSection() {
  const client = useClient();
  const queries = useQueryClient();
  const status = useMaintenance();
  const rebuild = useMutation({
    mutationFn: () => client.maintenance.rebuild(),
    onSettled: () => queries.invalidateQueries({ queryKey: keys.maintenance }),
  });
  const building = status.data?.images.some((i) => i.state === "building") ?? false;
  return (
    <Section
      label="Maintenance"
      kept={onThisMachine}
      intro="The image agent sessions run in. Rebuild picks up the latest harness versions; running sessions keep their image, new and restarted ones use the new one."
    >
      <Row title="Engine">
        <span
          className={`text-[13px] ${status.data?.engine.includes("not reachable") ? "text-err" : "text-ok"}`}
        >
          {status.data?.engine ?? "…"}
        </span>
      </Row>
      {status.data?.images.length === 0 && (
        <p className="m-0 text-[13.5px] text-tx3">
          Agents run directly on this machine: no images to build.
        </p>
      )}
      {status.data?.images.map((image) => (
        <ImageRow
          key={image.name}
          image={image}
          onRebuild={() => rebuild.mutate()}
          busy={building || rebuild.isPending}
        />
      ))}
      {rebuild.error && <p className="m-0 text-[13px] text-err">{rebuild.error.message}</p>}
    </Section>
  );
}

function ImageRow({
  image,
  onRebuild,
  busy,
}: {
  image: ImageStatus;
  onRebuild: () => void;
  busy: boolean;
}) {
  const [log, setLog] = useState(false);
  const state = imageState[image.state] ?? { label: image.state, color: "text-tx3" };
  return (
    <div className="flex flex-col gap-2.5">
      <div className="grid grid-cols-[1fr_auto] items-center gap-4">
        <span className="flex flex-col gap-0.5">
          <span className="font-mono text-[13px]">{image.name}</span>
          <span className={`text-[13px] ${state.color}`}>{state.label}</span>
        </span>
        <span className="flex gap-2">
          <Button variant="small" disabled={image.log === ""} onClick={() => setLog(!log)}>
            {log ? "Hide log" : "Log"}
          </Button>
          <Button variant="small" disabled={busy} onClick={onRebuild}>
            Rebuild
          </Button>
        </span>
      </div>
      {log && (
        <pre className="m-0 max-h-80 overflow-auto rounded-[10px] border border-line bg-term p-3.5 font-mono text-[12px] leading-[1.6] whitespace-pre-wrap text-tx2 select-text">
          {image.log}
        </pre>
      )}
    </div>
  );
}
