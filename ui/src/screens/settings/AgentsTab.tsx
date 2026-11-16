import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useEffect, useState } from "react";
import { type Settings as SettingsData, useClient } from "../../api/client";
import { harnessLabel, profileName } from "../../app/model";
import {
  keys,
  useHarnesses,
  useInstructions,
  useMaintenance,
  useProfiles,
} from "../../app/queries";
import { logCapChoices, stuckChoices, withCurrent } from "../../app/settings";
import { ConnectDialog } from "../../components/ConnectDialog";
import { Confirm } from "../../components/Overlays";
import { Select } from "../../components/Select";
import { useToast } from "../../components/Toasts";
import { Button, Segmented } from "../../components/ui";
import { inHQ, onThisMachine, Row, Section, useSave } from "./parts";

/** Settings → Agents: the profiles agents run on, what every agent is told, and how they run. */
export function AgentsTab({ settings }: { settings?: SettingsData }) {
  return (
    <>
      <ProfilesSection />
      <InstructionsSection />
      {settings && <RunningSection settings={settings} />}
    </>
  );
}

function RunningSection({ settings }: { settings: SettingsData }) {
  const save = useSave();
  const engine = useMaintenance();
  const [confirm, setConfirm] = useState(false);
  return (
    <Section label="Running agents" kept={onThisMachine}>
      <Row
        title="Running sessions at once"
        explanation="Extra sessions wait in a queue until a slot frees up."
      >
        <Stepper
          value={settings.maxSessions}
          min={1}
          max={64}
          label="Running sessions at once"
          onChange={(n) => save((s) => ({ ...s, maxSessions: n }))}
        />
      </Row>
      <Row
        title="Stuck after"
        explanation="An agent mid-turn that sends no sign of activity (a prompt, a tool call, a turn ending) for this long gets a card on the Dashboard. It keeps running; you decide."
      >
        <Select
          label="Stuck after"
          value={settings.stuckAfterMinutes}
          options={withCurrent(stuckChoices, settings.stuckAfterMinutes).map((m) => ({
            value: m,
            label: `${m} min`,
          }))}
          onChange={(m) => save((s) => ({ ...s, stuckAfterMinutes: m }))}
        />
      </Row>
      <Row
        title="Session logs per agent"
        explanation="Every byte each agent's terminal printed, kept in Foreman's home. Past this size, the oldest go first."
      >
        <Select
          label="Session logs per agent"
          value={settings.logCapMb}
          options={withCurrent(logCapChoices, settings.logCapMb).map((mb) => ({
            value: mb,
            label: `${mb} MB`,
          }))}
          onChange={(mb) => save((s) => ({ ...s, logCapMb: mb }))}
        />
      </Row>
      <Row
        title="Run agents in"
        explanation={
          <>
            Containers isolate each agent from your machine. Without them, agents run as plain
            processes with full access to it. Takes effect when Foreman restarts.
            {engine.data?.podman && (
              <span className="mt-1.5 block text-tx2">
                Containers aren't available: {engine.data.podman}
              </span>
            )}
            {engine.data && settings.engine !== engine.data.running && (
              <span className="mt-1.5 block font-medium text-attnTx">
                Restart Foreman to switch: agents run{" "}
                {engine.data.running === "direct" ? "directly" : "in containers"} until then.
              </span>
            )}
            {engine.data?.running === "direct" && (
              <span className="mt-1.5 block font-medium text-err">
                Agents run with no isolation: they can read and change anything your user can.
              </span>
            )}
          </>
        }
      >
        <Segmented
          label="Run agents in"
          value={settings.engine}
          onChange={(v) => (v === "direct" ? setConfirm(true) : save((s) => ({ ...s, engine: v })))}
          options={[
            // Containers can't be chosen without Podman: Foreman wouldn't start.
            {
              value: "podman",
              label: "Containers (Podman)",
              disabled: !!engine.data?.podman && settings.engine !== "podman",
            },
            { value: "direct", label: "Directly (no isolation)" },
          ]}
        />
      </Row>
      <Confirm
        open={confirm}
        onClose={() => setConfirm(false)}
        title="Run agents with no isolation?"
        body="Agents run as plain processes on this machine, with permission checks off: they can read and change anything your user can. Use this for development only."
        action="Run directly"
        onConfirm={() => save((s) => ({ ...s, engine: "direct" }))}
      />
    </Section>
  );
}

function Stepper({
  value,
  min,
  max,
  label,
  onChange,
}: {
  value: number;
  min: number;
  max: number;
  label: string;
  onChange: (n: number) => void;
}) {
  return (
    <fieldset className="m-0 flex items-center gap-1 border-0 p-0">
      <legend className="sr-only">{label}</legend>
      <Button
        className="!size-7 !p-0"
        aria-label="Fewer"
        disabled={value <= min}
        onClick={() => onChange(value - 1)}
      >
        −
      </Button>
      <span className="min-w-7 text-center font-mono text-sm font-medium" aria-live="polite">
        {value}
      </span>
      <Button
        className="!size-7 !p-0"
        aria-label="More"
        disabled={value >= max}
        onClick={() => onChange(value + 1)}
      >
        +
      </Button>
    </fieldset>
  );
}

/** The CEO's profiles: each a harness and its login, on this machine. A
 * project picks one in its settings. */
function ProfilesSection() {
  const client = useClient();
  const queries = useQueryClient();
  const toast = useToast();
  const profiles = useProfiles();
  const harnesses = useHarnesses();
  const [connecting, setConnecting] = useState<string>();
  const [adding, setAdding] = useState(false);
  const [deleting, setDeleting] = useState<string>();
  const refetch = () => {
    queries.invalidateQueries({ queryKey: keys.profiles });
    queries.invalidateQueries({ queryKey: keys.setup });
  };
  const remove = useMutation({
    mutationFn: (name: string) => client.profiles.remove(name),
    onSettled: refetch,
    onError: (e: Error) =>
      toast({ title: "Profile not deleted", body: e.message, color: "var(--err)" }),
  });
  return (
    <Section
      label="Profiles"
      kept={onThisMachine}
      intro="Each profile is a harness and its credentials, yours on this machine: Claude Code's or Codex's login, or the model server Pi runs on. Every project can use it; a project uses default unless its settings pick another."
    >
      {profiles.isError && <p className="m-0 text-[13px] text-err">{profiles.error.message}</p>}
      {(profiles.data ?? []).map((p) => (
        <div key={p.name} className="grid grid-cols-[1fr_auto_auto] items-center gap-4">
          <span className="flex min-w-0 items-baseline gap-2">
            <span className="truncate font-mono font-semibold">{p.name}</span>
            <span className="text-[13px] text-tx4">{harnessLabel(p.harness)}</span>
          </span>
          <span className={`text-[13px] ${p.connected ? "text-ok" : "text-tx4"}`}>
            {p.connected ? "connected" : "not connected"}
          </span>
          <span className="flex items-center gap-2">
            <Button variant="small" onClick={() => setConnecting(p.name)}>
              {p.connected ? "Reconnect" : "Connect"}
            </Button>
            {p.name !== "default" && (
              <Button variant="small" onClick={() => setDeleting(p.name)}>
                Delete
              </Button>
            )}
          </span>
        </div>
      ))}
      {adding ? (
        <NewProfile
          harnesses={(harnesses.data ?? []).map((h) => h.id)}
          onCancel={() => setAdding(false)}
          onCreated={(name) => {
            setAdding(false);
            refetch();
            setConnecting(name); // its login next
          }}
        />
      ) : (
        <div className="pt-3">
          <Button variant="small" onClick={() => setAdding(true)}>
            New profile
          </Button>
        </div>
      )}
      {connecting && (
        <ConnectDialog profile={connecting} onClose={() => setConnecting(undefined)} />
      )}
      <Confirm
        open={deleting !== undefined}
        title={`Delete the ${deleting} profile?`}
        body="Its credentials go with it. Projects that use it can't start agents until they pick another profile."
        action="Delete"
        onConfirm={() => deleting && remove.mutate(deleting)}
        onClose={() => setDeleting(undefined)}
      />
    </Section>
  );
}

/** A new profile: its name and its harness; its credentials come after (Connect). */
function NewProfile({
  harnesses,
  onCancel,
  onCreated,
}: {
  harnesses: string[];
  onCancel: () => void;
  onCreated: (name: string) => void;
}) {
  const client = useClient();
  const [typed, setTyped] = useState("");
  const [harness, setHarness] = useState(harnesses[0] ?? "claude-code");
  const name = profileName(typed);
  const create = useMutation({
    mutationFn: () => client.profiles.create(name, harness),
    onSuccess: (p) => onCreated(p.name),
  });
  return (
    <form
      className="flex flex-wrap items-center gap-2 text-[13px]"
      onSubmit={(e) => {
        e.preventDefault();
        if (name) create.mutate();
      }}
    >
      <label htmlFor="new-profile" className="text-tx3">
        Name
      </label>
      <input
        id="new-profile"
        // biome-ignore lint/a11y/noAutofocus: opened on purpose, to type its name
        autoFocus
        value={typed}
        onChange={(e) => setTyped(e.target.value)}
        placeholder="work"
        className="w-36 rounded-md border border-line2 bg-panel px-2 py-1 font-mono text-[13px]"
      />
      {typed && name !== typed && <span className="font-mono text-tx4">{name || "—"}</span>}
      <span className="ml-2 text-tx3">Harness</span>
      <Select
        label="Harness"
        value={harness}
        onChange={setHarness}
        options={harnesses.map((h) => ({ value: h, label: harnessLabel(h) }))}
      />
      <span className="flex-1" />
      <Button variant="small" onClick={onCancel}>
        Cancel
      </Button>
      <Button variant="small" type="submit" disabled={!name || create.isPending}>
        Create
      </Button>
      {create.error && <p className="m-0 w-full text-err">{create.error.message}</p>}
    </form>
  );
}

/** The CEO's standing instructions (instructions.md in Foreman's HQ): added
 * to every agent's session prompt, at its next start. Saved on Save, one
 * commit each, never mid-sentence. */
function InstructionsSection() {
  const client = useClient();
  const queries = useQueryClient();
  const toast = useToast();
  const saved = useInstructions().data;
  // undefined: not edited, the saved text shows (and follows a change on disk).
  const [draft, setDraft] = useState<string>();
  const save = useMutation({
    mutationFn: (text: string) => client.home.saveInstructions(text, saved?.version ?? ""),
    onSuccess: (i) => queries.setQueryData(keys.instructions, i),
    onError: (e) => {
      toast({ title: "Instructions not saved", body: e.message, color: "var(--err)" });
      queries.invalidateQueries({ queryKey: keys.instructions });
    },
  });
  const changed = saved !== undefined && draft !== undefined && draft !== saved.text;
  // Once saved, the draft goes when the instructions read hold it, not
  // before: the old text would flash back meanwhile.
  // biome-ignore lint/correctness/useExhaustiveDependencies: on the saved text only
  useEffect(() => {
    if (draft === saved?.text) setDraft(undefined);
  }, [saved?.text]);
  return (
    <Section
      label="Instructions for every agent"
      kept={inHQ}
      intro="Your standing rules, as you'd write them in your own CLAUDE.md or AGENTS.md. Foreman adds them at the end of every agent's session prompt, from its next start. Where they conflict with Foreman's own rules (team tools, git, the wiki), Foreman's win."
    >
      <div className="flex flex-col gap-2">
        <textarea
          rows={8}
          value={draft ?? saved?.text ?? ""}
          onChange={(e) => setDraft(e.target.value)}
          disabled={!saved}
          placeholder="e.g. Write commit messages in English. Prefer small functions."
          aria-label="Instructions for every agent"
          className="w-full resize-y rounded-[8px] border border-line2 bg-term px-3 py-2 font-mono text-[13px] text-tx placeholder:text-tx4"
        />
        {changed && (
          <div className="flex gap-2">
            <Button variant="primary" disabled={save.isPending} onClick={() => save.mutate(draft)}>
              Save
            </Button>
            <Button disabled={save.isPending} onClick={() => setDraft(undefined)}>
              Discard
            </Button>
          </div>
        )}
      </div>
    </Section>
  );
}
