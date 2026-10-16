import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { type ReactNode, useState } from "react";
import {
  type ForgeAccount,
  type ImageStatus,
  isNotBuilt,
  type Settings as SettingsData,
  useClient,
} from "../api/client";
import { harnessLabel } from "../app/model";
import { permission } from "../app/notifications";
import {
  keys,
  useHarnesses,
  useMaintenance,
  useSaveSettings,
  useSettings,
  useWorkspace,
} from "../app/queries";
import { applyTheme, savedTheme, type Theme } from "../app/theme";
import { ConnectDialog } from "../components/ConnectDialog";
import { Modal } from "../components/Overlays";
import { useToast } from "../components/Toasts";
import { Button, NotBuilt, PageTitle, Segmented, Switch } from "../components/ui";

// Workspace-wide settings (wiki/product/features/settings.md). Every change
// saves at once, guarded by the version read.

export function Settings() {
  const file = useSettings();
  const s = file.data?.settings;
  return (
    <div className="mx-auto flex w-full max-w-[760px] flex-col gap-9 px-12 pt-12 pb-16">
      <PageTitle title="Settings" />
      {file.data && file.data.problems.length > 0 && (
        <p className="m-0 rounded-lg border border-err px-3 py-2 text-[13px] text-err">
          settings.toml: {file.data.problems.join(" · ")}
        </p>
      )}
      {s && (
        <>
          <WorkspaceSection settings={s} />
          <HarnessesSection />
          <ForgeSection settings={s} />
          <NotificationsSection settings={s} />
        </>
      )}
      <MaintenanceSection />
      <AppearanceSection />
    </div>
  );
}

/** Saves a change to the settings read last; a failure says why and the screen refetches. */
function useSave() {
  const save = useSaveSettings();
  const toast = useToast();
  return (change: (s: SettingsData) => SettingsData) =>
    save(change).catch((e: Error) =>
      toast({ title: "Settings not saved", body: e.message, color: "var(--err)" }),
    );
}

function Section({
  label,
  intro,
  children,
}: {
  label: string;
  intro?: string;
  children: ReactNode;
}) {
  return (
    <section className="flex flex-col">
      <h2 className="m-0 mb-1 text-[13px] font-medium text-tx4">{label}</h2>
      {intro && <p className="m-0 mb-2 text-[13.5px] text-tx3">{intro}</p>}
      {children}
    </section>
  );
}

function Row({
  title,
  explanation,
  children,
}: {
  title: string;
  explanation?: ReactNode;
  children: ReactNode;
}) {
  return (
    <div className="grid grid-cols-[1fr_auto] items-center gap-6 border-b border-line py-3.5">
      <div className="flex flex-col gap-0.5">
        <span className="font-medium">{title}</span>
        {explanation && <span className="text-[13.5px] leading-[1.5] text-tx3">{explanation}</span>}
      </div>
      <div className="flex items-center gap-2">{children}</div>
    </div>
  );
}

/** Asks before a setting that weakens safety. */
function Confirm({
  open,
  title,
  body,
  action,
  onConfirm,
  onClose,
}: {
  open: boolean;
  title: string;
  body: string;
  action: string;
  onConfirm: () => void;
  onClose: () => void;
}) {
  return (
    <Modal open={open} onClose={onClose} title={title}>
      <div className="flex flex-col gap-4">
        <p className="m-0 text-tx2">{body}</p>
        <div className="flex justify-end gap-2">
          <Button onClick={onClose}>Cancel</Button>
          <Button
            variant="danger"
            onClick={() => {
              onConfirm();
              onClose();
            }}
          >
            {action}
          </Button>
        </div>
      </div>
    </Modal>
  );
}

function WorkspaceSection({ settings }: { settings: SettingsData }) {
  const save = useSave();
  const workspace = useWorkspace();
  const engine = useMaintenance();
  const [confirm, setConfirm] = useState<"lan" | "direct">();
  return (
    <Section label="Workspace">
      <Row
        title="Folder"
        explanation="Every subfolder with a project/project.toml is a project. Foreman doesn’t clone anything. It uses the git repos it finds in each project folder."
      >
        <span className="font-mono text-[13px] text-tx2">{workspace.data?.path}</span>
      </Row>
      <Row
        title="Listen on"
        explanation="Anyone with the URL Foreman printed at start has full control. Keep this on your own machine unless you trust the network. Takes effect on restart."
      >
        <Segmented
          label="Listen on"
          value={settings.listen}
          onChange={(v) => (v === "lan" ? setConfirm("lan") : save((s) => ({ ...s, listen: v })))}
          options={[
            { value: "local", label: "This machine" },
            { value: "lan", label: "Local network" },
          ]}
        />
      </Row>
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
              <span className="mt-1.5 block font-medium text-accTx">
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
          onChange={(v) =>
            v === "direct" ? setConfirm("direct") : save((s) => ({ ...s, engine: v }))
          }
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
        open={confirm === "lan"}
        onClose={() => setConfirm(undefined)}
        title="Listen on the local network?"
        body="Anyone on your network who gets the URL can drive your agents, and through them your code and your machine. Only do this on a network you trust. Takes effect when Foreman restarts."
        action="Listen on the network"
        onConfirm={() => save((s) => ({ ...s, listen: "lan" }))}
      />
      <Confirm
        open={confirm === "direct"}
        onClose={() => setConfirm(undefined)}
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

function HarnessesSection() {
  const harnesses = useHarnesses();
  const [connecting, setConnecting] = useState<string>();
  return (
    <Section
      label="Harnesses"
      intro="The coding programs agents run, built into the agent image. Each needs its own login, once per workspace."
    >
      {(harnesses.data ?? []).map((h) => (
        <div
          key={h.id}
          className="grid grid-cols-[130px_1fr_auto] items-center gap-4 border-b border-line py-3"
        >
          <span className="font-medium">{harnessLabel(h.id)}</span>
          <span className="font-mono text-[13px] text-tx3">{h.command}</span>
          <span className="flex items-center gap-3">
            <span className={`text-[13px] ${h.connected ? "text-ok" : "text-tx4"}`}>
              {h.connected ? "connected" : "not connected"}
            </span>
            <Button variant="small" onClick={() => setConnecting(h.id)}>
              {h.connected ? "Reconnect" : "Connect"}
            </Button>
          </span>
        </div>
      ))}
      {connecting && (
        <ConnectDialog harness={connecting} onClose={() => setConnecting(undefined)} />
      )}
    </Section>
  );
}

const forges = [
  { kind: "github", label: "GitHub", host: "https://github.com/" },
  { kind: "forgejo", label: "Forgejo", host: "https://" },
  { kind: "gitea", label: "Gitea", host: "https://" },
];

const forgeLabel = (kind: string) => forges.find((f) => f.kind === kind)?.label ?? kind;

function ForgeSection({ settings }: { settings: SettingsData }) {
  const client = useClient();
  const accounts = useQuery({ queryKey: keys.forge, queryFn: () => client.forge.accounts() });
  const [editing, setEditing] = useState<string>();
  const connected = (accounts.data ?? []).filter((a) => a.tokenSet).map((a) => a.kind);
  return (
    <Section
      label="Forge accounts"
      intro="Tokens Foreman uses when you ask it to create a PR or sync a repo, and to watch those PRs. They stay on this machine. Each project’s config names its forge."
    >
      {isNotBuilt(accounts.error) ? (
        <NotBuilt what="forge accounts" />
      ) : (
        forges.map((f) => (
          <ForgeRow
            key={f.kind}
            forge={f}
            account={accounts.data?.find((a) => a.kind === f.kind)}
            url={settings.forges?.[f.kind]?.url ?? ""}
            editing={editing === f.kind}
            onEdit={(on) => setEditing(on ? f.kind : undefined)}
          />
        ))
      )}
      <WorkspaceRepo settings={settings} connected={connected} />
    </Section>
  );
}

function ForgeRow({
  forge,
  account,
  url,
  editing,
  onEdit,
}: {
  forge: (typeof forges)[number];
  account?: ForgeAccount;
  url: string;
  editing: boolean;
  onEdit: (on: boolean) => void;
}) {
  const client = useClient();
  const queries = useQueryClient();
  const toast = useToast();
  const [host, setHost] = useState(url || forge.host);
  const [token, setToken] = useState("");
  const connect = useMutation({
    mutationFn: () => client.forge.connect(forge.kind, host.trim(), token),
    onSuccess: () => {
      setToken("");
      onEdit(false);
      queries.invalidateQueries({ queryKey: keys.forge });
      queries.invalidateQueries({ queryKey: keys.settings });
      toast({ title: `${forge.label} connected`, body: host.trim(), color: "var(--ok)" });
    },
  });
  return (
    <div className="flex flex-col gap-3 border-b border-line py-3">
      <div className="grid grid-cols-[130px_1fr_auto] items-center gap-4">
        <span className="font-medium">{forge.label}</span>
        <span className="truncate font-mono text-[13px] text-tx3">{url || "—"}</span>
        <span className="flex items-center gap-3">
          <span className={`text-[13px] ${account?.tokenSet ? "text-ok" : "text-tx4"}`}>
            {account?.tokenSet ? "token set" : "not connected"}
          </span>
          <Button variant="small" onClick={() => onEdit(!editing)}>
            {editing ? "Cancel" : account?.tokenSet ? "Change" : "Connect"}
          </Button>
        </span>
      </div>
      {editing && (
        <form
          className="flex flex-col gap-2.5 rounded-[10px] border border-line2 p-3.5"
          onSubmit={(e) => {
            e.preventDefault();
            connect.mutate();
          }}
        >
          <label className="flex flex-col gap-1 text-[13px] text-tx3">
            Host and owner
            <input
              value={host}
              onChange={(e) => setHost(e.target.value)}
              className="rounded-[7px] border border-line2 bg-transparent px-2.5 py-1.5 font-mono text-[13px] text-tx"
            />
          </label>
          <label className="flex flex-col gap-1 text-[13px] text-tx3">
            Token (stored on this machine, never shown again)
            <input
              type="password"
              autoComplete="off"
              value={token}
              onChange={(e) => setToken(e.target.value)}
              className="rounded-[7px] border border-line2 bg-transparent px-2.5 py-1.5 font-mono text-[13px] text-tx"
            />
          </label>
          {connect.error && <p className="m-0 text-[13px] text-err">{connect.error.message}</p>}
          <div className="flex justify-end">
            <Button
              type="submit"
              variant="primary"
              disabled={connect.isPending || token.trim() === "" || host.trim() === ""}
            >
              Save
            </Button>
          </div>
        </form>
      )}
    </div>
  );
}

/** Where the workspace repo is hosted, so the Chief of Staff's changes can become PRs there. */
function WorkspaceRepo({ settings, connected }: { settings: SettingsData; connected: string[] }) {
  const save = useSave();
  const current = settings.workspaceForge;
  // A forge needs its repo URL before it can be saved (settings validation).
  const [kind, setKind] = useState(current.kind);
  const [url, setUrl] = useState(current.url);
  const kinds = ["none", ...connected];
  if (current.kind !== "none" && !kinds.includes(current.kind)) kinds.push(current.kind);
  const choose = (k: string) => {
    setKind(k);
    if (k === "none") save((s) => ({ ...s, workspaceForge: { kind: "none", url: "" } }));
  };
  const commitUrl = () => {
    const next = url.trim();
    if (kind === "none" || next === "" || (kind === current.kind && next === current.url)) return;
    save((s) => ({ ...s, workspaceForge: { kind, url: next } }));
  };
  return (
    <div className="flex flex-col gap-2.5 py-3.5">
      <div className="flex flex-col gap-0.5">
        <span className="font-medium">Workspace repo</span>
        <span className="text-[13.5px] leading-[1.5] text-tx3">
          Where your team definitions are hosted, so the Chief of Staff’s changes can become PRs
          there. Without a forge you merge them locally. Nothing is pushed automatically.
        </span>
      </div>
      <Segmented
        label="Workspace repo forge"
        value={kind}
        onChange={choose}
        options={kinds.map((k) => ({ value: k, label: k === "none" ? "No forge" : forgeLabel(k) }))}
      />
      {kind !== "none" && (
        <span className="flex items-center gap-2">
          <input
            aria-label="Workspace repo URL"
            placeholder="https://…/workspace"
            value={url}
            onChange={(e) => setUrl(e.target.value)}
            onBlur={commitUrl}
            onKeyDown={(e) => e.key === "Enter" && e.currentTarget.blur()}
            className="flex-1 rounded-[7px] border border-line2 bg-transparent px-2.5 py-1.5 font-mono text-[13px] text-tx"
          />
          <Button variant="small" disabled title="Comes with the Git epic">
            Sync
          </Button>
        </span>
      )}
    </div>
  );
}

const permissionColor = {
  on: "text-ok",
  blocked: "text-err",
  "turn on": "text-accTx",
  "n/a": "text-tx4",
};

function NotificationsSection({ settings }: { settings: SettingsData }) {
  const save = useSave();
  const toast = useToast();
  const [state, setState] = useState(permission);
  const toggles: { key: keyof SettingsData["notifications"]; label: string }[] = [
    { key: "needsMe", label: "An agent needs me" },
    { key: "finishes", label: "An agent finishes" },
    { key: "mergeRequested", label: "A merge is requested" },
  ];
  return (
    <Section label="Notifications">
      <Row
        title="Browser notifications"
        explanation="Shown while Foreman isn’t the tab you’re looking at. In-app toasts always show."
      >
        <span className={`text-[13px] ${permissionColor[state]}`}>{state}</span>
        {state === "turn on" && (
          <Button
            variant="small"
            onClick={async () => {
              await Notification.requestPermission();
              setState(permission());
              if (permission() === "on")
                toast({
                  title: "Notifications on",
                  body: "You'll hear from agents when they finish or need you.",
                  color: "var(--ok)",
                });
            }}
          >
            Request
          </Button>
        )}
      </Row>
      {toggles.map((t) => (
        <Row key={t.key} title={t.label}>
          <Switch
            label={t.label}
            checked={settings.notifications[t.key]}
            onChange={(on) =>
              save((s) => ({ ...s, notifications: { ...s.notifications, [t.key]: on } }))
            }
          />
        </Row>
      ))}
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
        <p className="m-0 border-b border-line py-3.5 text-[13.5px] text-tx3">
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
      {rebuild.error && <p className="m-0 pt-2 text-[13px] text-err">{rebuild.error.message}</p>}
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
    <div className="flex flex-col gap-2.5 border-b border-line py-3.5">
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

function AppearanceSection() {
  const [theme, setTheme] = useState<Theme>(savedTheme);
  return (
    <Section label="Appearance">
      <Row title="Theme" explanation="System follows your operating system.">
        <Segmented
          label="Theme"
          value={theme}
          onChange={(t) => {
            setTheme(t);
            applyTheme(t);
          }}
          options={[
            { value: "system", label: "System" },
            { value: "dark", label: "Dark" },
            { value: "light", label: "Light" },
          ]}
        />
      </Row>
    </Section>
  );
}
